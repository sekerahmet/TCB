package interp

// GEÇİCİ (Faz 0.9): tip tanığı. main'e girmez; göç bitince geri alınır.
//
// TR_TYPE_WITNESS=<dosya> ile açılır: her kullanıcı fonksiyonu çağrısında argümanların, dönen
// değerin ve yapı alanlarının (kurmada ve atamada) gerçek tipini yazılan tiple karşılaştırır ve
// uyuşmazlığı dosyaya bir satır olarak ekler. Denetleyici bugün bileşik tipi asıl adına
// indirdiği için uydurma bir TR_List[TR_Int] ya da yanlış cinsli TR_Tensor hiçbir testte
// düşmez; tanık onları çalışma anında yakalar.

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

var (
	witnessFile = os.Getenv("TR_TYPE_WITNESS")
	witnessMu   sync.Mutex
	witnessSeen = map[string]bool{}
)

// WitnessTensorKind tensör değerinin öğe cinsini tanığa söyler: "Int" (tam sayılı işaretli),
// "Bool" (yalnız 0 ve 1), "Integral" (tam değerli ondalık), "Float".
type WitnessTensorKind interface{ WitnessKind() string }

const witnessLimit = 2000

func witnessOn() bool { return witnessFile != "" }

func (v *VM) witnessSourceName(src *Source) string {
	if src != nil {
		if src.Name != "" {
			return src.Name
		}
		return src.Path
	}
	return v.SourceFile
}

func witnessReport(line string) {
	witnessMu.Lock()
	defer witnessMu.Unlock()
	if witnessSeen[line] {
		return
	}
	witnessSeen[line] = true
	f, err := os.OpenFile(witnessFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

// witnessParams bağlanan parametreleri yazılan tipleriyle karşılaştırır.
func (v *VM) witnessParams(fn *FuncVal, fk *funcCode, env *Environment) {
	lits := v.witnessTakeLits(fn)
	for i, p := range fn.Parameters {
		if p.Type == nil || i >= len(fk.param) {
			continue
		}
		val := env.vals[fk.param[i]]
		why := v.witnessMatch(p.Type, val, env)
		if strings.HasPrefix(why, "TAM→ONDALIK") && (lits[fmt.Sprint(i)] || lits[p.Name] || isIntLit(p.DefaultValue)) {
			why = "" // tam sayı değişmezi TR_Float yerinde ondalıktır (3.13)
		}
		if why != "" {
			witnessReport(fmt.Sprintf("%s:%d: parametre %s.%s: yazılan %s, %s", v.witnessSourceName(fk.source), p.NameToken.Line, fn.Name, p.Name, typeRefText(p.Type), why))
		}
	}
}

// witnessReturn dönen değeri yazılan dönüş tipiyle karşılaştırır.
func (v *VM) witnessReturn(fn *FuncVal, fk *funcCode, env *Environment, val Value) {
	if fn.ReturnType == nil {
		return
	}
	if why := v.witnessMatch(fn.ReturnType, val, env); why != "" {
		witnessReport(fmt.Sprintf("%s:%d: dönüş %s: yazılan %s, %s", v.witnessSourceName(fk.source), fn.ReturnType.Token.Line, fn.Name, typeRefText(fn.ReturnType), why))
	}
}

// witnessFields nesnenin alanlarını yazılan tipleriyle karşılaştırır.
func (v *VM) witnessFields(def *StructDefVal, fields []ast.StructField, inst *StructInstanceVal) {
	env := def.fieldEnv
	if env == nil {
		env = v.GlobalEnv
	}
	lits := v.witnessTakeLits(def)
	for i, f := range fields {
		if f.Type != nil && (lits[fmt.Sprint(i)] || lits[f.Name] || isIntLit(f.DefaultValue)) && f.Type.Token.Code == codes.TypeFloat {
			continue // tam sayı değişmezi TR_Float yerinde ondalıktır (3.13)
		}
		v.witnessField(def, f, inst.Fields[f.Name], env)
	}
}

func (v *VM) witnessField(def *StructDefVal, f ast.StructField, val Value, env *Environment) {
	if f.Type == nil {
		return
	}
	if why := v.witnessMatch(f.Type, val, env); why != "" {
		witnessReport(fmt.Sprintf("%s:%d: alan %s.%s: yazılan %s, %s", v.witnessSourceName(def.witnessSource), f.NameToken.Line, def.Name, f.Name, typeRefText(f.Type), why))
	}
}

// witnessSetField nesne.alan = değer atamasını alanın tipiyle karşılaştırır.
func (v *VM) witnessSetField(inst *StructInstanceVal, prop string, val Value) {
	for _, f := range inst.Def.Fields {
		if f.Name == prop {
			env := inst.Def.fieldEnv
			if env == nil {
				env = v.GlobalEnv
			}
			v.witnessField(inst.Def, f, val, env)
		}
	}
}

func typeRefText(t *ast.TypeRef) string {
	var b strings.Builder
	switch {
	case t.Return != nil:
		b.WriteString("(")
		for i := range t.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeRefText(&t.Params[i]))
		}
		b.WriteString(") -> " + typeRefText(t.Return))
	case len(t.Elems) > 0:
		b.WriteString("(")
		for i := range t.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeRefText(&t.Elems[i]))
		}
		b.WriteString(")")
	default:
		b.WriteString(t.Name)
		if len(t.Args) > 0 {
			b.WriteString("[")
			for i := range t.Args {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(typeRefText(&t.Args[i]))
			}
			b.WriteString("]")
		}
	}
	if t.Optional {
		b.WriteString("?")
	}
	return b.String()
}

// witnessMatch değer yazılan tipte mi? Uyuşmazlıkta nedeni verir, uyuyorsa boş metin.
func (v *VM) witnessMatch(t *ast.TypeRef, val Value, env *Environment) string {
	if val == nil {
		val = Null
	}
	if _, isNull := val.(*NullVal); isNull {
		if t.Optional || t.Token.Code == codes.TypeNoneType || t.Token.Code == codes.TypeAny {
			return ""
		}
		return "gelen yok (TR_none): tip isteğe bağlı olmalı"
	}
	if j, ok := val.(*JSONVal); ok && t.Token.Code != codes.TypeJSON && t.Token.Code != codes.TypeAny {
		return "gelen TR_JSON (" + string(j.Type()) + ")"
	}
	switch {
	case t.Return != nil:
		switch val.(type) {
		case *FuncVal, *BuiltinVal, *StructDefVal, *TypeVal:
			return ""
		}
		return "gelen " + string(val.Type()) + " (fonksiyon değil)"
	case len(t.Elems) > 0:
		tv, ok := val.(*TupleVal)
		if !ok || len(tv.Elements) != len(t.Elems) {
			return "gelen " + string(val.Type()) + " (demet değil ya da öğe sayısı tutmuyor)"
		}
		for i := range t.Elems {
			if why := v.witnessMatch(&t.Elems[i], tv.Elements[i], env); why != "" {
				return fmt.Sprintf("demetin %d. öğesi: %s", i, why)
			}
		}
		return ""
	}
	return v.witnessNamed(t, val, env)
}

func (v *VM) witnessNamed(t *ast.TypeRef, val Value, env *Environment) string {
	got := "gelen " + string(val.Type())
	switch t.Token.Code {
	case codes.TypeAny:
		return ""
	case codes.TypeInt:
		if _, ok := val.(*IntVal); ok {
			return ""
		}
	case codes.TypeFloat:
		switch val.(type) {
		case *FloatVal:
			return ""
		case *IntVal:
			return "TAM→ONDALIK gelen Int"
		}
	case codes.TypeString:
		if _, ok := val.(*StringVal); ok {
			return ""
		}
	case codes.TypeBool:
		if _, ok := val.(*BoolVal); ok {
			return ""
		}
	case codes.TypeBytes:
		if _, ok := val.(*BytesVal); ok {
			return ""
		}
	case codes.TypeRange:
		if _, ok := val.(*RangeVal); ok {
			return ""
		}
	case codes.TypeJSON:
		if _, ok := val.(*JSONVal); ok {
			return ""
		}
	case codes.TypeType:
		switch val.(type) {
		case *TypeVal, *StructDefVal:
			return ""
		}
	case codes.TypeFunction, codes.TypeBuiltinFunction:
		switch val.(type) {
		case *FuncVal, *BuiltinVal:
			return ""
		}
	case codes.TypeList:
		l, ok := val.(*ListVal)
		if !ok {
			break
		}
		if len(t.Args) == 1 {
			for i, e := range l.Elements {
				if i >= witnessLimit {
					break
				}
				if why := v.witnessMatch(&t.Args[0], e, env); why != "" {
					return fmt.Sprintf("listenin %d. öğesi: %s", i, why)
				}
			}
		}
		return ""
	case codes.TypeDict:
		d, ok := val.(*DictVal)
		if !ok {
			break
		}
		if len(t.Args) == 2 {
			for i, k := range d.Order {
				if i >= witnessLimit {
					break
				}
				if why := v.witnessMatch(&t.Args[0], d.KeyValue(k), env); why != "" {
					return fmt.Sprintf("sözlüğün anahtarı %q: %s", k, why)
				}
				if why := v.witnessMatch(&t.Args[1], d.Pairs[k], env); why != "" {
					return fmt.Sprintf("sözlüğün %q değeri: %s", k, why)
				}
			}
		}
		return ""
	case codes.TypeTensor:
		if val.Type() != ValTensor {
			break
		}
		return witnessTensorKind(t, val)
	default:
		return v.witnessOther(t, val, env)
	}
	return got
}

// witnessTensorKind tensörün öğe cinsini yazılan cinsle karşılaştırır (15.1). Bugün bütün
// tensörler float32; cins değerlerden ve tam sayılı işaretten kestirilir.
func witnessTensorKind(t *ast.TypeRef, val Value) string {
	want := codes.TypeFloat
	if len(t.Args) == 1 {
		want = t.Args[0].Token.Code
	}
	wk, ok := val.(WitnessTensorKind)
	if !ok {
		return ""
	}
	kind := wk.WitnessKind()
	switch want {
	case codes.TypeFloat:
		if kind == "Int" {
			return "gelen tam sayılı tensör (TR_Tensor[TR_Int] olmalı)"
		}
	case codes.TypeInt:
		if kind == "Float" {
			return "gelen kesirli değerli tensör (TR_Tensor[TR_Float])"
		}
	case codes.TypeBool:
		if kind != "Bool" {
			return "gelen 0/1 dışı değerli tensör (" + kind + ")"
		}
	default:
		return "bilinmeyen tensör cinsi"
	}
	return ""
}

// witnessOther dilin temel tipleri dışındaki tipler: motor tipleri (TR_Tokenizer, TR_Path ...),
// yapılar, durumlu tipler ve arayüzler.
func (v *VM) witnessOther(t *ast.TypeRef, val Value, env *Environment) string {
	if t.Token.Code != codes.None && t.Token.Code.Namespace() == codes.NamespaceType && val.Type() == TypeName(t.Token.Code) {
		return ""
	}
	if string(val.Type()) == t.Name {
		return ""
	}
	inst, isInst := val.(*StructInstanceVal)
	var def *StructDefVal
	if env != nil {
		if d, ok := env.Get(t.Name); ok {
			def, _ = d.(*StructDefVal)
		}
	}
	if def == nil && v.GlobalEnv != nil {
		if d, ok := v.GlobalEnv.Get(t.Name); ok {
			def, _ = d.(*StructDefVal)
		}
	}
	if isInst {
		if inst.Def.Name == t.Name || def != nil && inst.Def == def {
			return ""
		}
		if def != nil && def.Kind == ast.KindInterface {
			for name := range def.Methods {
				if _, ok := inst.Def.Methods[name]; !ok {
					return "gelen " + inst.Def.Name + " (arayüzün " + name + " metodu yok)"
				}
			}
			return ""
		}
		return "gelen yapı " + inst.Def.Name
	}
	if def == nil && t.Token.Code == codes.None {
		return "bilinmeyen tip adı (gelen " + string(val.Type()) + ")"
	}
	return "gelen " + string(val.Type())
}

// --- Gözlem kipi (GEÇİCİ) ---
//
// TR_TYPE_PROFILE=<dosya>: her kullanıcı fonksiyonu çağrısında parametrelerin, dönen değerin ve
// kurulan nesnenin alanlarının gerçek tipini (liste ve sözlükte öğelerin tipleriyle, tensörde
// kestirilen cinsle) "yer<TAB>tip" satırı olarak yazar; tip yazılmış olsun olmasın. Tiplerin kodun
// gerçek kullanımından çıkması için: çağrı yerlerinin hepsi bir arada görülür.

var profileFile = os.Getenv("TR_TYPE_PROFILE")

func profileOn() bool { return profileFile != "" }

func profileReport(place, typ string) { profileLine(place + "\t" + typ) }

var profileSeen = map[string]bool{}

func profileLine(line string) {
	witnessMu.Lock()
	defer witnessMu.Unlock()
	if profileSeen[line] {
		return
	}
	profileSeen[line] = true
	f, err := os.OpenFile(profileFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

func (v *VM) profileParams(fn *FuncVal, fk *funcCode, env *Environment) {
	for i, p := range fn.Parameters {
		if i >= len(fk.param) || p.NameToken.Line == 0 {
			continue
		}
		profileReport(fmt.Sprintf("%s:%d parametre %s.%s", v.witnessSourceName(fk.source), p.NameToken.Line, fn.Name, p.Name), describeValue(env.vals[fk.param[i]], 0))
	}
}

func (v *VM) profileReturn(fn *FuncVal, fk *funcCode, val Value) {
	line := 0
	if fn.Body != nil {
		line = fn.Body.Token.Line
	}
	profileReport(fmt.Sprintf("%s:%d dönüş %s", v.witnessSourceName(fk.source), line, fn.Name), describeValue(val, 0))
}

func (v *VM) profileFields(def *StructDefVal, fields []ast.StructField, inst *StructInstanceVal) {
	for _, f := range fields {
		v.profileField(def, f, inst.Fields[f.Name])
	}
}

func (v *VM) profileField(def *StructDefVal, f ast.StructField, val Value) {
	profileReport(fmt.Sprintf("%s:%d alan %s.%s", v.witnessSourceName(def.witnessSource), f.NameToken.Line, def.Name, f.Name), describeValue(val, 0))
}

func (v *VM) profileSetField(inst *StructInstanceVal, prop string, val Value) {
	for _, f := range inst.Def.Fields {
		if f.Name == prop {
			v.profileField(inst.Def, f, val)
		}
	}
}

// describeValue değerin tipini yazar: liste ve sözlükte görülen öğe tiplerinin birleşimiyle.
func describeValue(val Value, depth int) string {
	if val == nil {
		return "None"
	}
	switch x := val.(type) {
	case *NullVal:
		return "None"
	case *IntVal:
		return "Int"
	case *FloatVal:
		return "Float"
	case *StringVal:
		return "String"
	case *BoolVal:
		return "Bool"
	case *FuncVal, *BuiltinVal:
		return "Function"
	case *StructDefVal:
		return "StructDef(" + x.Name + ")"
	case *StructInstanceVal:
		return x.Def.Name
	case *TupleVal:
		parts := make([]string, len(x.Elements))
		for i, e := range x.Elements {
			parts[i] = describeValue(e, depth+1)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *ListVal:
		if depth > 4 {
			return "List[...]"
		}
		return "List[" + unionOf(x.Elements, depth) + "]"
	case *DictVal:
		if depth > 4 {
			return "Dict[...]"
		}
		var keys, vals []Value
		for i, k := range x.Order {
			if i >= 200 {
				break
			}
			keys = append(keys, x.KeyValue(k))
			vals = append(vals, x.Pairs[k])
		}
		return "Dict[" + unionOf(keys, depth) + ", " + unionOf(vals, depth) + "]"
	}
	if val.Type() == ValTensor {
		if wk, ok := val.(WitnessTensorKind); ok {
			return "Tensor[" + wk.WitnessKind() + "]"
		}
		return "Tensor"
	}
	return string(val.Type())
}

func unionOf(elems []Value, depth int) string {
	if len(elems) == 0 {
		return "?"
	}
	seen := map[string]bool{}
	var out []string
	for i, e := range elems {
		if i >= 200 {
			break
		}
		d := describeValue(e, depth+1)
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return strings.Join(out, " | ")
}

// witnessLitsOf çağrının tam sayı değişmezi olan argümanlarıdır (sıra numarası ya da ad): 3.13'e göre
// TR_Float beklenen yerde ondalıktır, tanık onları TAM→ONDALIK saymaz.
func witnessLitsOf(e *ast.CallExpression) map[string]bool {
	if !witnessOn() {
		return nil
	}
	var out map[string]bool
	isLit := isIntLit
	for i, a := range e.Arguments {
		if isLit(a) {
			if out == nil {
				out = map[string]bool{}
			}
			out[fmt.Sprint(i)] = true
		}
	}
	for _, a := range e.NamedArgs {
		if isLit(a.Value) {
			if out == nil {
				out = map[string]bool{}
			}
			out[a.Name] = true
		}
	}
	return out
}

func (v *VM) witnessMark(fn Value, lits map[string]bool) {
	if lits != nil {
		v.witnessLitFn, v.witnessLits = fn, lits
	}
}

// witnessTakeLits çağrılan fonksiyonun değişmez argümanlarını alır (başka bir çağrınınsa boş).
func (v *VM) witnessTakeLits(fn Value) map[string]bool {
	if v.witnessLitFn != fn {
		return nil
	}
	lits := v.witnessLits
	v.witnessLitFn, v.witnessLits = nil, nil
	return lits
}

func isIntLit(x ast.Expression) bool {
	if p, ok := x.(*ast.PrefixExpression); ok && p.Operator == "-" {
		x = p.Right
	}
	_, ok := x.(*ast.IntegerLiteral)
	return ok
}

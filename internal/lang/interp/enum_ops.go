package interp

// Durumlu tip (7.7) ve arayüz (7.8) yapının değerleriyle taşınır: tanım bir StructDefVal'dir (Kind),
// durumlu tipin değeri durumunu taşıyan bir StructInstanceVal'dir (Variant). Üye, metot, metin,
// eşitlik ve tip yolları yapınınkilerdir (struct_ops.go); burada yalnız onlara özgü olan durur.

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// VariantDef durumlu tipin bir durumudur: adı ve alanları (7.1'deki gibi, varsayılanlarıyla).
type VariantDef struct {
	Name   string
	Fields []ast.StructField
	// fieldCode alanların derlenmiş varsayılanlarıdır (Fields ile aynı sırada; yoksa nil).
	fieldCode []exprCode
	// bare alansız durumun tek değeridir (Durum.Bekliyor her okunuşta aynı değerdir).
	bare *StructInstanceVal
}

// typeDef durumlu tip ya da arayüz tanımını derler. Durumlu tipin metotları yapınınkiler gibi
// derlenir; arayüzün metotları yalnız imzadır, çalışma anında tanımın kendisi bir tiptir.
func (c *compiler) typeDef(k *compilerScope, s *ast.StructStatement) statementCode {
	type methodCode struct {
		lit     *ast.FunctionLiteral
		codeVal *funcCode
	}
	methodList := map[string]methodCode{}
	if s.Kind == ast.KindEnum {
		for name, fnLit := range s.Methods {
			methodList[name] = methodCode{lit: fnLit, codeVal: c.function(k, fnLit.Parameters, fnLit.Body, true)}
		}
	}
	variantCode := make([][]exprCode, len(s.Variants))
	for i, vr := range s.Variants {
		variantCode[i] = c.fieldDefaults(k, vr.Fields)
	}
	definition := identifier(k, s.Name)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		methods := make(map[string]*FuncVal, len(s.Methods))
		for name, lit := range s.Methods {
			methods[name] = &FuncVal{Name: name, Parameters: lit.Parameters, Body: lit.Body, Env: env, codeVal: methodList[name].codeVal, ReturnType: lit.ReturnType}
		}
		def := &StructDefVal{Name: s.Name, Methods: methods, Kind: s.Kind, fieldEnv: env, witnessSource: v.source}
		for i, vr := range s.Variants {
			vd := &VariantDef{Name: vr.Name, Fields: vr.Fields, fieldCode: variantCode[i]}
			if len(vr.Fields) == 0 {
				vd.bare = &StructInstanceVal{Def: def, Fields: map[string]Value{}, Variant: vd}
			}
			def.Variants = append(def.Variants, vd)
		}
		definition(env, def, true)
		return def, signalNone, nil
	}
}

// variant tanımın adı name olan durumudur (yoksa nil).
func (s *StructDefVal) variant(name string) *VariantDef {
	for _, vd := range s.Variants {
		if vd.Name == name {
			return vd
		}
	}
	return nil
}

// variantLabel durumun iletideki adıdır: Durum.Çalışıyor.
func variantLabel(def *StructDefVal, vd *VariantDef) string { return def.Name + "." + vd.Name }

// fieldNames alanların adlarıdır, tanım sırasıyla.
func fieldNames(fields []ast.StructField) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return names
}

// enumMember durumlu tipin kendisinin üyesidir (Durum.X): alansız durum değerdir; alanlı durum ancak
// çağrılarak kurulur, çağrısız okunması hatadır (ileti çağrıyı önerir). Metot nesnesizdir (7.3).
func enumMember(def *StructDefVal, e *ast.MemberExpression) (Value, error) {
	if vd := def.variant(e.Property); vd != nil {
		if vd.bare != nil {
			return vd.bare, nil
		}
		return nil, messages.At(e.Line(), 0, messages.New(messages.H2755, variantLabel(def, vd), variantCallText(vd)))
	}
	if _, ok := def.Methods[e.Property]; ok {
		return nil, messages.At(e.Line(), 0, messages.New(messages.H2756, def.Name, e.Property))
	}
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2765, def.Name, e.Property, enumHint(def, e.Property)))
}

// variantCallText alanlı durumun kurma yazımının argümanlarıdır: adım: ..., kayıp: ...
func variantCallText(vd *VariantDef) string {
	text := ""
	for i, f := range vd.Fields {
		if i > 0 {
			text += ", "
		}
		text += f.Name + ": ..."
	}
	return text
}

// constructVariant durumlu tipin bir durumunu kurar (Durum.Çalışıyor(5)): argümanlar 6.5'teki gibi,
// alanlar tanım sırasıyla ya da adla; varsayılanı olmayan ve verilmeyen alan hatadır (7.2 gibi).
// Alansız durum çağrılmaz (ileti çağrısız yazımı önerir).
func (v *VM) constructVariant(def *StructDefVal, mem *ast.MemberExpression, args []Value, named map[string]Value, line int) (Value, error) {
	vd := def.variant(mem.Property)
	if vd == nil {
		if _, ok := def.Methods[mem.Property]; ok {
			return nil, messages.At(line, 0, messages.New(messages.H2756, def.Name, mem.Property))
		}
		return nil, messages.At(line, 0, messages.New(messages.H2765, def.Name, mem.Property, enumHint(def, mem.Property)))
	}
	label := variantLabel(def, vd)
	if vd.bare != nil {
		return nil, messages.At(line, 0, messages.New(messages.H2757, label))
	}
	if err := checkFieldArgs(label, vd.Fields, len(args), named, line, true); err != nil {
		return nil, err
	}
	inst := &StructInstanceVal{Def: def, Fields: make(map[string]Value, len(vd.Fields)), Variant: vd}
	for i, f := range vd.Fields {
		switch {
		case i < len(args):
			inst.Fields[f.Name] = args[i]
		case named[f.Name] != nil:
			inst.Fields[f.Name] = named[f.Name]
		case f.DefaultValue != nil && vd.fieldCode[i] != nil:
			val, err := vd.fieldCode[i](v, def.fieldEnv)
			if err != nil {
				return nil, err
			}
			inst.Fields[f.Name] = val
		default:
			return nil, messages.At(line, 0, messages.New(messages.H2750, label, f.Name))
		}
		escape(inst.Fields[f.Name])
	}
	if witnessOn() {
		v.witnessFields(def, vd.Fields, inst)
	}
	if profileOn() {
		v.profileFields(def, vd.Fields, inst)
	}
	return inst, nil
}

// variantCallee adlı argümanlı çağrıda (Durum.Çalışıyor(adım: 5)) çağrılan değerdir: durumu kuran bir
// yerleşik. Yalnız çağrı yolu verir; çağrısız okuma enumMember'ın hatasıdır.
func (v *VM) variantCallee(def *StructDefVal, mem *ast.MemberExpression) Value {
	return &BuiltinVal{Name: def.Name + "." + mem.Property, Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		return vm.constructVariant(def, mem, args, named, mem.Line())
	}}
}

// enumArm durumlu tip desenli bir eşle kolunun derlenmiş hâlidir (7.7). Desen tanımı ve durumu
// çalışma anında bulur; bağlanan adlar yalnız kolun kendi ortamındadır.
type enumArm struct {
	pattern *ast.EnumPattern
	enum    exprCode
	info    *scopeInfo
	slots   []int // Names'in kolun ortamındaki yuvaları ("_" için -1)
	// matched eşleşen değeri kolun gövdesine taşır: desen değeri ve gövde aynı ortamla çağrılır
	// (anahtar o ortamdır; aynı kolu çalıştıran görevler birbirini görmez).
	matched sync.Map
}

// enumArmCode durumlu tip desenli kolun desenini, gövdesini ve değerini derler; desen durumlu tip deseni
// değilse ok=false.
func (c *compiler) enumArmCode(k *compilerScope, mc ast.MatchCase) (pattern exprCode, body statementCode, value exprCode, ok bool) {
	if len(mc.Patterns) != 1 {
		return nil, nil, nil, false
	}
	pat, isEnum := mc.Patterns[0].(*ast.EnumPattern)
	if !isEnum {
		return nil, nil, nil, false
	}
	arm := &enumArm{pattern: pat, enum: c.expression(k, pat.Enum)}
	sub := subScope(k)
	for _, id := range pat.Names {
		if id.Value == "_" {
			arm.slots = append(arm.slots, -1)
			continue
		}
		sub.define(id.Value)
		arm.slots = append(arm.slots, sub.info.slot(id.Value))
	}
	var block statementCode
	var expr exprCode
	if mc.Body != nil {
		block = c.blockStatement(sub, mc.Body)
	}
	if mc.ValueExpr != nil {
		expr = c.expression(sub, mc.ValueExpr)
	}
	arm.info = sub.info
	pattern = func(v *VM, env *Environment) (Value, error) {
		return arm.patternValue(v, env)
	}
	if block != nil {
		body = func(v *VM, env *Environment) (Value, flowSignal, error) {
			armEnv := arm.bind(env)
			defer armEnv.closeEnv()
			return block(v, armEnv)
		}
	}
	if expr != nil {
		value = func(v *VM, env *Environment) (Value, error) {
			armEnv := arm.bind(env)
			defer armEnv.closeEnv()
			return expr(v, armEnv)
		}
	}
	return pattern, body, value, true
}

var _ Equaler = (*enumPatternVal)(nil)

// enumPatternVal bir kolun desen değeridir: konu bu durumdaysa eşleşir ve eşleşen değeri kola bırakır.
type enumPatternVal struct {
	arm *enumArm
	env *Environment
	def *StructDefVal
	vd  *VariantDef
}

func (p *enumPatternVal) Type() ValueType { return ValueType(p.def.Name) }
func (p *enumPatternVal) Inspect() string { return variantLabel(p.def, p.vd) }
func (p *enumPatternVal) IsTruthy() bool  { return true }

// Equal konunun desenin durumunda olup olmadığını söyler (eşle, 5.6).
func (p *enumPatternVal) Equal(other Value) bool {
	inst, ok := other.(*StructInstanceVal)
	if !ok || inst.Def != p.def || inst.Variant != p.vd {
		return false
	}
	p.arm.matched.Store(p.env, inst)
	return true
}

// patternValue deseni çalışma anında çözer: tanım durumlu tip, durum onun bir durumu ve bağlanan ad
// sayısı alan sayısıyla aynı olmalıdır; değilse kol denenirken hatadır (4. katmanın denetimiyle aynı kod).
func (a *enumArm) patternValue(v *VM, env *Environment) (Value, error) {
	line := a.pattern.Line()
	ev, err := a.enum(v, env)
	if err != nil {
		return nil, err
	}
	def, ok := ev.(*StructDefVal)
	if !ok || def.Kind != ast.KindEnum {
		return nil, messages.At(line, 0, messages.New(messages.H2758, a.pattern.Variant))
	}
	vd := def.variant(a.pattern.Variant)
	if vd == nil {
		return nil, messages.At(line, 0, messages.New(messages.H2765, def.Name, a.pattern.Variant, enumHint(def, a.pattern.Variant)))
	}
	if len(a.pattern.Names) != len(vd.Fields) {
		return nil, messages.At(line, 0, messages.New(messages.H2759, variantLabel(def, vd), len(vd.Fields), len(a.pattern.Names)))
	}
	return &enumPatternVal{arm: a, env: env, def: def, vd: vd}, nil
}

// bind kolun ortamını açar ve eşleşen değerin alanlarını bağlanan adlara yazar.
func (a *enumArm) bind(env *Environment) *Environment {
	armEnv := newEnv(env, a.info)
	if x, ok := a.matched.LoadAndDelete(env); ok {
		inst := x.(*StructInstanceVal)
		for i, slot := range a.slots {
			if slot >= 0 {
				armEnv.vals[slot] = inst.Fields[inst.Variant.Fields[i].Name]
			}
		}
	}
	return armEnv
}

// enumOf değer bir durumlu tipin kendisi mi (Durum.X(...) çağrısının alıcısı).
func enumOf(obj Value) (*StructDefVal, bool) {
	def, ok := obj.(*StructDefVal)
	return def, ok && def.Kind == ast.KindEnum
}

// callVariant konumlu argümanlı Durum.Çalışıyor(5) çağrısıdır.
func (v *VM) callVariant(def *StructDefVal, mem *ast.MemberExpression, argCode []exprCode, env *Environment, line int) (Value, error) {
	args, err := arguments(v, env, argCode)
	if err != nil {
		return nil, err
	}
	escapeAll(args, nil)
	return v.constructVariant(def, mem, args, nil, line)
}

// enumHint durumlu tipte olmayan bir adın ipucudur: 2.3a'nın uzaklığıyla en yakın durum ya da metot.
func enumHint(def *StructDefVal, prop string) *messages.Error {
	names := make([]string, 0, len(def.Variants)+len(def.Methods))
	for _, vd := range def.Variants {
		names = append(names, vd.Name)
	}
	for name := range def.Methods {
		names = append(names, name)
	}
	slices.Sort(names)
	return hint.ClosestSuggestion(prop, names, nil)
}

// enumKeyError durumlu tipin değeri sözlük anahtarı olabilir mi (3.7): değer değişmez (7.7), alanlarının
// hepsi anahtar olabiliyorsa anahtardır. Liste gibi paylaşılan bir alan değişebilir: o zaman değil.
func enumKeyError(inst *StructInstanceVal) error {
	for _, f := range inst.Variant.Fields {
		if err := DictKeyError(inst.Fields[f.Name]); err != nil {
			return messages.New(messages.H2766, variantLabel(inst.Def, inst.Variant), f.Name, err)
		}
	}
	return nil
}

// enumKey durumlu tipin değerinin sözlükteki anahtarıdır: tanım, durum ve alanların anahtarları. Eşitliği
// '=='nin eşitliğidir (aynı durum, eşit alanlar; 7.7). Alan anahtarları uzunluklarıyla yazılır, böylece
// hiçbir metin alanı başka bir değerin anahtarıyla çakışmaz.
func enumKey(inst *StructInstanceVal) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%senum:%p.%s", nonStringKeyPrefix, inst.Def, inst.Variant.Name)
	for _, f := range inst.Variant.Fields {
		k := DictKey(inst.Fields[f.Name])
		fmt.Fprintf(&sb, ":%d:%s", len(k), k)
	}
	return sb.String()
}

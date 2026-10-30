package interp

// Yapının (7. bölüm) yorumlayıcıdaki işleri: kurma, alan okuma ve atama, eşitlik ve metin.
// Genel yollar (çağrı, üye, atama, eşitlik, yazdırma) yapı değerinde buraya yönlendirir.

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkFieldArgs alanlarla kurmanın (kurucusu olmayan yapı, durumlu tipin durumu) argümanlarını
// alanlara göre denetler: alan sayısından fazla konumlu argüman, alan olmayan bir adla verilen argüman
// ve hem sırayla hem adla verilen alan hatadır. label iletideki addır (Nokta, Durum.Çalışıyor); state
// durumlu tipin durumu için durumun iletilerini seçer.
func checkFieldArgs(label string, fields []ast.StructField, nargs int, named map[string]Value, line int, state bool) error {
	if nargs <= len(fields) && len(named) == 0 {
		return nil
	}
	unknown, tooMany, twice := messages.H2418, messages.H2419, messages.H2420
	if state {
		unknown, tooMany, twice = messages.H2767, messages.H2768, messages.H2769
	}
	nameList := fieldNames(fields)
	alignNamed(named, nameList)
	for k := range named {
		if !slices.Contains(nameList, k) {
			return messages.At(line, 0, messages.New(unknown, label, k, hint.ClosestSuggestion(k, nameList, nil)))
		}
	}
	if nargs > len(fields) {
		return messages.At(line, 0, messages.New(tooMany, label, len(fields), strings.Join(nameList, ", "), nargs))
	}
	for i, f := range nameList[:nargs] {
		if _, ok := named[f]; ok {
			return messages.At(line, 0, messages.New(twice, label, f, i+1))
		}
	}
	return nil
}

// construct yapının çağrısıdır (7.2, 7.4). Kurucusu olmayan yapıda alanlar tanım sırasıyla konumlu
// argümanlardan, adlı argümanlardan ya da varsayılanlardan dolar; varsayılanı olmayan ve verilmeyen
// alan hatadır. Kurucu varsa nesne yalnız onun imzasıyla kurulur: varsayılanlı alanlar kurucunun
// gövdesinden önce, tanım sırasıyla değerini alır, argümanlar yalnız kurucuya gider.
func (v *VM) construct(fn *StructDefVal, args []Value, named map[string]Value, line int) (Value, error) {
	switch fn.Kind {
	case ast.KindEnum:
		// durumlu tip çağrılmaz: değeri bir durumudur (7.7)
		return nil, messages.At(line, 0, messages.New(messages.H2761, fn.Name))
	case ast.KindInterface:
		return nil, messages.At(line, 0, messages.New(messages.H2762, fn.Name))
	}
	inst := &StructInstanceVal{
		Def:    fn,
		Fields: make(map[string]Value),
	}
	if fn.fieldCode == nil {
		// Derleyici dışında kurulmuş tanım: varsayılanlar genel kapsamda değerlendirilir
		c := v.newCompiler()
		fn.fieldCode = c.fieldDefaults(c.root, fn.Fields)
		fn.fieldEnv = v.GlobalEnv
	}
	fieldEnv := fn.fieldEnv
	if fieldEnv == nil {
		fieldEnv = v.GlobalEnv
	}
	initMethod, withCtor := fn.Methods[ctorName]
	if withCtor {
		// argümanlar kurucunun imzasına göre denetlenir (gövdeden önce: varsayılanlar boşa hesaplanmaz)
		if err := checkArity(initMethod, fn, len(args), named, line); err != nil {
			return nil, err
		}
	} else if err := checkFieldArgs(fn.Name, fn.Fields, len(args), named, line, false); err != nil {
		// kurucu yoksa argümanlar alanlara gider: fazlası ya da bilinmeyen adlısı hatadır
		return nil, err
	}
	for i, f := range fn.Fields {
		switch {
		case !withCtor && i < len(args):
			inst.Fields[f.Name] = args[i]
		case !withCtor && named[f.Name] != nil:
			inst.Fields[f.Name] = named[f.Name]
		case f.DefaultValue != nil && i < len(fn.fieldCode) && fn.fieldCode[i] != nil:
			defVal, err := fn.fieldCode[i](v, fieldEnv)
			if err != nil {
				return nil, err
			}
			inst.Fields[f.Name] = defVal
		case withCtor:
			// kurucu atar; atanmadan okunması 4a'nın denetimidir (7.4)
			inst.Fields[f.Name] = Null
		default:
			return nil, messages.At(line, 0, messages.New(messages.H2750, fn.Name, f.Name))
		}
		escape(inst.Fields[f.Name]) // nesne alanına yazılan değer kaçar
	}
	if withCtor {
		boundInit := &FuncVal{
			Name:       initMethod.Name,
			Parameters: initMethod.Parameters,
			Body:       initMethod.Body,
			Env:        initMethod.Env,
			BoundSelf:  inst,
			codeVal:    initMethod.codeVal,
			ReturnType: initMethod.ReturnType,
		}
		if _, err := v.ApplyFunction(boundInit, args, named, line); err != nil {
			return nil, err
		}
	}
	if witnessOn() {
		v.witnessFields(fn, fn.Fields, inst)
	}
	return inst, nil
}

// structMember nesnenin alanını ya da (bağlı fonksiyon olarak) metodunu döndürür.
func structMember(inst *StructInstanceVal, e *ast.MemberExpression) (Value, error) {
	if val, ok := inst.Fields[e.Property]; ok {
		if inst.Variant != nil {
			// durumun alanları yalnız eşlemeyle okunur (7.7)
			return nil, messages.At(e.Line(), 0, messages.New(messages.H2763, variantLabel(inst.Def, inst.Variant), e.Property))
		}
		return val, nil
	}
	if method, ok := inst.Def.Methods[e.Property]; ok {
		return &FuncVal{
			Name:       method.Name,
			Parameters: method.Parameters,
			Body:       method.Body,
			Env:        method.Env,
			BoundSelf:  inst,
			codeVal:    method.codeVal,
			ReturnType: method.ReturnType,
		}, nil
	}
	// '?.' yalnız yokluğu açar (4.8): var olan nesnede olmayan alan '.' ile aynı hatadır (7.5)
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2468, inst.Def.Name, e.Property, memberHint(inst.Def, e.Property)))
}

// memberHint yapıda olmayan bir üyenin ipucudur (7.5): adı 2.3a'nın uzaklığıyla en yakın alan ya da
// metot; yakın ad yoksa alanı yapıya tipiyle eklemek.
func memberHint(def *StructDefVal, prop string) *messages.Error {
	names := make([]string, 0, len(def.Fields)+len(def.Methods))
	for _, f := range def.Fields {
		names = append(names, f.Name)
	}
	for name := range def.Methods {
		names = append(names, name)
	}
	slices.Sort(names)
	if h := hint.ClosestSuggestion(prop, names, nil); h != nil {
		return h
	}
	return messages.New(messages.H2754, prop, def.Name)
}

// structDefMember yapının kendisinin (nesnesiz) üyesine erişimdir (N.m): yapının nesnesiz
// metodu ve alanı yoktur (7.3). İleti nesneyi kurmayı önerir; yapı argümansız kurulabiliyorsa
// önerilen yazım olduğu gibi çalışır.
func structDefMember(def *StructDefVal, e *ast.MemberExpression) (Value, error) {
	switch def.Kind {
	case ast.KindEnum:
		return enumMember(def, e)
	case ast.KindInterface:
		return nil, messages.At(e.Line(), 0, messages.New(messages.H2760, def.Name, e.Property))
	}
	args := "..."
	if constructibleBare(def) {
		args = ""
	}
	if m, ok := def.Methods[e.Property]; ok {
		margs := "..."
		if defaultsSuffice(m, 0) {
			margs = ""
		}
		return nil, messages.At(e.Line(), 0, messages.New(messages.H2751, def.Name, e.Property, args, margs))
	}
	for _, f := range def.Fields {
		if f.Name == e.Property {
			return nil, messages.At(e.Line(), 0, messages.New(messages.H2752, def.Name, e.Property, args))
		}
	}
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2468, def.Name, e.Property, memberHint(def, e.Property)))
}

// constructibleBare yapı argümansız kurulabilir mi: kurucusu varsa parametrelerinin hepsinin,
// yoksa alanlarının hepsinin varsayılanı var.
func constructibleBare(def *StructDefVal) bool {
	if init, ok := def.Methods[ctorName]; ok {
		return defaultsSuffice(init, 0)
	}
	for _, f := range def.Fields {
		if f.DefaultValue == nil {
			return false
		}
	}
	return true
}

// setStructField nesne.alan = değer atamasıdır. Nesnenin alanları her zaman yapı
// tanımındakilerdir (7.5): tanımda olmayan alana ve metoda atama yapılamaz.
func (v *VM) setStructField(inst *StructInstanceVal, prop string, codeVal uint8, rightVal Value, line int) error {
	if inst.Variant != nil {
		// durumlu tipin değeri değişmez (7.7)
		return messages.At(line, 0, messages.New(messages.H2764, variantLabel(inst.Def, inst.Variant), prop))
	}
	cur, field := inst.Fields[prop]
	if !field {
		if _, meth := inst.Def.Methods[prop]; meth {
			return messages.At(line, 0, messages.New(messages.H2404, prop, inst.Def.Name))
		}
		return messages.At(line, 0, messages.New(messages.H2405, inst.Def.Name, prop, memberHint(inst.Def, prop)))
	}
	if cur == nil {
		cur = Null
	}
	finalVal, err := v.assignOp(codeVal, cur, rightVal, line)
	if err != nil {
		return err
	}
	escape(finalVal)
	if witnessOn() {
		v.witnessSetField(inst, prop, finalVal)
	}
	inst.Fields[prop] = finalVal
	return nil
}

// structsEqual aynı yapıdan iki nesneyi alanlarıyla, tanım sırasıyla karşılaştırır (7.6);
// başka yapının nesnesi ya da başka değer kimlikle karşılaştırılır.
func structsEqual(x *StructInstanceVal, b Value, nestDepth int) bool {
	if y, ok := b.(*StructInstanceVal); ok && x.Def == y.Def && x.Variant == y.Variant {
		fields := x.Def.Fields
		if x.Variant != nil {
			fields = x.Variant.Fields
		}
		for _, f := range fields {
			va, okA := x.Fields[f.Name]
			vb, okB := y.Fields[f.Name]
			if okA != okB || (okA && !valuesEqual(va, vb, nestDepth+1)) {
				return false
			}
		}
		return true
	}
	return Value(x) == b
}

// structComparisonError '=='nin bir nesneyle başlayan karşılaştırmasını structsEqual'dan önce gezer
// (7.6): fonksiyon ya da tensör tutan alan ve karşılaştırılmakta olan bir nesne çiftine yeniden varmak
// (kendine bağlanan nesneler) çalışma hatasıdır. Gezinti comparisonError'ınki gibidir: aynı yapıdan
// (durumlu tipte aynı durumdan) nesnelerin her alan çifti, boyu aynı kapların her öğe çifti; başka
// değerler comparisonError'a gider. Çiftlerin kümesi gezintinin yolundadır: bir çift bitince çıkar.
func structComparisonError(x *StructInstanceVal, b Value, nestDepth int) error {
	return structComparison{}.walk(x, b, nestDepth)
}

type structComparison map[[2]*StructInstanceVal]bool

func (w structComparison) walk(a, b Value, nestDepth int) error {
	if nestDepth >= equalityDepth {
		return nil
	}
	switch x := a.(type) {
	case *StructInstanceVal:
		y, ok := b.(*StructInstanceVal)
		if !ok {
			// a'nın yerine b ile: comparisonError nesneye dönüp buraya gelmez
			return comparisonError(b, a, nestDepth)
		}
		if x != y && x.Def == y.Def && x.Variant == y.Variant {
			return w.fields(x, y, nestDepth)
		}
		return nil
	case *ListVal:
		if y, ok := b.(*ListVal); ok && x != y && len(x.Elements) == len(y.Elements) {
			for i := range x.Elements {
				if err := w.walk(x.Elements[i], y.Elements[i], nestDepth+1); err != nil {
					return err
				}
			}
			return nil
		}
	case *DictVal:
		if y, ok := b.(*DictVal); ok && x != y && len(x.Pairs) == len(y.Pairs) {
			for k, va := range x.Pairs {
				if vb, ok := y.Pairs[k]; ok {
					if err := w.walk(va, vb, nestDepth+1); err != nil {
						return err
					}
				}
			}
			return nil
		}
	case *TupleVal:
		return tupleComparisonError(x, b, nestDepth, w.walk) // tuple_ops.go
	}
	return comparisonError(a, b, nestDepth)
}

func (w structComparison) fields(x, y *StructInstanceVal, nestDepth int) error {
	pair := [2]*StructInstanceVal{x, y}
	label, fields := x.Def.Name, x.Def.Fields
	if x.Variant != nil {
		label, fields = variantLabel(x.Def, x.Variant), x.Variant.Fields
	}
	if w[pair] {
		return messages.New(messages.H2771, label, plainField(x, y, ""))
	}
	w[pair] = true
	defer delete(w, pair)
	for _, f := range fields {
		va, okA := x.Fields[f.Name]
		vb, okB := y.Fields[f.Name]
		if !okA || !okB {
			continue
		}
		if va.Type() != ValNull && vb.Type() != ValNull {
			for _, v := range []Value{va, vb} {
				kind := ""
				if isFunction(v) {
					kind = "function"
				} else if v.Type() == ValTensor {
					kind = "tensor"
				}
				if kind != "" {
					return messages.New(messages.H2770, label, f.Name, kind, plainField(x, y, f.Name))
				}
			}
		}
		if err := w.walk(va, vb, nestDepth+1); err != nil {
			return err
		}
	}
	return nil
}

// plainField iletinin önereceği alandır: iki nesnede de sayı, metin ya da mantık değeri tutan ilk alan
// (except dışında). Durumlu tipin alanı yalnız eşlemeyle okunur (7.7); onda öneri yoktur.
func plainField(x, y *StructInstanceVal, except string) string {
	if x.Variant != nil {
		return ""
	}
	for _, f := range x.Def.Fields {
		if f.Name == except {
			continue
		}
		switch x.Fields[f.Name].(type) {
		case *IntVal, *FloatVal, *StringVal, *BoolVal:
			if y.Fields[f.Name] != nil && y.Fields[f.Name].Type() == x.Fields[f.Name].Type() {
				return f.Name
			}
		}
	}
	return ""
}

// structCycleText yazılmakta olan bir nesneye yeniden varılınca yazılan metindir (7.6).
func structCycleText(*StructInstanceVal, *codes.Language) string {
	return "..."
}

// writeStruct nesnenin metnini kurma yazımıyla yazar (7.6): yapının adı ve alanları adlarıyla, tanım
// sırasıyla: Nokta(x: 3.0, y: 4.0). Metin programa kopyalanınca aynı değeri kurar.
func writeStruct(sb *strings.Builder, x *StructInstanceVal, lang *codes.Language, filePath []Value) {
	sb.WriteString(shownName(x.Def.Name, lang))
	fields := x.Def.Fields
	if x.Variant != nil {
		// durumlu tipin değeri: Durum.Bekliyor, Durum.Çalışıyor(adım: 5) (7.7)
		sb.WriteByte('.')
		sb.WriteString(shownName(x.Variant.Name, lang))
		if x.Variant.bare != nil {
			return
		}
		fields = x.Variant.Fields
	}
	sb.WriteByte('(')
	first := true
	for _, f := range fields {
		if val, ok := x.Fields[f.Name]; ok {
			if !first {
				sb.WriteString(", ")
			}
			first = false
			sb.WriteString(shownName(f.Name, lang))
			sb.WriteString(": ")
			sb.WriteString(writeElement(val, lang, filePath)) // metin tırnaklı (3.14), listedeki gibi
		}
	}
	sb.WriteByte(')')
}

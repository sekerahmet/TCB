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

// checkStructArgs kurucusu olmayan bir yapının oluşturulmasındaki argümanları alanlara
// göre denetler: alan sayısından fazla konumlu argüman ya da alan olmayan bir adla verilen
// argüman hatadır.
func checkStructArgs(def *StructDefVal, nargs int, named map[string]Value, line int) error {
	if nargs <= len(def.Fields) && len(named) == 0 {
		return nil
	}
	nameList := make([]string, len(def.Fields))
	for i, f := range def.Fields {
		nameList[i] = f.Name
	}
	alignNamed(named, nameList)
	for k := range named {
		if !slices.Contains(nameList, k) {
			return messages.At(line, 0, messages.New(messages.H2418, def.Name, k, hint.ClosestSuggestion(k, nameList, nil)))
		}
	}
	if nargs > len(def.Fields) {
		return messages.At(line, 0, messages.New(messages.H2419, def.Name, len(def.Fields), strings.Join(nameList, ", "), nargs))
	}
	for i, f := range nameList[:nargs] {
		if _, ok := named[f]; ok {
			return messages.At(line, 0, messages.New(messages.H2420, def.Name, f, i+1))
		}
	}
	return nil
}

// construct yapının çağrısıdır (7.2, 7.4). Kurucusu olmayan yapıda alanlar tanım sırasıyla konumlu
// argümanlardan, adlı argümanlardan ya da varsayılanlardan dolar; varsayılanı olmayan ve verilmeyen
// alan hatadır. Kurucu varsa nesne yalnız onun imzasıyla kurulur: varsayılanlı alanlar kurucunun
// gövdesinden önce, tanım sırasıyla değerini alır, argümanlar yalnız kurucuya gider.
func (v *VM) construct(fn *StructDefVal, args []Value, named map[string]Value, line int) (Value, error) {
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
	} else if err := checkStructArgs(fn, len(args), named, line); err != nil {
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
		}
		if _, err := v.ApplyFunction(boundInit, args, named, line); err != nil {
			return nil, err
		}
	}
	return inst, nil
}

// structMember nesnenin alanını ya da (bağlı fonksiyon olarak) metodunu döndürür.
func structMember(inst *StructInstanceVal, e *ast.MemberExpression) (Value, error) {
	if val, ok := inst.Fields[e.Property]; ok {
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
		}, nil
	}
	if e.Safe {
		return Null, nil
	}
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2468, inst.Def.Name, e.Property, (*messages.Error)(nil)))
}

// structDefMember yapının kendisinin (nesnesiz) üyesine erişimdir (N.m): yapının nesnesiz
// metodu ve alanı yoktur (7.3). İleti nesneyi kurmayı önerir; yapı argümansız kurulabiliyorsa
// önerilen yazım olduğu gibi çalışır.
func structDefMember(def *StructDefVal, e *ast.MemberExpression) (Value, error) {
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
	names := make([]string, 0, len(def.Fields)+len(def.Methods))
	for _, f := range def.Fields {
		if f.Name == e.Property {
			return nil, messages.At(e.Line(), 0, messages.New(messages.H2752, def.Name, e.Property, args))
		}
		names = append(names, f.Name)
	}
	for name := range def.Methods {
		names = append(names, name)
	}
	slices.Sort(names)
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2468, def.Name, e.Property, hint.ClosestSuggestion(e.Property, names, nil)))
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
	cur, field := inst.Fields[prop]
	if !field {
		if _, meth := inst.Def.Methods[prop]; meth {
			return messages.At(line, 0, messages.New(messages.H2404, prop, inst.Def.Name))
		}
		return messages.At(line, 0, messages.New(messages.H2405, inst.Def.Name, prop))
	}
	if cur == nil {
		cur = Null
	}
	finalVal, err := v.assignOp(codeVal, cur, rightVal, line)
	if err != nil {
		return err
	}
	escape(finalVal)
	inst.Fields[prop] = finalVal
	return nil
}

// structsEqual aynı yapıdan iki nesneyi alanlarıyla, tanım sırasıyla karşılaştırır (7.6);
// başka yapının nesnesi ya da başka değer kimlikle karşılaştırılır.
func structsEqual(x *StructInstanceVal, b Value, nestDepth int) bool {
	if y, ok := b.(*StructInstanceVal); ok && x.Def == y.Def {
		for _, f := range x.Def.Fields {
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

// structCycleText yazılmakta olan bir nesneye yeniden varılınca yazılan metindir (7.6).
func structCycleText(x *StructInstanceVal, lang *codes.Language) string {
	return shownName(x.Def.Name, lang) + "{...}"
}

// writeStruct nesnenin metnini yazar: yapının adı ve alanları, tanım sırasıyla (7.6).
func writeStruct(sb *strings.Builder, x *StructInstanceVal, lang *codes.Language, filePath []Value) {
	sb.WriteString(shownName(x.Def.Name, lang))
	sb.WriteByte('{')
	first := true
	for _, f := range x.Def.Fields {
		if val, ok := x.Fields[f.Name]; ok {
			if !first {
				sb.WriteString(", ")
			}
			first = false
			sb.WriteString(shownName(f.Name, lang))
			sb.WriteString(": ")
			sb.WriteString(writeItem(val, lang, filePath))
		}
	}
	sb.WriteByte('}')
}

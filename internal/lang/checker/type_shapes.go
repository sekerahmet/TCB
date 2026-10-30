package checker

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// Denetleyici tipleri metin olarak taşır; kabın metni öğe tipini, demetinki öğelerinin tiplerini de
// yazar (3.6, 3.6a, 3.7): TR_List[TR_Int], TR_Dict[TR_String, TR_Int], (TR_Int, TR_String). Öğe tipi
// bilinmeyen kap asıl adıyla kalır (TR_List). Bu dosya metni böler, kurar ve iki tipin uyumuna karar verir.

// typeShape bir tip metninin parçalarıdır: asıl ad ve kabın tip argümanları; demette asıl ad boştur,
// elems öğelerin tipleridir.
type typeShape struct {
	base  string
	args  []string
	elems []string
}

// shapeOf tip metnini parçalarına ayırır.
func shapeOf(t string) typeShape {
	if strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		return typeShape{elems: splitTypes(t[1 : len(t)-1])}
	}
	if i := strings.IndexByte(t, '['); i > 0 && strings.HasSuffix(t, "]") {
		return typeShape{base: t[:i], args: splitTypes(t[i+1 : len(t)-1])}
	}
	return typeShape{base: t}
}

// splitTypes virgülle ayrılmış tip metinlerini en dış düzeyde böler (iç içe köşeli parantezin ve
// parantezin içindeki virgül bölmez).
func splitTypes(text string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range text {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(text[start:]))
}

// containerType yazılan ya da çıkarılan bir kabın tipidir: TR_List'in bir, TR_Dict'in iki tip
// argümanı varsa onlarla (TR_List[TR_Int]), yoksa asıl adıyla. Öbür adların argümanı tipe girmez.
func containerType(name string, args []string) string {
	if name == typeList && len(args) == 1 || name == typeDict && len(args) == 2 {
		return name + "[" + strings.Join(args, ", ") + "]"
	}
	return name
}

// tupleType öğelerinin tipleri elems olan demetin tipidir (3.6a): (TR_Int, TR_String).
func tupleType(elems []string) string { return "(" + strings.Join(elems, ", ") + ")" }

// baseType tipin asıl adıdır: TR_List[TR_Int] → TR_List; demette boş.
func baseType(t string) string { return shapeOf(t).base }

// mapTypeNames tip metnindeki her adı f'nin verdiğiyle değiştirir; ayraçlar ([, ], (, ), virgül,
// boşluk, ?) kalır. İleti bileşik tipin adlarını böyle yazar (typeText, typeSpelling).
func mapTypeNames(t string, f func(string) string) string {
	var b strings.Builder
	start := -1
	for i, r := range t {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			b.WriteString(f(t[start:i]))
			start = -1
		}
		b.WriteRune(r)
	}
	if start >= 0 {
		b.WriteString(f(t[start:]))
	}
	return b.String()
}

// İki tipin uyumu: beklenen yere verilen tipte bir değer konabilir mi (3.13). Denetleyici yalnız
// tanıdığı tiplerde karar verir: çekirdek tipler ve programın yapıları. Arayüz, durumlu tip, tip
// parametresi ya da başka bir modülün tipi denetleyiciye yalnız bir addır; böyle bir ad başka bir
// kullanıcı tipiyle karşılaşınca uyumlu sayılır (yanlış alarm vermez). Kap tip argümanında
// değişmezdir (3.6): TR_List[TR_Int] TR_List[TR_Float] beklenen yere verilmez; değişmezdeki tam sayı
// yazımı ise ondalık beklenen yerde ondalıktır (3.13). Öğe tipi bilinmeyen kap asıl adıyla karşılaştırılır.

// compat iki tipin uyumuna karar verir. isStruct adın denetleyicinin bildiği bir yapı olup
// olmadığını söyler (nil: hiçbir yapı bilinmiyor); literal verilen değer bir değişmezdir.
type compat struct {
	isStruct func(string) bool
	literal  bool
}

func typesCompatible(expected, actual string) bool {
	return compat{}.types(expected, actual)
}

// assignable value'nun (tipi actual) beklenen tipin yerine verilip verilemeyeceğidir; programın
// yapılarını bilir.
func (c *Checker) assignable(expected, actual string, value ast.Expression) bool {
	return compat{isStruct: c.knownStruct, literal: literalValue(value)}.types(expected, actual)
}

// literalValue değer bir liste, sözlük ya da demet değişmezi mi (içindeki tam sayı yazımı ondalık
// beklenen yerde ondalıktır)?
func literalValue(value ast.Expression) bool {
	switch value.(type) {
	case *ast.ListLiteral, *ast.DictLiteral, *ast.TupleExpression:
		return true
	}
	return false
}

func (k compat) types(expected, actual string) bool {
	return k.fits(expected, actual, false)
}

// fits expected yerine actual konabilir mi; inner bir kabın tip argümanıdır.
func (k compat) fits(expected, actual string, inner bool) bool {
	e, a := shapeOf(expected), shapeOf(actual)
	if e.elems != nil || a.elems != nil {
		return k.tuplesFit(e, a)
	}
	e.base, a.base = normalizeTypeName(e.base), normalizeTypeName(a.base)
	if e.base == typeAny || a.base == typeAny || a.base == typeNull {
		return true
	}
	if e.base != a.base {
		return k.namesFit(e.base, a.base, inner)
	}
	if len(e.args) == 0 || len(a.args) != len(e.args) {
		return true
	}
	for i := range e.args {
		if !k.fits(e.args[i], a.args[i], true) {
			return false
		}
	}
	return true
}

// tuplesFit bir yanı demet olan iki tipin uyumudur (3.6a): iki demet aynı sayıda öğeli ve öğeleri
// sırayla uyumluysa uyar; demetin karşısında Herhangi ya da bilinmeyen bir ad (tip parametresi) her
// demeti karşılayabilir, öbür tipler karşılamaz.
func (k compat) tuplesFit(e, a typeShape) bool {
	if e.elems == nil || a.elems == nil {
		other := normalizeTypeName(e.base + a.base)
		return other == typeAny || other == typeNull || k.unknownName(other)
	}
	if len(e.elems) != len(a.elems) {
		return false
	}
	for i := range e.elems {
		if !k.fits(e.elems[i], a.elems[i], true) {
			return false
		}
	}
	return true
}

// namesFit ayrı iki asıl adın uyumudur. Tam sayı Ondalık beklenen yere atanabilir (kabın içinde
// yalnız değişmezde). Kabın içinde bilinmeyen ad (tip parametresi, arayüz) her tipi karşılayabilir.
func (k compat) namesFit(exp, act string, inner bool) bool {
	if exp == typeFloat && act == typeInt && (!inner || k.literal) {
		return true
	}
	if inner && (k.unknownName(exp) || k.unknownName(act)) {
		return true
	}
	// İki kullanıcı tipi: ikisi de bilinen yapıysa ayrıdır; biri arayüz ya da bilinmeyen bir tipse
	// değer onu karşılayabilir.
	return !coreTypes[exp] && exp != typeNull && !coreTypes[act] && !(k.structName(exp) && k.structName(act))
}

// structName ad bilinen bir yapının adı mı?
func (k compat) structName(name string) bool {
	return k.isStruct != nil && k.isStruct(name)
}

// unknownName ad denetleyicinin anlamını bilmediği bir tip adı mı (çekirdek tip, yokluk ya da bilinen yapı değil)?
func (k compat) unknownName(name string) bool {
	return !coreTypes[name] && name != typeNull && !k.structName(name)
}

// listType TR_List[elem]'dir.
func listType(elem string) string { return containerType(typeList, []string{elem}) }

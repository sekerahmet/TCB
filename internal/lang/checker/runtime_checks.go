package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tipi bilinmeyen değerin tipli yere girişi (1.2): uygunluk sınamasının iki kolu tipi bilinmeyeni
// her yere uygun sayar: verilen yanda TR_Any (type_shapes.go fits) ve öğe tipi bilinmeyen kap. Yedi
// tipli yerde (tanım, atama, programın fonksiyonuna argüman, dönüş, alan yazma, kurucu, kap öğesi)
// değer yalnız bu iki kolla uyuyorsa denetleyici onu işaretler (ast.RuntimeCheck); yorumlayıcı
// değeri girişte, denetleyicinin kuralıyla denetler (interp/runtime_checks.go). Tipi bilinen
// değere işaret konmaz.

// placeArg iletinin argümanlarında çalışma anının dolduracağı yuvadır (checkPlace).
type placeArg uint8

const (
	argGiven placeArg = iota + 1 // değerin tipi: denetimde çıkarılan, çalışırken değerin tipi
	argHint                      // ipucu: denetimde typeHint, çalışırken H1170
)

// place tipli bir yere girişin denetimidir. at hatanın yeri, value giren ifade (kap öğesinin
// bileşik atamasında nil), op bileşik atamanın işleci, assign o atamanın kendisi; divides
// typeHint'in bölme ipucunu açar. args iletinin argümanlarıdır, argGiven ve argHint yuvalarıyla.
type place struct {
	at       ast.Node
	assign   *ast.AssignStatement // bileşik atamanın kendisi: işaret ona konur
	value    ast.Expression
	expected string
	actual   string
	op       string
	divides  bool
	code     messages.Code
	args     []any
}

// checkPlace değerin tipli yere girişini denetler: uymuyorsa yerin koduyla denetim hatasıdır;
// yalnız tipi bilinmediği için uyuyorsa çalışma denetimi için işaretlenir.
func (c *Checker) checkPlace(p place) {
	if !c.assignable(p.expected, p.actual, p.value) {
		c.addErrorAt(p.at, messages.New(p.code, c.fillPlaceArgs(p)...))
		return
	}
	c.noteRuntimeCheck(p)
}

// fillPlaceArgs iletinin yuvalarını denetimin bildiğiyle doldurur.
func (c *Checker) fillPlaceArgs(p place) []any {
	args := make([]any, len(p.args))
	for i, a := range p.args {
		switch a {
		case argGiven:
			args[i] = c.typeArg(p.actual)
		case argHint:
			args[i] = c.typeHint(p.expected, p.actual, p.value, p.divides)
		default:
			args[i] = a
		}
	}
	return args
}

// noteRuntimeCheck uyan değerin yalnız tipi bilinmediği için uyup uymadığına bakar ve öyleyse
// işaretler. TR_JSON yer değeri kendisi kurar (3.7a, typed_places.go); ona işaret konmaz.
func (c *Checker) noteRuntimeCheck(p place) {
	if normalizeTypeName(shapeOf(p.expected).base) == typeJSON || c.placeCompat(p.value).strict(p.expected, p.actual, false) {
		return
	}
	key := ast.Node(p.value)
	if p.op != "" {
		key = p.assign
	}
	if c.runtimeChecks == nil {
		c.runtimeChecks = ast.RuntimeChecks{}
	}
	m := &ast.RuntimeCheck{Expected: p.expected, Op: p.op, Code: p.code, Given: -1, Hint: -1,
		Spelled: c.language() == nil, At: p.at, Structs: c.runtimeStructs()}
	m.Args = make([]any, len(p.args))
	for i, a := range p.args {
		switch a {
		case argGiven:
			m.Given = i
		case argHint:
			m.Hint, a = i, ""
		}
		m.Args[i] = a
	}
	// İpucunun yolu iletide ters tırnak içinde yazılır: birden çok satırlık ya da ters tırnaklı
	// yazım yolu kopyalanamaz kılar, ipucu yazılmaz.
	if p.value != nil && p.op == "" {
		if text, ok := c.exprText(p.value); ok && !strings.ContainsAny(text, "\n`") {
			m.Text = text
		}
	}
	c.runtimeChecks[key] = m
}

// runtimeStructs programın bilinen yapılarının adlarıdır; bir denetimin bütün işaretleri aynı
// kümeyi paylaşır.
func (c *Checker) runtimeStructs() map[string]bool {
	if c.runtimeStructNames == nil {
		c.runtimeStructNames = map[string]bool{}
		for name := range c.structs {
			c.runtimeStructNames[name] = true
		}
	}
	return c.runtimeStructNames
}

// takeRuntimeChecks denetimin işaretlerini programa yazar ve bir sonraki denetim için sıfırlar.
func (c *Checker) takeRuntimeChecks(prog *ast.Program) {
	prog.RuntimeChecks, c.runtimeChecks, c.runtimeStructNames = c.runtimeChecks, nil, nil
}

// strict fits'tir, ama verilen yanda TR_Any ve öğe tipi bilinmeyen kap (beklenen yanın tip
// argümanı varken) uymaz: değer yalnız bu iki kolla uyuyorsa false. Beklenen yanda TR_Any ya da öğe
// tipi yazılmamış kap varsa tipi bilinen değer uyar. Bileşik atamada value nil'dir.
func (k compat) strict(expected, actual string, inner bool) bool {
	e, a := shapeOf(expected), shapeOf(actual)
	if actual == typeNull {
		return k.fits(expected, actual, inner)
	}
	if e.elems != nil || a.elems != nil {
		if e.elems != nil && a.elems != nil {
			return len(e.elems) == len(a.elems) && k.allStrict(e.elems, a.elems)
		}
		if a.elems == nil && normalizeTypeName(a.base) == typeAny {
			return false // demet beklenen yere tipi bilinmeyen değer
		}
		return k.tuplesFit(e, a)
	}
	if inner && a.optional && !e.optional {
		return false
	}
	eb, ab := normalizeTypeName(e.base), normalizeTypeName(a.base)
	switch {
	case eb == typeAny:
		return true
	case ab == typeAny:
		return false
	case eb != ab:
		return k.namesFit(eb, ab, inner)
	case len(e.args) == 0:
		return true
	case len(a.args) != len(e.args):
		return false // öğe tipi bilinmeyen kap
	}
	return k.allStrict(e.args, a.args)
}

func (k compat) allStrict(expected, actual []string) bool {
	for i := range expected {
		if !k.strict(expected[i], actual[i], true) {
			return false
		}
	}
	return true
}

// Çalışma anının denetimi (interp/runtime_checks.go) denetleyicinin kuralını bu işlevlerle kullanır.

// RuntimeShape t tip metninin parçalarıdır: asıl ad (normalleşmiş), kabın tip argümanları, demetin
// öğeleri ve sondaki '?'.
func RuntimeShape(t string) (base string, args, elems []string, optional bool) {
	s := shapeOf(t)
	if s.base != "" {
		s.base = normalizeTypeName(s.base) // demetin asıl adı boştur, TR_Any'ye dönmez
	}
	return s.base, s.args, s.elems, s.optional
}

// RuntimeFits çalışırken tipi actual olan değerin expected yerine uyup uymadığıdır: denetleyicinin
// uygunluğu, tam sayının ondalık yerine geçmesi kapalı (çalışma anı TR_Int değeri çevirmez; 3.13'ün
// yazım istisnası yalnız denetimdedir). structs denetleyicinin bildiği yapıların adlarıdır.
func RuntimeFits(expected, actual string, inner bool, structs map[string]bool) bool {
	k := compat{isStruct: func(name string) bool { return structs[name] }, exact: true}
	return k.fits(expected, actual, inner)
}

// RuntimeCoreType name denetleyicinin çekirdek tiplerinden biri mi (tam aynılıkla karşılaştırılır)?
func RuntimeCoreType(name string) bool { return coreTypes[name] }

// RuntimeIsFunctionType t bir fonksiyon tipi mi: fonksiyon değerinde yalnız buna bakılır, imzaya
// bakılmaz (6.9; denetleyici de bakmaz).
func RuntimeIsFunctionType(t string) bool { return functionType(t) }

// RuntimeResultType bileşik atamanın sonucunun tipidir (x op= v): denetleyicinin arithmeticType'ı.
func RuntimeResultType(op, left, right string) string { return arithmeticType(op, left, right) }

// RuntimeTypeArg t tipinin ileti argümanıdır: spelled ise dosyanın kimlik yazımıyla (TR_Int),
// değilse iletinin dilinde (typeArg ile aynı seçim).
func RuntimeTypeArg(t string, spelled bool) messages.Localized {
	if spelled {
		return typeSpelling(t)
	}
	return typeText(t)
}

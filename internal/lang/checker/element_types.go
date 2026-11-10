package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kabın öğe tipi (3.6, 3.7, 4.10): liste tek tiplidir, sözlüğün değerleri tek tiptedir. Öğe tipi
// bilinen kabın indeksine başka tipte değer yazmak H1136, öğe tipleri ayrı iki listenin '+'sı (ve
// '+='sı) H1138'dir. Öğeleri başvuruyla paylaşılan (liste, sözlük, tensör, yapı; onları tutan demet)
// listenin '*' ile yinelenmesi H1139'dur: satırlar aynı öğe olurdu ([[0] * 3] * 3). Çalışma anı öğe
// tipini taşımadığı için üçü de bugün çalışınca sessizce geçer;
// denetim tipi bilinen yerde yakalar. Tipi bilinmeyen öğe ya da kap, öğe tipsiz kap (TR_List) ve
// fonksiyon karar vermez.

// distinctTypes a ile b'nin ayrı iki tip olduğudur: ikisi de denetleyicinin tanıdığı tip ve biri
// öbürünün yerine verilmez (kaplar tip argümanında değişmezdir, 3.6). Sayılar birbirinden ayrı
// değildir (4.5); fonksiyon tipleri burada karar vermez.
func (c *Checker) distinctTypes(a, b string) bool {
	if !c.knownType(a) || !c.knownType(b) || functionType(a) || functionType(b) {
		return false
	}
	k := compat{isStruct: c.knownStruct}
	return !k.types(a, b) && !k.types(b, a)
}

// functionType t bir fonksiyon tipi mi (kullanıcı fonksiyonu ya da yerleşik)?
func functionType(t string) bool {
	return strings.HasSuffix(normalizeTypeName(baseType(t)), "Function")
}

// checkElementAssign öğe tipi bilinen kabın indeksine atamayı denetler (3.6, 3.7; H1136): m[0] = "s",
// d["b"] = "s"; bileşik atamada (m[0] += 1.5) atanan değer işlemin sonucudur. Dilime atama ve
// açılmamış isteğe bağlı değer (H1135, optional_values.go) burada değildir.
func (c *Checker) checkElementAssign(s *ast.AssignStatement, valType string) {
	ix, ok := s.Target.(*ast.IndexExpression)
	if !ok {
		return
	}
	container := c.itemTypes[ix.Left] // inferNested kaydeder
	shape := shapeOf(container)
	elem, kind := "", "list"
	switch base := normalizeTypeName(shape.base); {
	case shape.optional || shape.elems != nil:
		return
	case base == typeList && len(shape.args) == 1 && c.itemTypes[ix.Index] != typeRange:
		elem = shape.args[0]
	case base == typeDict && len(shape.args) == 2:
		elem, kind = shape.args[1], "dict"
	default:
		return
	}
	value := s.Value
	if op := strings.TrimSuffix(string(s.Operator), "="); op != "" {
		valType, value = arithmeticType(op, elem, valType), nil
	}
	if bad, _ := c.unopened(elem, valType, value); bad != nil {
		return // H1135
	}
	// TR_none (TR_NoneType) isteğe bağlı olmayan öğeye girmez (3.5; DENETİM 400 kapanış K-N1)
	known := c.knownType(valType) || valType == typeNull
	if c.knownType(elem) && known && !functionType(valType) && !c.assignable(elem, valType, value) {
		c.addErrorAt(s.Value, messages.New(messages.H1136, c.typeArg(valType), c.typeArg(container), c.typeArg(elem), kind))
	}
}

// checkListPlus öğe tipleri ayrı iki listenin '+'sını denetler (4.10, 3.6; H1138): [1] + ["a"],
// m += ["a"].
func (c *Checker) checkListPlus(n ast.Node, left, right string) {
	l, r := shapeOf(left), shapeOf(right)
	if l.optional || r.optional || normalizeTypeName(l.base) != typeList || normalizeTypeName(r.base) != typeList {
		return
	}
	if c.distinctTypes(left, right) {
		c.addErrorAt(n, messages.New(messages.H1138, c.typeArg(left), c.typeArg(right)))
	}
}

// checkSequenceOperator metnin ve listenin '+' ve '*' işlecini denetler (4.10); n işlecin yeri,
// bileşik atamada (x *= y) sol yanın ifadesi nil'dir.
func (c *Checker) checkSequenceOperator(n ast.Node, op string, lExpr, rExpr ast.Expression, left, right string) {
	switch op {
	case "+":
		c.checkPlus(n, left, right)
		c.checkListPlus(n, left, right)
	case "*":
		c.checkSharedRepeat(n, lExpr, rExpr, left, right)
	}
}

// checkSharedRepeat öğeleri başvuruyla paylaşılan listenin '*' ile yinelenmesini denetler (4.10;
// H1139): [[0] * 3] * 3, 3 * [p]. Öneri tek öğeli liste değişmezinde her satırı üreteçle ayrı kurar.
func (c *Checker) checkSharedRepeat(n ast.Node, lExpr, rExpr ast.Expression, left, right string) {
	list, count, listType := lExpr, rExpr, left
	if normalizeTypeName(baseType(left)) != typeList {
		list, count, listType = rExpr, lExpr, right
	}
	s := shapeOf(listType)
	if s.optional || normalizeTypeName(s.base) != typeList || len(s.args) != 1 || !c.sharedElement(s.args[0]) {
		return
	}
	c.addErrorAt(n, messages.New(messages.H1139, c.typeArg(listType), c.repeatSuggestion(list, count)))
}

// sharedElement t tipindeki değer başvuruyla mı paylaşılır (3.6): liste, sözlük, tensör, programın
// yapısı ya da bunlardan birini tutan demet. Metin, sayı, mantık değeri, JSON ve tipi bilinmeyen değer
// paylaşılmaz sayılır.
func (c *Checker) sharedElement(t string) bool {
	s := shapeOf(t)
	for _, e := range s.elems {
		if c.sharedElement(e) {
			return true
		}
	}
	switch base := normalizeTypeName(s.base); base {
	case typeList, typeDict, typeTensor:
		return true
	default:
		return base != "" && c.knownStruct(base)
	}
}

// repeatSuggestion [E] * n'nin her satırı ayrı kuran yazımıdır: [E TR_for _ TR_in 0..n]. Liste tek
// öğeli bir değişmez değilse ya da yazımlar dosyada bilinmiyorsa boştur.
func (c *Checker) repeatSuggestion(list, count ast.Expression) string {
	lit, ok := list.(*ast.ListLiteral)
	if !ok || len(lit.Elements) != 1 || count == nil {
		return ""
	}
	elem, ok1 := c.exprText(lit.Elements[0])
	n, ok2 := c.exprText(count)
	if !ok1 || !ok2 {
		return ""
	}
	switch count.(type) {
	case *ast.IntegerLiteral, *ast.Identifier:
	default:
		n = "(" + n + ")"
	}
	return "[" + elem + " " + codes.Spelling(codes.KwFor) + " _ " + codes.Spelling(codes.KwIn) + " 0.." + n + "]"
}

// searchesList k, listede 4.5'in eşitliğiyle değer arayan metot mu (13.3).
func searchesList(k codes.Code) bool { return k == codes.MemContains || k == codes.MemIndexOf }

// checkContainsArg öğe tipi bilinen listenin .TR_contains'ine ve .TR_index_of'una verilen değeri denetler
// (3.6, 13.10: TR_List[T].TR_contains(x: T)): öğe tipinden ayrı tipte (distinctTypes) değer listede hiç
// bulunmaz ve motor hatasız TR_false ya da TR_none verir; denetim hatasıdır, H1142. Sayılar ayrı değildir (4.5:
// [1, 2].TR_contains(1.0) doğru). Değer metinse ve öğe sayıysa ya da tersiyse ileti tip çağrısıyla
// çevirmeyi önerir (TR_Int("2")). JSON (H1131), TR_none (4.5'in sınırı) ve tipi bilinmeyen değer
// burada değildir.
func (c *Checker) checkContainsArg(e *ast.CallExpression, given string) {
	recvType, arg := c.receivers[e], e.Arguments[0]
	s := shapeOf(recvType)
	if s.base != typeList || len(s.args) != 1 || s.optional || given == typeNull || requiredType(s.args[0]) == typeJSON ||
		requiredType(given) == typeJSON || !c.distinctTypes(requiredType(s.args[0]), requiredType(given)) {
		return
	}
	c.addErrorAt(arg, messages.New(messages.H1142, c.typeArg(recvType), c.typeArg(s.args[0]), c.typeArg(given), c.conversion(s.args[0], given, arg)))
}

// conversion değeri öğe tipine çeviren tip çağrısıdır: sayıdan metne (TR_String(n)) ve sayı yazılı
// metin değişmezinden sayıya (TR_Int("2")); öbür metin çalışırken çevrilemez. Değerin dosyadaki
// yazımı bilinmiyorsa "".
func (c *Checker) conversion(elem, given string, arg ast.Expression) string {
	number := func(t string) bool { return t == typeInt || t == typeFloat }
	ok := elem == typeString && number(given)
	if s, lit := arg.(*ast.StringLiteral); lit && given == typeString {
		ok = elem == typeInt && numberText(s.Value, false) || elem == typeFloat && numberText(s.Value, true)
	}
	text, known := c.exprText(arg)
	k, found := codes.Find(codes.NamespaceType, elem)
	if !ok || !known || !found {
		return ""
	}
	return codes.Spelling(k) + "(" + text + ")"
}

// numberText s'nin ondalık basamaklarla yazılmış bir sayı olduğudur ("-12"; decimal ise "2.5" de):
// tip çağrısının çevirebildiği en dar yazım.
func numberText(s string, decimal bool) bool {
	s = strings.TrimPrefix(s, "-")
	whole, frac, dot := strings.Cut(s, ".")
	digits := func(p string) bool {
		return p != "" && strings.Trim(p, "0123456789") == ""
	}
	return digits(whole) && (!dot || decimal && digits(frac))
}

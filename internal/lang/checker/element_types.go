package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kabın öğe tipi (3.6, 3.7, 4.10): liste tek tiplidir, sözlüğün değerleri tek tiptedir. Öğe tipi
// bilinen kabın indeksine başka tipte değer yazmak H1136, öğe tipleri ayrı iki listenin '+'sı (ve
// '+='sı) H1138'dir. Çalışma anı öğe tipini taşımadığı için ikisi de bugün çalışınca sessizce geçer;
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
	if c.knownType(elem) && c.knownType(valType) && !functionType(valType) && !c.assignable(elem, valType, value) {
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

package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Demetin uzunluğu ve öğeleri tipindedir (3.6a, 4.4, 15.5): tipi bilinen iki demetin karşılaştırması
// çalışmadan önce denetlenir, çalışma anının koduyla (13.11): '<', '>', '<=', '>='de sıralanamayan öğe
// çifti (iki sayı, iki metin ya da iki demet değil) H2772. Kural çalışma anının tuple_ops.go'suyla aynı
// sırayla yürür: öğeler ortak uzunluğa kadar, iç içe demetlerde içteki öğenin indeksiyle. Tipi bilinmeyen
// ya da isteğe bağlı öğe denetlenmez. '==' ve '!='de tensör tutan öğe eşitlenebilirliğin kuralıdır
// (equality_check.go, H1148).

// checkTupleComparison iki yanı da tipi bilinen demet olan sıralama karşılaştırmasını denetler.
func (c *Checker) checkTupleComparison(e *ast.InfixExpression, left, right string) {
	l, r := shapeOf(left), shapeOf(right)
	if l.elems == nil || r.elems == nil || l.optional || r.optional {
		return
	}
	switch e.Operator {
	case "<", ">", "<=", ">=":
		if i, a, b, ok := unorderedElement(l.elems, r.elems); ok {
			c.addErrorAt(e, messages.New(messages.H2772, string(e.Operator), i, c.typeArg(a), c.typeArg(b)))
		}
	}
}

// unorderedElement 4.4'ün sıralayamadığı ilk öğe çiftidir: indeksi ve iki yanın tipi.
func unorderedElement(x, y []string) (int, string, string, bool) {
	for i := 0; i < min(len(x), len(y)); i++ {
		a, b := shapeOf(x[i]), shapeOf(y[i])
		switch {
		case !knownElement(x[i]) || !knownElement(y[i]):
		case numericType(x[i]) && numericType(y[i]):
		case x[i] == typeString && y[i] == typeString:
		case a.elems != nil && b.elems != nil:
			if j, ea, eb, ok := unorderedElement(a.elems, b.elems); ok {
				return j, ea, eb, true
			}
		default:
			return i, x[i], y[i], true
		}
	}
	return 0, "", "", false
}

// knownElement öğenin tipi karşılaştırmanın sonucunu belirleyecek kadar biliniyor mu: isteğe bağlı
// değil ve çekirdek tip, yokluk ya da demet.
func knownElement(t string) bool {
	s := shapeOf(t)
	if s.optional {
		return false
	}
	return s.elems != nil || t == typeNull || coreTypes[normalizeTypeName(s.base)]
}

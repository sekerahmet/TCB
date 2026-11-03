package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Ayrı tiplerin eşitliği (4.5): == ve != aynı tipten iki değeri karşılaştırır; tipleri bilinen ve ayrı
// iki değerin karşılaştırması ("1" == 1, [1] == [1.0]) sonucu hep aynı olan bir denetim hatasıdır,
// H1141. Bugün çalışma anı onu sessizce TR_false (ya da TR_true) sayar. Kural dışı (kendi kuralları ya
// da kodları olan): iki sayı (1 == 1.0), isteğe bağlı ile asıl tipi ve TR_none (3.5), JSON (H1131,
// H1132), fonksiyon (H2613), tip değeri (H1105'in kalıbı), tensör ve tensör tutan değer (15.5), tipi
// bilinmeyen değer ve öğe tipsiz kap. Kaplar tip argümanında değişmezdir (3.6): TR_List[TR_Int] ile
// TR_List[TR_Float] ayrıdır.

// checkEqualityTypes e'nin == ya da != ise iki yanının tipini denetler.
func (c *Checker) checkEqualityTypes(e *ast.InfixExpression, left, right string) {
	if e.Operator != "==" && e.Operator != "!=" || left == typeNull || right == typeNull {
		return
	}
	l, r := requiredType(left), requiredType(right)
	if !ownEquality(l) || !ownEquality(r) {
		return
	}
	if c.distinctTypes(l, r) {
		c.addErrorAt(e, messages.New(messages.H1141, string(e.Operator), c.typeArg(left), c.typeArg(right)))
	}
}

// ownEquality t'nin eşitliği bu kuralın işi mi: JSON, tensör, fonksiyon ve tip değeri (içlerinde
// geçenler de) kendi kurallarındadır.
func ownEquality(t string) bool {
	for _, other := range []string{typeJSON, "Tensor", "Function", typeType} {
		if strings.Contains(t, other) {
			return false
		}
	}
	return true
}

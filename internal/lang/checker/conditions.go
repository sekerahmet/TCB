package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Koşul TR_Bool olmalıdır (3.10): TR_if ve koşullu TR_for'un koşulu, liste üretecinin ve
// TR_breakpoint TR_if'in koşulu (H2600), TR_and, TR_or ve TR_not'un işlenenleri (H2601), TR_assert'in
// koşulu (H2612). Tipi bilinen başka değer (liste, sayı, metin, demet, yapı ...) denetim hatasıdır;
// kod çalışma anındakidir (13.11'in istisnası, internal/lang/messages/README.md). Tipi bilinmeyen,
// isteğe bağlı (TR_Bool?, daraltma 4b'nin) ve denetleyicinin tanımadığı tip susar.

// condition koşulun tipini çıkarır ve TR_Bool olmasını denetler (H2600).
func (c *Checker) condition(e ast.Expression) {
	if e != nil {
		c.requireBool(e, c.inferExpression(e), messages.H2600)
	}
}

// requireBool tipi t olan e TR_Bool değilse code ile çalışma anındaki hatayı verir; tip
// bilinmiyorsa susar.
func (c *Checker) requireBool(e ast.Expression, t string, code messages.Code) {
	if t == typeBool || t == "" || t == typeAny || shapeOf(t).optional || !c.knownType(t) && t != typeNull {
		return
	}
	c.addErrorAt(e, messages.New(code, c.typeArg(t)))
}

// prefixType önek işlecin tipidir: TR_not'un işleneni TR_Bool olmalıdır (H2601) ve sonucu TR_Bool;
// '~' tam sayıda tam sayı; '-' ve '+' işlenenin işaretli tipi (signType).
func (c *Checker) prefixType(e *ast.PrefixExpression) string {
	right := c.inferExpression(e.Right)
	switch e.Operator {
	case "!":
		c.requireBool(e.Right, right, messages.H2601)
		return typeBool
	case "~":
		if right == typeInt {
			return typeInt
		}
		return typeAny
	}
	return signType(right)
}

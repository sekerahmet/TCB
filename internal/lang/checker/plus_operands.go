package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// '+'nın işlenenleri (4.10, 5.4): '+' iki sayıyı toplar, iki metni ya da iki listeyi uç uca ekler;
// başka tiple metin ya da liste denetim hatasıdır, çalışma anındaki kodla (13.11'in emsali): metinde
// H2609 (ileti yerleştirmeyi önerir), metinsiz listede H2614 (ileti l + [x] ve .TR_append'i önerir).
// x += y, x = x + y'dir; aynı kural uygulanır. Tipi bilinmeyen işlenen ve tensör denetlenmez.

// plusOperands '+'nın iki işlenenine uyan çalışma anı hatasının kodudur; geçerli ya da bilinmeyen
// işlenenlerde 0.
func plusOperands(left, right string) messages.Code {
	if !plusKnown(left) || !plusKnown(right) {
		return 0
	}
	lStr, rStr := left == typeString, right == typeString
	lList, rList := baseType(left) == typeList && !shapeOf(left).optional, baseType(right) == typeList && !shapeOf(right).optional
	switch {
	case lStr && rStr, lList && rList:
		return 0
	case (lList || rList) && !lStr && !rStr:
		return messages.H2614
	case lStr || rStr:
		return messages.H2609
	}
	return 0
}

// plusKnown '+'nın kuralının karar verebildiği bir işlenen tipi mi: sayı, mantık değeri, yokluk, metin,
// liste ya da sözlük (tensör ve bayt dizisi kendi kurallarındadır; isteğe bağlı tip 3.5'in işidir).
func plusKnown(t string) bool {
	switch shapeOf(t).base {
	case typeInt, typeFloat, typeBool, typeString, typeList, typeDict:
		return !shapeOf(t).optional
	}
	return t == typeNull
}

// checkPlus n'deki '+'nın (ya da '+='nın) işlenenlerini denetler; hata n'nin yerindedir.
func (c *Checker) checkPlus(n ast.Node, left, right string) {
	if code := plusOperands(left, right); code != 0 {
		c.addErrorAt(n, messages.New(code, c.typeArg(left), c.typeArg(right)))
	}
}

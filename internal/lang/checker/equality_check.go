package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Eşitlenemeyen değerin karşılaştırması denetim hatasıdır (4.5, 5.6, 7.6–7.8, 13.3, 13.10, 15.5); kural
// equatable.go'nun tek tanımıdır. Üç yer: '==' ve '!=' (tensör H1148, isteğe bağlı tensör H1151, fonksiyon
// H1152, arayüz H1153), '==' isteyen değer deseni (H1150), .TR_contains, .TR_index_of ve .TR_unique (H1149). İstisnalar: isteğe bağlı değerin TR_none değişmeziyle
// karşılaştırması (3.5) ve iki tensörün öğe öğe '=='si (15.5). TR_assert_equal burada değildir: içteki
// tensörü TR_equal'ın anlamıyla karşılaştırır (12.4, çalışma anı).

// checkEquatable '==' ya da '!='nin iki yanını denetler; hata verdiyse doğrudur.
func (c *Checker) checkEquatable(e *ast.InfixExpression, left, right string) bool {
	if e.Operator != "==" && e.Operator != "!=" || noneLiteral(e.Left) || noneLiteral(e.Right) {
		return false
	}
	for _, t := range [2]string{left, right} {
		what := c.unequatable(t)
		if what == "" || elementwise(t) {
			continue
		}
		op, typ := string(e.Operator), c.typeArg(t)
		switch {
		case what == holdsFunction:
			c.addErrorAt(e, messages.New(messages.H1152, op, typ))
		case what == holdsInterface:
			c.addErrorAt(e, messages.New(messages.H1153, op, typ))
		case shapeOf(t).optional:
			c.addErrorAt(e, messages.New(messages.H1151, op, typ))
		default:
			how, suggestion := c.equalityAdvice(t, e.Left, e.Right)
			c.addErrorAt(e, messages.New(messages.H1148, op, typ, how, suggestion))
		}
		return true
	}
	return false
}

// checkEquatablePattern eşlemenin '==' isteyen desenini denetler (5.6): konu ya da desen eşitlenemiyorsa
// hatadır, tensör de (desen tensörü bütün olarak karşılaştırır). Durum deseni karşılaştırmaz (7.7): durum
// açan desen (ast.EnumPattern, Cevap.Tamam(x) =>) ve parantezsiz alansız durum (C.Yok =>; TCB'nin kararı,
// 2026-11-09). Parantezli (C.Yok) => değer desenidir (5.6). bare desenin parantezsiz yazıldığıdır.
func (c *Checker) checkEquatablePattern(e *ast.MatchExpression, subject string, pattern ast.Expression, patType string, bare bool) {
	if _, opens := pattern.(*ast.EnumPattern); opens || noneLiteral(pattern) || bare && c.bareState(pattern) {
		return
	}
	for _, t := range [2]string{subject, patType} {
		what := c.unequatable(t)
		if what == "" {
			continue
		}
		c.addErrorAt(pattern, messages.New(messages.H1150, c.typeArg(t), what))
		return
	}
}

// checkEquatableSearch listenin '==' ile arayan metodunu denetler (13.3, 13.10): .TR_contains(x),
// .TR_index_of(x) ve .TR_unique() eşitlenebilir öğe ister; öğe tipi bilinmeyen listede aranan değer.
func (c *Checker) checkEquatableSearch(e *ast.CallExpression, objType string, argTypes []string) {
	m, ok := e.Function.(*ast.MemberExpression)
	if !ok || m.Safe {
		return
	}
	method := ""
	switch m.NameToken.Code {
	case codes.MemContains:
		method = "contains"
	case codes.MemIndexOf:
		method = "index"
	case codes.MemUnique:
		method = "unique"
	default:
		return
	}
	recv := shapeOf(objType)
	if recv.optional || normalizeTypeName(recv.base) != typeList {
		return
	}
	part, types := "elements", recv.args
	if method != "unique" && len(argTypes) == 1 && c.firstUnequatable(types, map[string]bool{}) == "" {
		part, types = "value", argTypes[:1]
	}
	for _, t := range types {
		if what := c.unequatable(t); what != "" {
			c.addErrorAt(e, messages.New(messages.H1149, c.typeArg(objType), part, what))
			return
		}
	}
}

// bareState desen programın bir durumlu tipinin alansız durumu mu (C.Yok)?
func (c *Checker) bareState(pattern ast.Expression) bool {
	m, ok := pattern.(*ast.MemberExpression)
	if !ok || m.Safe {
		return false
	}
	id, ok := m.Object.(*ast.Identifier)
	if !ok {
		return false
	}
	def := c.typeDefs[id.Value]
	if def == nil || def.Kind != ast.KindEnum {
		return false
	}
	for _, v := range def.Variants {
		if v.Name == m.Property {
			return len(v.Fields) == 0
		}
	}
	return false
}

// bareAt kolun i. deseninin parantezsiz yazıldığıdır (ast.MatchCase.Bare); bilinmiyorsa yanlış.
func bareAt(bare []bool, i int) bool {
	return i < len(bare) && bare[i]
}

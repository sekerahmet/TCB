package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Döngü (5.7): adların tipi kaynaktan gelir (loopTypes). Öğenin yerindeki demet deseni
// (TR_for (a, b) TR_in l, TR_for i, (a, b) TR_in l) yalnız demeti açar: öğenin tipi biliniyor ve
// demet değilse (liste ve isteğe bağlı demet dahil) H2731; demetin öğe sayısı desenin ad sayısını
// tutmazsa H1026, örnek düzeltilmiş döngü başlığıdır. Tipi bilinmeyen öğede ikisi de çalışırken
// aynı kodla denetlenir (interp, compiler_loop_bind.go; H2731, H2709).

// checkForIn döngüyü denetler: adları kendi kapsamında öğenin tipleriyle tanımlar, gövdeyi denetler.
func (c *Checker) checkForIn(s *ast.ForInStatement) {
	key, value := loopTypes(c.inferExpression(s.Iterable), s.KeyVar != "")
	prev := c.scope
	c.scope = NewScope(prev)
	types := c.loopNameTypes(s, key, value)
	for i, n := range s.Names() {
		c.defineName(n.Name, kindVariable, siteOf(n.Token, s.Line()), TypeInfo{TypeName: types[i]})
	}
	c.checkStatements(s.Body.Statements)
	c.scope = prev
}

// loopNameTypes Names()'in sırasıyla adların tipleridir: indeks ya da anahtar, sonra öğe ya da
// desenin adları.
func (c *Checker) loopNameTypes(s *ast.ForInStatement, key, value string) []string {
	var out []string
	if s.KeyVar != "" {
		out = append(out, key)
	}
	if s.Pattern == nil {
		if s.ValueVar != "" {
			out = append(out, value)
		}
		return out
	}
	return append(out, c.patternTypes(s, value)...)
}

// patternTypes desenin adlarının tipleridir (demetin öğe tipleri, bilinmeyen TR_Any) ve desenin
// hatalarını yazar.
func (c *Checker) patternTypes(s *ast.ForInStatement, value string) []string {
	types := make([]string, len(s.Pattern))
	for i := range types {
		types[i] = typeAny
	}
	sh := shapeOf(value)
	switch {
	case sh.elems != nil && !sh.optional:
		if len(sh.elems) != len(s.Pattern) {
			kind, example := ast.UnpackSuggestion(s, len(sh.elems))
			c.addErrorAt(s, messages.New(messages.H1026, len(sh.elems), len(s.Pattern), kind, example))
		}
		for i := range types {
			if i < len(sh.elems) {
				types[i] = sh.elems[i]
			}
		}
	case c.knownType(value):
		kind := "other"
		if sh.base == typeList && !sh.optional {
			kind = "list"
		}
		c.addErrorAt(s, messages.New(messages.H2731, s.PatternSpelling(), c.typeArg(value), kind, s.PatternHead(), s.IterableToken.Literal))
	}
	return types
}

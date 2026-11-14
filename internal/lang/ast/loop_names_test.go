package ast_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// forIn kodlu gövdenin ilk deyimini döngü olarak verir.
func forIn(t *testing.T, src string) *ast.ForInStatement {
	t.Helper()
	p := parser.New(lexer.NewBody(src, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("%q: %v", src, p.Errors())
	}
	s, ok := prog.Statements[0].(*ast.ForInStatement)
	if !ok {
		t.Fatalf("%q: döngü değil: %T", src, prog.Statements[0])
	}
	return s
}

// Döngünün adları (5.7) Names'ten yazılış sırasıyla gelir: indeks ya da anahtar, sonra öğe ya da
// desenin adları; her ad kendi sözcük birimini taşır. İletilerin parçaları desenden kurulur: yazılan
// desen, deseni bırakan tek adlı baş (ilk "_" olmayan ad, hepsi "_" ise x) ve sayısı düzeltilmiş
// döngü başlığı (fazla ad silinir, eksik öğeye _ yazılır).
func TestLoopNamesAndPatternTexts(t *testing.T) {
	cases := []struct{ src, names, spelling, head, two, three string }{
		{"TR_for x TR_in l { }", "x", "()", "x", "", ""},
		{"TR_for i, x TR_in l { }", "i x", "()", "i, x", "", ""},
		{"TR_for (a, b) TR_in f(l, 2) { }", "a b", "(a, b)", "a", "TR_for (a, b) TR_in f(l, 2)", "TR_for (a, b, _) TR_in f(l, 2)"},
		{"TR_for i, (_, b, c) TR_in l { }", "i _ b c", "(_, b, c)", "i, b", "TR_for i, (_, b) TR_in l", "TR_for i, (_, b, c) TR_in l"},
		{"TR_for (_, _) TR_in l { }", "_ _", "(_, _)", "x", "TR_for (_, _) TR_in l", "TR_for (_, _, _) TR_in l"},
	}
	for _, c := range cases {
		s := forIn(t, c.src)
		var names []string
		for _, n := range s.Names() {
			if n.Token.Literal != n.Name || n.Token.Line != 1 {
				t.Errorf("%q: %q adının sözcük birimi yanlış: %+v", c.src, n.Name, n.Token)
			}
			names = append(names, n.Name)
		}
		if got := strings.Join(names, " "); got != c.names {
			t.Errorf("%q: adlar %q, beklenen %q", c.src, got, c.names)
		}
		if s.Pattern == nil {
			continue
		}
		if got := s.PatternSpelling(); got != c.spelling {
			t.Errorf("%q: desen %q, beklenen %q", c.src, got, c.spelling)
		}
		if got := s.PatternHead(); got != c.head {
			t.Errorf("%q: baş %q, beklenen %q", c.src, got, c.head)
		}
		for count, want := range map[int]string{2: c.two, 3: c.three} {
			if _, got := ast.UnpackSuggestion(s, count); got != want {
				t.Errorf("%q, %d öğe: örnek %q, beklenen %q", c.src, count, got, want)
			}
		}
	}
}

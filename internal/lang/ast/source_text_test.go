package ast_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// ast.SourceText ifadenin dosyadaki yazımını, ağaçta tutulmayan kapanan ayraçlarla verir.
func TestSourceText(t *testing.T) {
	src := "TR_print(f(a[1], (b + 2)))"
	p := parser.New(lexer.NewBody(src, true))
	prog := p.ParseProgram()
	call := prog.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	inner := call.Arguments[0].(*ast.CallExpression)
	lines := []string{src}
	for _, c := range []struct {
		n    ast.Expression
		want string
	}{{call, src}, {inner, "f(a[1], (b + 2))"}, {inner.Arguments[0], "a[1]"}, {inner.Arguments[1], "b + 2"}} {
		if got, ok := ast.SourceText(lines, c.n); !ok || got != c.want {
			t.Errorf("%q %v, beklenen %q", got, ok, c.want)
		}
	}
	if _, ok := ast.SourceText(nil, call); ok {
		t.Error("satırı bilinmeyen ifadenin yazımı olmaz")
	}
}

// Kural (15.6d, K4): iletinin önerisindeki deyim kopya adımının yerine adı yazar; ayraçlı adım
// ayracıyla gider; adlar dosyadaki ve kapsamdaki adlarla çakışmaz; uzun indeks ayraç alır.
func TestNewCopyWriteText(t *testing.T) {
	cases := []struct {
		src, write, copy, index string
	}{
		{"t[idx][0] += 5.0", "c[0] += 5.0", "c", "idx"},
		{"(t * 2.0)[0] = 9.0", "c[0] = 9.0", "c", ""},
		{"t[a + b][1][2] = c", "c2[1][2] = c", "c2", "(a + b)"},
		{"m[idx].TR_slice(1, 0, 1)[0] = 1.0", "c.TR_slice(1, 0, 1)[0] = 1.0", "c", "idx"},
	}
	for _, c := range cases {
		p := parser.New(lexer.NewBody(c.src, true))
		s := p.ParseProgram().Statements[0].(*ast.AssignStatement)
		chain := ast.WriteChain(s.Target.(*ast.IndexExpression).Left)
		step := chain[len(chain)-1] // işlem
		if !step.Copies() {
			step = chain[len(chain)-2] // kökün bir üstü: tensörle indeks
		}
		got, ok := ast.NewCopyWriteText([]string{c.src}, s.Target, s.Value, step, func(string) bool { return false })
		if !ok || got.Write != c.write || got.Copy != c.copy || got.Index != c.index {
			t.Errorf("%s: %+v %v; beklenen yazım %q, ad %q, indeks %q", c.src, got, ok, c.write, c.copy, c.index)
		}
	}
	src := "TR_write_slice(m[idx], k, TR_dim: 1, TR_start: 0)"
	call := parser.New(lexer.NewBody(src, true)).ParseProgram().Statements[0].(*ast.ExpressionStatement).Expression
	step := ast.WriteChain(call.(*ast.CallExpression).Arguments[0])[0]
	got, ok := ast.NewCopyWriteText([]string{src}, call, call, step, func(n string) bool { return n == "c" })
	if want := "TR_write_slice(c2, k, TR_dim: 1, TR_start: 0)"; !ok || got.Write != want || !strings.HasPrefix(got.K, "k") || got.K == "k" {
		t.Errorf("TR_write_slice: %+v %v, beklenen %q ve k'den başka sayaç", got, ok, want)
	}
}

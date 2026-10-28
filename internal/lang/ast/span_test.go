package ast_test

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// ast.RangeOf runs from the first token of a node to the end of its last one.
func TestRangeOfANode(t *testing.T) {
	p := parser.New(lexer.NewBody("TR_var x = foo + 12 * y\nTR_print(x)", true))
	prog := p.ParseProgram()
	if len(p.Errs()) != 0 {
		t.Fatalf("errors %v", p.Errs())
	}
	cases := []struct {
		name string
		node any
		want messages.Range
	}{
		{"a declaration", prog.Statements[0], messages.Range{Line: 1, Col: 1, EndLine: 1, EndCol: 24}},
		{"its value", prog.Statements[0].(*ast.VarStatement).Value, messages.Range{Line: 1, Col: 12, EndLine: 1, EndCol: 24}},
		{"a name", prog.Statements[0].(*ast.VarStatement).Name, messages.Range{Line: 1, Col: 8, EndLine: 1, EndCol: 9}},
	}
	for _, c := range cases {
		if got := ast.RangeOf(c.node); got != c.want {
			t.Errorf("%s: range %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := ast.RangeOf(&ast.Program{}); got != (messages.Range{}) {
		t.Errorf("an empty program has no place, got %+v", got)
	}
}

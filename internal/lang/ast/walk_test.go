package ast

import "testing"

// Kural: Walk ağacın her ast düğümüne bir kez ve kaynak sırasıyla fn çağırır; false dönen düğümün
// çocuklarına inilmez; Token alanları gezilmez.
// Neden: codeindex ve biçimleyici aynı yansımalı gezgini ayrı ayrı yazıyordu.
func TestWalkVisitsNodesInOrderAndHonoursFalse(t *testing.T) {
	inner := &InfixExpression{Left: &Identifier{Value: "a"}, Operator: "+", Right: &Identifier{Value: "b"}}
	prog := &Program{Statements: []Statement{
		&ExpressionStatement{Expression: inner},
		&ExpressionStatement{Expression: &Identifier{Value: "c"}},
	}}
	var seen []string
	Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *Identifier:
			seen = append(seen, n.Value)
		case *InfixExpression:
			seen = append(seen, "infix")
		}
		return true
	})
	if got := join(seen); got != "infix a b c" {
		t.Errorf("sıra = %q", got)
	}
	seen = nil
	Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *Identifier:
			seen = append(seen, n.Value)
		case *InfixExpression:
			return false // çocuklara inme
		}
		return true
	})
	if got := join(seen); got != "c" {
		t.Errorf("false sonrası = %q", got)
	}
}

func join(list []string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

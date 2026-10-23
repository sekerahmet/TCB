package ast

import (
	"slices"
	"testing"
)

func named(name string) *FunctionLiteral { return &FunctionLiteral{Name: name} }

func exprNames(e Expression) []string {
	var got []string
	ExprDefinedNames(e, func(n string) { got = append(got, n) })
	return got
}

// Kural (5.12): ExprDefinedNames adlı fonksiyonları bulunduğu kapsam için toplar; koşul, eşle konusu
// ve desenleri bulunulan kapsamdadır, eşle kolunun tek satırlık ifadesi ve bloklar kolun/bloğun
// kapsamıdır, fonksiyon gövdesine inilmez.
func TestExprDefinedNamesFollowsScopeRule(t *testing.T) {
	ifExpr := &IfExpression{
		Condition: named("koşul"),
		ElseIfs:   []ElseIfBranch{{Condition: named("değilse_koşul")}},
	}
	matchExpr := &MatchExpression{
		Subject: named("konu"),
		Cases: []MatchCase{
			{Patterns: []Expression{named("desen")}, ValueExpr: named("kol")},
			{IsDefault: true, ValueExpr: named("öbür_kol")},
		},
	}
	for _, d := range []struct {
		ad   string
		e    Expression
		want []string
	}{
		{"nil", nil, nil},
		{"adsız fonksiyon", named(""), nil},
		{"adlı fonksiyon", named("f"), []string{"f"}},
		{"çağrı", &CallExpression{Function: &Identifier{Value: "g"}, Arguments: []Expression{named("a")}, NamedArgs: []NamedArg{{Value: named("b")}}}, []string{"a", "b"}},
		{"eğer koşulları", ifExpr, []string{"koşul", "değilse_koşul"}},
		{"eşle: konu ve desen, kol değil", matchExpr, []string{"konu", "desen"}},
		{"liste üreteci: yalnızca kaynak", &ListComprehension{Iterable: named("kaynak"), Output: named("çıktı"), Condition: named("süz")}, []string{"kaynak"}},
	} {
		if got := exprNames(d.e); !slices.Equal(got, d.want) {
			t.Errorf("%s: %v, beklenen %v", d.ad, got, d.want)
		}
	}
}

// Kural: StatementDefinedNames deyimin çalıştığı kapsama tanımladığı adları verir; eşzamanlı çağrı
// görevin kapsamında kalır, takma adsız aktar "" bildirir.
func TestStatementDefinedNames(t *testing.T) {
	var stmts = []Statement{
		&VarStatement{Name: &Identifier{Value: "v"}, Value: named("içte")},
		&MultiVarStatement{Names: []*Identifier{{Value: "a"}, {Value: "b"}}, Values: []Expression{named("c")}},
		&StructStatement{Name: "Y"},
		&ImportStatement{Alias: ""},
		&ExpressionStatement{Expression: named("e")},
		&AsyncStatement{Call: named("görevde")},
		&DeferStatement{Call: named("ertelenen")},
	}
	var got []string
	for _, st := range stmts {
		StatementDefinedNames(st, func(n string) { got = append(got, n) })
	}
	want := []string{"v", "içte", "a", "b", "c", "Y", "", "e", "ertelenen"}
	if !slices.Equal(got, want) {
		t.Errorf("%v, beklenen %v", got, want)
	}
}

// Kural: SubExpressions doğrudan alt ifadeleri kaynak sırasıyla verir ve true dönen ziyarette durur.
func TestSubExpressionsOrderAndStop(t *testing.T) {
	infix := &InfixExpression{Left: &Identifier{Value: "a"}, Right: &Identifier{Value: "b"}}
	var seen []string
	SubExpressions(infix, func(c Expression) bool { seen = append(seen, c.(*Identifier).Value); return false })
	if !slices.Equal(seen, []string{"a", "b"}) {
		t.Errorf("sıra %v", seen)
	}
	seen = nil
	stopped := SubExpressions(infix, func(c Expression) bool { seen = append(seen, c.(*Identifier).Value); return true })
	if !stopped || !slices.Equal(seen, []string{"a"}) {
		t.Errorf("durma: %v %v", stopped, seen)
	}
}

package ast

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// at verilen konumda bir sözcük birimi kurar (döküm bu konumu görmemeli).
func at(line, col int, lit string) lexer.Token {
	return lexer.Token{Type: lexer.TOKEN_IDENT, Literal: lit, Line: line, Column: col}
}

// sample `f(a + 1)` ağacını verilen konum kaydırmasıyla kurar; işleç ve sayı parametredir.
func sample(shift int, op string, n int64) *Program {
	infix := &InfixExpression{
		Token:    at(1+shift, 5+shift, op),
		Left:     &Identifier{Token: at(1+shift, 3+shift, "a"), Value: "a"},
		Operator: op,
		Right:    &IntegerLiteral{Token: at(1+shift, 7+shift, "1"), Value: n},
	}
	call := &CallExpression{
		Token:     at(1+shift, 2+shift, "("),
		Function:  &Identifier{Token: at(1+shift, 1+shift, "f"), Value: "f"},
		Arguments: []Expression{infix},
	}
	return &Program{Statements: []Statement{&ExpressionStatement{Token: at(1+shift, 1+shift, "f"), Expression: call}}}
}

// Anlam konumdan bağımsız karşılaştırılır: aynı programın konumları farklı iki ağacının dökümü
// aynıdır; anlamı değişen (işleç, sayı) ağaçların dökümü farklıdır.
func TestDumpIgnoresPositions(t *testing.T) {
	if a, b := Dump(sample(0, "+", 1)), Dump(sample(40, "+", 1)); a != b {
		t.Errorf("konum dökümü değiştirdi:\n%s\n---\n%s", a, b)
	}
	base := Dump(sample(0, "+", 1))
	if base == Dump(sample(0, "-", 1)) {
		t.Error("işleç değişti ama döküm aynı")
	}
	if base == Dump(sample(0, "+", 2)) {
		t.Error("sayı değişti ama döküm aynı")
	}
	if !strings.Contains(base, "Operator") || strings.Contains(base, "Line") || strings.Contains(base, "Column") {
		t.Errorf("döküm alan adlarını yazar, konumu yazmaz:\n%s", base)
	}
	if got := sample(3, "+", 1).String(); got != base {
		t.Error("Program.String dökümle aynı olmalı")
	}
}

// Dışa açık her alan dökümde görünür (Token dışında); boş değer ayırt edilir. Yeni alan eklenince
// dökümü değiştirmek gerekmez ama bu test o alanın görünmesini bekler: Dump'ın atladığı tek şey
// lexer.Token alanlarıdır.
func TestDumpWritesEveryNonTokenField(t *testing.T) {
	d := Dump(&CallExpression{Token: at(1, 1, "("), Function: &Identifier{Value: "f"}})
	for _, field := range []string{"Function", "Arguments", "NamedArgs"} {
		if !strings.Contains(d, field+":") {
			t.Errorf("%s alanı dökümde yok:\n%s", field, d)
		}
	}
	if !strings.Contains(d, "∅") && !strings.Contains(d, "[\n") {
		t.Errorf("boş değerler belirtilmeli:\n%s", d)
	}
}

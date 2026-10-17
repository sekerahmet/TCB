package parser

import (
	"fmt"
	"strings"
	"testing"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
)

// shape ifade ağacını parantezli biçimde yazar (yalnızca testlerde kullanılan düğümler).
func shape(e ast.Expression) string {
	switch n := e.(type) {
	case *ast.InfixExpression:
		return fmt.Sprintf("(%s %s %s)", shape(n.Left), n.Operator, shape(n.Right))
	case *ast.PrefixExpression:
		return fmt.Sprintf("(%s%s)", n.Operator, shape(n.Right))
	case *ast.Identifier:
		return n.Value
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", n.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", n.Value)
	}
	return fmt.Sprintf("<%T>", e)
}

func firstExpr(t *testing.T, code string) ast.Expression {
	t.Helper()
	p := New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("%s: beklenmeyen hata: %v", code, p.Errors())
	}
	es, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("%s: ifade deyimi bekleniyordu, %T alındı", code, prog.Statements[0])
	}
	return es.Expression
}

// Hatalı yapı metodları eskiden "interface conversion: ast.Expression is nil" paniğine yol açıyordu.
func TestMalformedStructMethodReportsErrorInsteadOfPanic(t *testing.T) {
	for _, code := range []string{
		"yapı A { fonksiyon f { } }",
		"yapı A { fonksiyon f { yazdır(1) } }\nyazdır(2)",
		"yapı A { fonksiyon { } }",
		"yapı A { fonksiyon f(",
		"yapı A { fonksiyon f { { { }",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%q ayrıştırılırken panik: %v", code, r)
				}
			}()
			p := New(lexer.New(code))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Errorf("%q için sözdizimi hatası bekleniyordu", code)
			}
		}()
	}
}

func TestValidStructMethodStillParses(t *testing.T) {
	p := New(lexer.New("yapı A {\n  ad: Metin\n  fonksiyon selam() { sonuç kendi.ad }\n}"))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("geçerli yapı hata vermemeli: %v", p.Errors())
	}
}

func parseErrors(code string) []string {
	p := New(lexer.New(code))
	p.ParseProgram()
	return p.Errors()
}

// Metin içindeki tek bir '{' eskiden dosyanın geri kalanını yutuyordu.
func TestUnbalancedInterpolationIsReportedWithLine(t *testing.T) {
	errs := parseErrors("yazdır(\"a\")\nyazdır(\"{\")\nyazdır(\"b\")")
	if len(errs) == 0 || !strings.Contains(errs[0], "Satır 2") || !strings.Contains(errs[0], "'{' kapatılmamış") {
		t.Fatalf("2. satırda kapatılmamış '{' hatası bekleniyordu, alınan: %v", errs)
	}
}

func TestUnterminatedStringIsReported(t *testing.T) {
	errs := parseErrors("değişken a = 1\nyazdır(\"merhaba)\n")
	if len(errs) == 0 || !strings.Contains(errs[0], "Satır 2") || !strings.Contains(errs[0], "kapatılmamış metin") {
		t.Fatalf("2. satırda kapatılmamış metin hatası bekleniyordu, alınan: %v", errs)
	}
}

func TestEscapedBracesParseAsLiteral(t *testing.T) {
	if got := shape(firstExpr(t, `"\{ x \}"`)); got != `"{ x }"` {
		t.Fatalf("düz süslü parantez bekleniyordu, %s alındı", got)
	}
}

func TestPowerAndNotPrecedenceShape(t *testing.T) {
	cases := map[string]string{
		"2 ** 3 ** 2":  "(2 ** (3 ** 2))",
		"-2 ** 2":      "(-(2 ** 2))",
		"değil 1 == 2": "(!(1 == 2))",
		"değil a ve b": "((!a) && b)",
		"-a * b":       "((-a) * b)",
	}
	for code, want := range cases {
		if got := shape(firstExpr(t, code)); got != want {
			t.Errorf("%s: %s bekleniyordu, %s alındı", code, want, got)
		}
	}
}

func TestCallOnNextLineStartsNewStatement(t *testing.T) {
	p := New(lexer.New("değişken f = 5\n(1 + 2) |> yazdır\nx\n[1]"))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("beklenmeyen hata: %v", p.Errors())
	}
	if len(prog.Statements) != 4 {
		t.Fatalf("4 deyim bekleniyordu, %d alındı", len(prog.Statements))
	}
}

package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
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

// Hatalı yapı metodları eskiden "interface conversion: agac.Expression is nil" paniğine yol açıyordu.
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

// 'test' yalnızca deyim başında ve ardından metin geldiğinde anahtar kelimedir.
func TestTestIsContextualKeyword(t *testing.T) {
	p := New(lexer.New("test \"toplama\" { doğrula(1 + 1 == 2) }"))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("beklenmeyen hata: %v", p.Errors())
	}
	ts, ok := prog.Statements[0].(*ast.TestStatement)
	if !ok || ts.Name != "toplama" || len(ts.Body.Statements) != 1 {
		t.Fatalf("test deyimi bekleniyordu, %T alındı", prog.Statements[0])
	}

	// Ad olarak kullanım bozulmamalı
	for _, code := range []string{"değişken test = 5\ntest = test + 1\nyazdır(test)", "test := [1]\ntest[0]", "fonksiyon test(a) { sonuç a }\ntest(\"x\")"} {
		p := New(lexer.New(code))
		prog := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%q: beklenmeyen hata: %v", code, p.Errors())
		}
		for _, st := range prog.Statements {
			if _, ok := st.(*ast.TestStatement); ok {
				t.Fatalf("%q: 'test' adı test deyimi olarak okundu", code)
			}
		}
	}

	// Gövdesiz test bir sözdizimi hatasıdır
	p = New(lexer.New("test \"ad\" doğrula(doğru)"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("gövdesiz test hata vermedi")
	}
}

// Metin içi yerleştirmedeki ifadeler dosyadaki gerçek konumlarını taşımalı (eskiden Satır 1, Sütun 1).
func TestInterpolationKeepsSourcePosition(t *testing.T) {
	p := New(lexer.New("değişken a = 1\nyazdır(\"x = {toplm + 1}\")"))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("beklenmeyen hata: %v", p.Errors())
	}
	call := prog.Statements[1].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	interp := call.Arguments[0].(*ast.InterpolatedString)
	infix := interp.Parts[1].(*ast.InfixExpression)
	id := infix.Left.(*ast.Identifier)
	if id.Token.Line != 2 || id.Token.Column != 14 {
		t.Fatalf("konum Satır 2, Sütun 14 olmalıydı, alınan: Satır %d, Sütun %d", id.Token.Line, id.Token.Column)
	}
}

// Test raporu Y1: kapatılmamış '{' hatasız kabul ediliyor, sonraki kod sessizce bloğa giriyordu.
func TestUnclosedBlockBraceIsReported(t *testing.T) {
	for code, want := range map[string]string{
		"eğer yanlış {\n    yazdır(\"gizli\")\nyazdır(\"son\")":    "Satır 1, Sütun 13: '{' kapatılmadı",
		"fonksiyon f() {\n    sonuç 1\nyazdır(f())":                "Satır 1, Sütun 15: '{' kapatılmadı",
		"yapı Kişi {\n    ad: Metin\n":                             "Satır 1, Sütun 11: '{' kapatılmadı",
		"değişken x = eşle 1 {\n    1 => \"bir\"\n":                "'{' kapatılmadı",
		"döngü i içinde 1..3 {\n    eğer i == 2 {\n        çık\n}": "Satır 1, Sütun 21: '{' kapatılmadı",
	} {
		errs := parseErrors(code)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), want) {
			t.Errorf("%q için %q bekleniyordu, alınan: %v", code, want, errs)
		}
	}
	if errs := parseErrors("eğer doğru {\n    yazdır(1)\n}\n"); len(errs) != 0 {
		t.Fatalf("kapatılmış blok hata vermemeli: %v", errs)
	}
}

// Kaçışsız her '{' bir yerleştirme başlatır (2.7): içi ifade olarak ayrıştırılamayan süslü
// parantezler (JSON, başka dillerde kod) sessizce düz metin kalmaz, '{'nin konumunda
// sözdizimi hatası verir. Düz süslü parantezin iki yazımı (\{ ve ham metin) hata vermez.
func TestNonExpressionInterpolationIsError(t *testing.T) {
	for code, colNo := range map[string]string{
		`yazdır("{\"kod\": 5}")`:      "Sütun 9:",
		`yazdır("Nokta{x: 1, y: 2}")`: "Sütun 14:",
		`yazdır("şablon: {'a': 1}")`:  "Sütun 17:",
		`yazdır("{değil}")`:           "Sütun 9:",
		`yazdır("boş {}")`:            "Sütun 13:",
	} {
		errs := strings.Join(parseErrors(code), "\n")
		if !strings.Contains(errs, colNo+" Sözdizimi hatası — metindeki her '{' bir yerleştirme başlatır") ||
			!strings.Contains(errs, "ham metin") {
			t.Errorf("%s: %s konumunda yerleştirme hatası bekleniyordu: %s", code, colNo, errs)
		}
	}
	for _, code := range []string{"yazdır(\"\\{\\\"kod\\\": 5\\}\")", "yazdır(`{\"kod\": 5}`)", `yazdır("a {1 + 2} b")`} {
		if errs := parseErrors(code); len(errs) != 0 {
			t.Errorf("%s hata vermemeli: %v", code, errs)
		}
	}
}

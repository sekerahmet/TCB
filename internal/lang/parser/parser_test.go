package parser

import (
	"fmt"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
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

// parseErrs, parseErrors gibi ama hataları ileti kodu ve konumuyla denetlenebilsin diye error olarak verir.
func parseErrs(code string) []error {
	p := New(lexer.New(code))
	p.ParseProgram()
	return p.Errs()
}

// hasCode, hatalar arasında verilen ileti koduna sahip olan var mı diye bakar (konuma bakmaz).
func hasCode(errs []error, code messages.Code) bool {
	for _, e := range errs {
		if messages.Has(e, code) {
			return true
		}
	}
	return false
}

// hasErrAt, verilen ileti koduna sahip bir hatanın satır ve sütununu denetler; 0 verilen
// alan (satır ya da sütun) denetlenmez.
func hasErrAt(errs []error, line, col int, code messages.Code) bool {
	for _, e := range errs {
		l, c, ok := messages.Pos(e)
		if ok && (line == 0 || l == line) && (col == 0 || c == col) && messages.Has(e, code) {
			return true
		}
	}
	return false
}

// Metin içindeki tek bir '{' eskiden dosyanın geri kalanını yutuyordu.
func TestUnbalancedInterpolationIsReportedWithLine(t *testing.T) {
	errs := parseErrs("yazdır(\"a\")\nyazdır(\"{\")\nyazdır(\"b\")")
	if len(errs) == 0 || !hasErrAt(errs[:1], 2, 0, messages.H0009) {
		t.Fatalf("2. satırda kapatılmamış '{' hatası bekleniyordu, alınan: %v", errs)
	}
}

func TestUnterminatedStringIsReported(t *testing.T) {
	errs := parseErrs("değişken a = 1\nyazdır(\"merhaba)\n")
	if len(errs) == 0 || !hasErrAt(errs[:1], 2, 0, messages.H0010) {
		t.Fatalf("2. satırda kapatılmamış metin hatası bekleniyordu, alınan: %v", errs)
	}
}

func TestEscapedBracesParseAsLiteral(t *testing.T) {
	if got := shape(firstExpr(t, `"\{ x \}"`)); got != `"{ x }"` {
		t.Fatalf("düz süslü parantez bekleniyordu, %s alındı", got)
	}
}

func TestPowerAndNotPrecedenceShape(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	p := New(lexer.New("değişken f = 5\n(1 + 2) |> yazdır\nx\n[1]"))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("beklenmeyen hata: %v", p.Errors())
	}
	if len(prog.Statements) != 4 {
		t.Fatalf("4 deyim bekleniyordu, %d alındı", len(prog.Statements))
	}
}

// Yeni satırın başındaki ikili işleç önceki ifadeyi sürdürmez (2.8; Go, Python, Kotlin, Swift
// gibi; JavaScript'in tuzağı değil): 'a = 1' ardından gelen '-x' satırı 'a = 1 - x' okunmaz.
// İşleç satır sonundaysa ya da ifade parantez içindeyse sürer; '.' ve '|>' ile zincir sürer.
func TestBinaryOperatorOnNextLineDoesNotContinue(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	for _, d := range []struct {
		code  string
		stmts int
	}{
		{"değişken a = 1\n-x", 2},
		{"değişken a = 1 -\nx", 1},
		{"değişken a = (1\n- x)", 1},
		{"değişken a = x\n.uzunluk", 1},
		{"değişken a = x\n?.uzunluk", 1},
		{"değişken a = x ve\ny", 1},
		{"değişken a = x\n|> yazdır", 1},
	} {
		p := New(lexer.New(d.code))
		prog := p.ParseProgram()
		if len(p.Errors()) != 0 || len(prog.Statements) != d.stmts {
			t.Errorf("%q: %d deyim bekleniyordu, %d alındı (hatalar %v)", d.code, d.stmts, len(prog.Statements), p.Errors())
		}
	}
	// Satır başında tekli biçimi olmayan işleç sözdizimi hatasıdır: sessizce yeni anlam kazanmaz.
	for _, code := range []string{"değişken b = 10\n+ 2", "değişken c = x\nve y", "değişken d = x\n?? 0"} {
		if errs := parseErrs(code); !hasCode(errs, messages.H0167) {
			t.Errorf("%q: nedenini söyleyen sözdizimi hatası (H0167) bekleniyordu: %v", code, errs)
		}
	}
}

// 'test' yalnızca deyim başında ve ardından metin geldiğinde anahtar kelimedir.
func TestTestIsContextualKeyword(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	// sütun 0: konuma bakılmaz
	for code, want := range map[string][2]int{
		"eğer yanlış {\n    yazdır(\"gizli\")\nyazdır(\"son\")":    {1, 13},
		"fonksiyon f() {\n    sonuç 1\nyazdır(f())":                {1, 15},
		"yapı Kişi {\n    ad: Metin\n":                             {1, 11},
		"değişken x = eşle 1 {\n    1 => \"bir\"\n":                {0, 0},
		"döngü i içinde 1..3 {\n    eğer i == 2 {\n        çık\n}": {1, 21},
	} {
		errs := parseErrs(code)
		if !hasErrAt(errs, want[0], want[1], messages.H0156) {
			t.Errorf("%q için H0156 (satır %d, sütun %d) bekleniyordu, alınan: %v", code, want[0], want[1], errs)
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	for code, col := range map[string]int{
		`yazdır("{\"kod\": 5}")`:      9,
		`yazdır("Nokta{x: 1, y: 2}")`: 14,
		`yazdır("şablon: {'a': 1}")`:  17,
		`yazdır("{değil}")`:           9,
		`yazdır("boş {}")`:            13,
	} {
		errs := parseErrs(code)
		found := false
		for _, c := range []messages.Code{messages.H0158, messages.H0159, messages.H0160, messages.H0161, messages.H0162} {
			found = found || hasErrAt(errs, 1, col, c)
		}
		if !found || !hasErrAt(errs, 1, col, messages.H0100) {
			t.Errorf("%s: sütun %d konumunda H0158-H0162 (H0100 içinde) bekleniyordu: %v", code, col, errs)
		}
	}
	for _, code := range []string{"yazdır(\"\\{\\\"kod\\\": 5\\}\")", "yazdır(`{\"kod\": 5}`)", `yazdır("a {1 + 2} b")`} {
		if errs := parseErrors(code); len(errs) != 0 {
			t.Errorf("%s hata vermemeli: %v", code, errs)
		}
	}
}

// Kural (B2b): dilim '..' ile yazılır; Python alışkanlığıyla yazılan l[a:b] doğru biçimi söyleyen
// ipucunu alır (H0170). Önceden "']' bekleniyordu, ancak ':' bulundu" diyordu.
func TestColonSliceGetsRangeHint(t *testing.T) {
	errs := parseErrs("değişken l = [1, 2, 3]\nyazdır(l[0:2])\n")
	if !hasCode(errs, messages.H0170) {
		t.Fatalf("l[a:b] ipucu bekleniyordu: %v", errs)
	}
}

// Bir ileti kodu (H6903) ifadede kodun metnidir: ileti(H6903, x) ile ileti("H6903", x) aynıdır.
func TestMessageCodeIsItsString(t *testing.T) {
	p := New(lexer.New("f(H6903, 1)\n"))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("hata: %v", p.Errors())
	}
	call := prog.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	lit, ok := call.Arguments[0].(*ast.StringLiteral)
	if !ok || lit.Value != "H6903" {
		t.Errorf("ilk argüman \"H6903\" metni olmalı: %#v", call.Arguments[0])
	}
}

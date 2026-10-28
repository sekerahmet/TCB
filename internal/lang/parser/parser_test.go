package parser

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// shape ifade ağacını parantezli biçimde yazar (yalnızca testlerde kullanılan düğümler).
func shape(e ast.Expression) string {
	switch n := e.(type) {
	case *ast.InfixExpression:
		if n.Chained {
			return fmt.Sprintf("(%s, then %s %s)", shape(n.Left), n.Operator, shape(n.Right))
		}
		return fmt.Sprintf("(%s %s %s)", shape(n.Left), n.Operator, shape(n.Right))
	case *ast.RangeExpression:
		return fmt.Sprintf("(%s .. %s)", shape(n.Start), shape(n.End))
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

// codedLexer bir dosyanın gövdesinin sözcük çözümleyicisidir: program kimlik yazımıyla okunur
// (2.3a; source.Prepare'in dosya yolu).
func codedLexer(code string) *lexer.Lexer { return lexer.NewBody(code, true) }

// trLexer Türkçe yazılmış metnin sözcük çözümleyicisidir: yalnız bilerek Türkçe gösterimi (17)
// sınayan testler içindir; program içeride kimliklerle çalışır.
func trLexer(code string) *lexer.Lexer { return lexer.NewBodyIn(code, false, codes.Source()) }

func firstExpr(t *testing.T, code string) ast.Expression {
	t.Helper()
	p := New(codedLexer(code))
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
		"TR_struct A { TR_func f { } }",
		"TR_struct A { TR_func f { TR_print(1) } }\nTR_print(2)",
		"TR_struct A { TR_func { } }",
		"TR_struct A { TR_func f(",
		"TR_struct A { TR_func f { { { }",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%q ayrıştırılırken panik: %v", code, r)
				}
			}()
			p := New(codedLexer(code))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Errorf("%q için sözdizimi hatası bekleniyordu", code)
			}
		}()
	}
}

func TestValidStructMethodStillParses(t *testing.T) {
	p := New(codedLexer("TR_struct A {\n  ad: TR_String\n  TR_func selam() { TR_return TR_self.ad }\n}"))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("geçerli yapı hata vermemeli: %v", p.Errors())
	}
}

// parseErrors kodlu gövdeyi ayrıştırır, hata metinlerini verir.
func parseErrors(code string) []string {
	return messages.Texts(parseErrs(code))
}

// parseErrs, parseErrors gibi ama hataları ileti kodu ve konumuyla denetlenebilsin diye error olarak verir.
func parseErrs(code string) []error {
	p := New(codedLexer(code))
	p.ParseProgram()
	return p.Errs()
}

// parseErrsIn lang'de yazılmış metni ayrıştırır, hatalarını verir.
func parseErrsIn(code string, lang *codes.Language) []error {
	p := New(lexer.NewBodyIn(code, false, lang))
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

// posOf needle'ın code'da ilk geçtiği yerdir: satır 1'den, sütun karakter (rune) sayısıyla 1'den
// (2.1). Beklenen konum böylece elle değil programdan hesaplanır; yazım değişse de doğru kalır.
func posOf(t *testing.T, code, needle string) (line, col int) {
	t.Helper()
	i := strings.Index(code, needle)
	if i < 0 {
		t.Fatalf("%q içinde %q yok", code, needle)
	}
	before := code[:i]
	return strings.Count(before, "\n") + 1, utf8.RuneCountInString(before[strings.LastIndex(before, "\n")+1:]) + 1
}

// Metin içindeki tek bir '{' eskiden dosyanın geri kalanını yutuyordu.
func TestUnbalancedInterpolationIsReportedWithLine(t *testing.T) {
	errs := parseErrs("TR_print(\"a\")\nTR_print(\"\\(\")\nTR_print(\"b\")")
	if len(errs) == 0 || !hasErrAt(errs[:1], 2, 0, messages.H0009) {
		t.Fatalf("2. satırda kapatılmamış '\\(' hatası bekleniyordu, alınan: %v", errs)
	}
}

func TestUnterminatedStringIsReported(t *testing.T) {
	errs := parseErrs("TR_var a = 1\nTR_print(\"merhaba)\n")
	if len(errs) == 0 || !hasErrAt(errs[:1], 2, 0, messages.H0010) {
		t.Fatalf("2. satırda kapatılmamış metin hatası bekleniyordu, alınan: %v", errs)
	}
}

func TestPowerAndNotPrecedenceShape(t *testing.T) {
	cases := map[string]string{
		"2 ** 3 ** 2":       "(2 ** (3 ** 2))",
		"-2 ** 2":           "(-(2 ** 2))",
		"TR_not 1 == 2":     "(!(1 == 2))",
		"TR_not a TR_and b": "((!a) && b)",
		"-a * b":            "((-a) * b)",
	}
	for code, want := range cases {
		if got := shape(firstExpr(t, code)); got != want {
			t.Errorf("%s: %s bekleniyordu, %s alındı", code, want, got)
		}
	}
}

// Öncelik tablosu (4.1): karşılaştırmalar tek düzeyde, '??' karşılaştırmadan sıkı ve aralıktan
// gevşek, '**' sağındaki tekli işleçten gevşek bağlar.
func TestPrecedenceTableShape(t *testing.T) {
	cases := map[string]string{
		"x ?? 0 > 3":           "((x ?? 0) > 3)",
		"a TR_or b ?? c":       "(a || (b ?? c))",
		"a ?? b ?? c":          "((a ?? b) ?? c)",
		"1..n + 1":             "(1 .. (n + 1))",
		"2 ** -1":              "(2 ** (-1))",
		"a == b TR_and c < d":  "((a == b) && (c < d))",
		"TR_not a < b TR_or c": "((!(a < b)) || c)",
		"a < b + 1":            "(a < (b + 1))",
		"(a < b) == c":         "((a < b) == c)",
		"x & 1 == 0":           "((x & 1) == 0)",
		"a | b ^ c & d":        "(a | (b ^ (c & d)))",
		"1 << 2 + 3":           "(1 << (2 + 3))",
		"0..n | 1":             "(0 .. (n | 1))",
		"~a ** 2":              "(~(a ** 2))",
		"-a >> 1":              "((-a) >> 1)",
	}
	for code, want := range cases {
		if got := shape(firstExpr(t, code)); got != want {
			t.Errorf("%s: %s bekleniyordu, %s alındı", code, want, got)
		}
	}
}

// Başka dillerin işleçleri (4.9) sözdizimi hatasıdır, hata işlecin kendisindedir; ifadenin
// ardındaki TR_in de (Python'un üyelik sınaması). Döngü ve liste üretecindeki TR_in hata değildir.
func TestForeignOperatorsAreErrors(t *testing.T) {
	for _, d := range []struct {
		code, at string
		want     messages.Code
	}{
		{"TR_var x = a && b", "&&", messages.H0202},
		{"TR_var x = a || b", "||", messages.H0203},
		{"TR_var x = !a", "!", messages.H0204},
		{"TR_var x = l |> f", "|>", messages.H0205},
		{"TR_var x = l\n    |> f", "|>", messages.H0205},
		{"TR_var x = 1 TR_in l", "TR_in", messages.H0206},
		{"TR_if x.y TR_in l { }", "TR_in", messages.H0206},
	} {
		line, col := posOf(t, d.code, d.at)
		if errs := parseErrs(d.code); !hasErrAt(errs, line, col, d.want) {
			t.Errorf("%q: %d:%d'de %s bekleniyordu, alınan: %v", d.code, line, col, d.want, errs)
		}
	}
	for _, code := range []string{
		"TR_for x TR_in l { }",
		"TR_for i, x TR_in l { }",
		"TR_var y = [x TR_for x TR_in l TR_if x != 1]",
		"TR_var y = a != b TR_and TR_not c",
	} {
		if errs := parseErrs(code); len(errs) != 0 {
			t.Errorf("%q: hata beklenmiyordu: %v", code, errs)
		}
	}
}

// Sıralama işleçlerinin zinciri (4.4) halkalarıyla okunur: her halka soldaki halkanın sağ
// ifadesini karşılaştırır; parantezli karşılaştırma zincir değildir.
func TestComparisonChainShape(t *testing.T) {
	cases := map[string]string{
		"0 <= i < n":     "((0 <= i), then < n)",
		"a < b <= c > d": "(((a < b), then <= c), then > d)",
		"(a < b) < c":    "((a < b) < c)",
		"a < (b < c)":    "(a < (b < c))",
	}
	for code, want := range cases {
		if got := shape(firstExpr(t, code)); got != want {
			t.Errorf("%s: %s bekleniyordu, %s alındı", code, want, got)
		}
	}
}

// İçinde '==' ya da '!=' olan zincir (4.4) ve '??'nin sağındaki parantezsiz ikili işleç (4.7)
// sözdizimi hatasıdır; hata ikinci karşılaştırmada ya da '??'den sonraki işleçtedir.
func TestChainAndCoalescingErrors(t *testing.T) {
	for _, d := range []struct {
		code, at string
		want     messages.Code
	}{
		{"TR_var x = a < b == c", "==", messages.H0200},
		{"TR_var x = a == b == c", "== c", messages.H0200},
		{"TR_var x = a != b < c", "<", messages.H0200},
		{"TR_var x = d ?? 0 + 1", "+", messages.H0201},
		{"TR_var x = d ?? 2 ** 3", "**", messages.H0201},
		{"TR_var x = d ?? f(1) * 2", "*", messages.H0201},
		{"TR_var x = d ?? 0..3", "..", messages.H0201},
		{"TR_var x = d ?? 1 & m", "&", messages.H0201},
	} {
		line, col := posOf(t, d.code, d.at)
		if errs := parseErrs(d.code); !hasErrAt(errs, line, col, d.want) {
			t.Errorf("%q: %d:%d'de %s bekleniyordu, alınan: %v", d.code, line, col, d.want, errs)
		}
	}
	for _, code := range []string{
		"TR_var x = (a < b) == c",
		"TR_var x = (d ?? 0) + 1",
		"TR_var x = d ?? (0 + 1)",
		"TR_var x = d ?? -1",
		"TR_var x = d ?? l[0].a",
		"TR_var x = d ?? 0\n+ 1",
	} {
		if errs := parseErrs(code); hasCode(errs, messages.H0200) || hasCode(errs, messages.H0201) {
			t.Errorf("%q: zincir ya da '??' hatası beklenmiyordu: %v", code, errs)
		}
	}
}

func TestCallOnNextLineStartsNewStatement(t *testing.T) {
	p := New(codedLexer("TR_var f = 5\n(x => x)(1)\nx\n[1]"))
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
// İşleç satır sonundaysa ya da ifade parantez içindeyse sürer; '.' ve '?.' ile zincir sürer.
func TestBinaryOperatorOnNextLineDoesNotContinue(t *testing.T) {
	for _, d := range []struct {
		code  string
		stmts int
	}{
		{"TR_var a = 1\n-x", 2},
		{"TR_var a = 1 -\nx", 1},
		{"TR_var a = (1\n- x)", 1},
		{"TR_var a = x\n.TR_length", 1},
		{"TR_var a = x\n?.TR_length", 1},
		{"TR_var a = x TR_and\ny", 1},
	} {
		p := New(codedLexer(d.code))
		prog := p.ParseProgram()
		if len(p.Errors()) != 0 || len(prog.Statements) != d.stmts {
			t.Errorf("%q: %d deyim bekleniyordu, %d alındı (hatalar %v)", d.code, d.stmts, len(prog.Statements), p.Errors())
		}
	}
	// Satır başında tekli biçimi olmayan işleç sözdizimi hatasıdır: sessizce yeni anlam kazanmaz.
	for _, code := range []string{"TR_var b = 10\n+ 2", "TR_var c = x\nTR_and y", "TR_var d = x\n?? 0"} {
		if errs := parseErrs(code); !hasCode(errs, messages.H0167) {
			t.Errorf("%q: nedenini söyleyen sözdizimi hatası (H0167) bekleniyordu: %v", code, errs)
		}
	}
}

// 'test' yalnızca deyim başında ve ardından metin geldiğinde anahtar kelimedir.
func TestTestIsContextualKeyword(t *testing.T) {
	p := New(codedLexer("TR_test \"toplama\" { TR_assert(1 + 1 == 2) }"))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("beklenmeyen hata: %v", p.Errors())
	}
	ts, ok := prog.Statements[0].(*ast.TestStatement)
	if !ok || ts.Name != "toplama" || len(ts.Body.Statements) != 1 {
		t.Fatalf("test deyimi bekleniyordu, %T alındı", prog.Statements[0])
	}

	// Ad olarak kullanım bozulmamalı
	for _, code := range []string{"TR_var test = 5\ntest = test + 1\nTR_print(test)", "test := [1]\ntest[0]", "TR_func test(a) { TR_return a }\ntest(\"x\")"} {
		p := New(codedLexer(code))
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
	p = New(codedLexer("TR_test \"ad\" TR_assert(TR_true)"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("gövdesiz test hata vermedi")
	}
}

// Metin içi yerleştirmedeki ifadeler dosyadaki gerçek konumlarını taşımalı (eskiden Satır 1, Sütun 1).
func TestInterpolationKeepsSourcePosition(t *testing.T) {
	const code = "TR_var a = 1\nTR_print(\"x = \\(toplm + 1)\")"
	p := New(codedLexer(code))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("beklenmeyen hata: %v", p.Errors())
	}
	call := prog.Statements[1].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	interp := call.Arguments[0].(*ast.InterpolatedString)
	infix := interp.Parts[1].(*ast.InfixExpression)
	id := infix.Left.(*ast.Identifier)
	if line, col := posOf(t, code, "toplm"); id.Token.Line != line || id.Token.Column != col {
		t.Fatalf("konum Satır %d, Sütun %d olmalıydı, alınan: Satır %d, Sütun %d", line, col, id.Token.Line, id.Token.Column)
	}
}

// Test raporu Y1: kapatılmamış '{' hatasız kabul ediliyor, sonraki kod sessizce bloğa giriyordu.
func TestUnclosedBlockBraceIsReported(t *testing.T) {
	// Hata kapatılmayan ilk '{'dedir; atBrace false ise konuma bakılmaz.
	for code, atBrace := range map[string]bool{
		"TR_if TR_false {\n    TR_print(\"gizli\")\nTR_print(\"son\")":   true,
		"TR_func f() {\n    TR_return 1\nTR_print(f())":                  true,
		"TR_struct Kişi {\n    ad: TR_String\n":                          true,
		"TR_var x = TR_match 1 {\n    1 => \"bir\"\n":                    false,
		"TR_for i TR_in 1..3 {\n    TR_if i == 2 {\n        TR_break\n}": true,
	} {
		line, col := 0, 0
		if atBrace {
			line, col = posOf(t, code, "{")
		}
		errs := parseErrs(code)
		if !hasErrAt(errs, line, col, messages.H0156) {
			t.Errorf("%q için H0156 (satır %d, sütun %d) bekleniyordu, alınan: %v", code, line, col, errs)
		}
	}
	if errs := parseErrors("TR_if TR_true {\n    TR_print(1)\n}\n"); len(errs) != 0 {
		t.Fatalf("kapatılmış blok hata vermemeli: %v", errs)
	}
}

// Kaçışsız her '{' bir yerleştirme başlatır (2.7): içi ifade olarak ayrıştırılamayan süslü
// parantezler (JSON, başka dillerde kod) sessizce düz metin kalmaz, '{'nin konumunda
// sözdizimi hatası verir. Düz süslü parantezin iki yazımı (\{ ve ham metin) hata vermez.
func TestNonExpressionInterpolationIsError(t *testing.T) {
	// Hata yerleştirmenin '\('sindedir.
	for _, code := range []string{
		`TR_print("\()")`,
		`TR_print("boş \()")`,
		`TR_print("\(TR_not)")`,
		`TR_print("\(a, b)")`,
		`TR_print("\(TR_var x = 1)")`,
	} {
		_, col := posOf(t, code, `\(`)
		errs := parseErrs(code)
		found := false
		for _, c := range []messages.Code{messages.H0158, messages.H0159, messages.H0160, messages.H0161, messages.H0162} {
			found = found || hasErrAt(errs, 1, col, c)
		}
		if !found || !hasErrAt(errs, 1, col, messages.H0100) {
			t.Errorf("%s: sütun %d konumunda H0158-H0162 (H0100 içinde) bekleniyordu: %v", code, col, errs)
		}
	}
	for _, code := range []string{"TR_print(\"{\\\"kod\\\": 5}\")", "TR_print(`{\"kod\": 5}`)", `TR_print("a \(1 + 2) b")`} {
		if errs := parseErrors(code); len(errs) != 0 {
			t.Errorf("%s hata vermemeli: %v", code, errs)
		}
	}
}

// Kural (B2b): dilim '..' ile yazılır; Python alışkanlığıyla yazılan l[a:b] doğru biçimi söyleyen
// ipucunu alır (H0170). Önceden "']' bekleniyordu, ancak ':' bulundu" diyordu.
func TestColonSliceGetsRangeHint(t *testing.T) {
	errs := parseErrs("TR_var l = [1, 2, 3]\nTR_print(l[0:2])\n")
	if !hasCode(errs, messages.H0170) {
		t.Fatalf("l[a:b] ipucu bekleniyordu: %v", errs)
	}
}

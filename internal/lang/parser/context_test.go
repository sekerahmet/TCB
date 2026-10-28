package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Döngü dışındaki 'çık'/'devam' çalışma anında sessizce yok sayılıyor (fonksiyon 'yok'
// dönüyor, testler yanlışlıkla geçiyordu). Derleme hatasıdır: fonksiyon, ok fonksiyonu,
// sondaki blok, test, ertele ve eşzamanlı gövdesi döngünün içini kesmez, sınır sayılır.
func TestBreakContinueOutsideLoopIsError(t *testing.T) {
	bad := []string{
		"TR_break",
		"TR_continue",
		"TR_func f() { TR_break }",
		"TR_for { TR_func f() { TR_break } }",
		"TR_var f = (x) => { TR_continue }",
		"TR_var g = (a, b) => { TR_break }",
		"TR_test \"t\" { TR_break }",
		"TR_for { TR_defer { TR_break } }",
		"TR_for { TR_async { TR_continue } }",
		"TR_if TR_true { TR_break }",
	}
	for _, codeVal := range bad {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0151) {
			t.Errorf("%q: H0151 bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	// Sondaki blok döngü değildir (6.7): iletisi TR_return'ü değil blokta dalları önerir (H0264).
	for _, codeVal := range []string{"TR_print(TR_error_code { TR_continue })", "TR_for { TR_print(TR_error_code { TR_if TR_true { TR_break } }) }"} {
		if h := parseErrs(codeVal); len(h) == 0 || !messages.Has(h[0], messages.H0264) {
			t.Errorf("%q: H0264 bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	// Fonksiyonda TR_break yazılırsa hata (H0151) çıkış için TR_return'ü önerir; dosyadan gelen hatanın
	// önerisi kimlik yazımıyladır, iletinin her dilinde aynı (2.3a).
	h := parseErrs("TR_func f() { TR_break }")
	if len(h) == 0 || !messages.Has(h[0], messages.H0151) {
		t.Fatalf("TR_return önerisi (H0151) bekleniyordu, alınan: %v", h)
	}
	for _, lang := range []string{"tr", "en"} {
		if name := codes.Spelling(codes.KwReturn); !strings.Contains(messages.ErrorIn(h[0], lang), "'"+name+"'") {
			t.Errorf("%s iletisinde '%s' önerisi bekleniyordu: %s", lang, name, messages.ErrorIn(h[0], lang))
		}
	}
	valid := []string{
		"TR_for { TR_break }",
		"TR_for TR_true { TR_continue }",
		"TR_for i TR_in 1..3 { TR_if i > 1 { TR_break } }",
		"TR_for i TR_in 1..3 { TR_match i { 1 => { TR_continue } _ => { TR_break } } }",
		"TR_for x < 3 { TR_try { TR_break } TR_catch { TR_continue } }",
		"TR_func f() { TR_for { TR_break } }",
		"TR_print(TR_error_code { TR_for { TR_break }; 1 })",
		"TR_for { TR_for { TR_break }; TR_break }",
	}
	for _, codeVal := range valid {
		if h := parseErrors(codeVal); len(h) > 0 {
			t.Errorf("%q geçerli olmalıydı: %v", codeVal, h)
		}
	}
}

// Test gövdesinde fonksiyon dışındaki 'sonuç' derleme hatasıdır (testi sessizce geçiriyordu);
// test içindeki bir fonksiyonda geçerlidir.
func TestReturnInTestBodyIsError(t *testing.T) {
	for _, codeVal := range []string{
		"TR_test \"t\" { TR_return }",
		"TR_test \"t\" { TR_if TR_true { TR_return 1 } }",
		"TR_test \"t\" { TR_try { TR_print(1) } TR_catch h { TR_return TR_none } }",
		"TR_test \"t\" { TR_match 1 { 1 => TR_return 2 } }",
	} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0153) {
			t.Errorf("%q: test gövdesi hatası (H0153) bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	for _, codeVal := range []string{
		"TR_test \"t\" { TR_func f() { TR_return 1 }; TR_assert_equal(1, f()) }",
		"TR_test \"t\" { TR_var f = (x) => x + 1 }",
	} {
		if h := parseErrors(codeVal); len(h) > 0 {
			t.Errorf("%q geçerli olmalıydı: %v", codeVal, h)
		}
	}
}

// Programın en dışındaki 'sonuç' sözdizimi hatasıdır (1.10): program yalnız TR_exit ile erken biter;
// ileti çıkış çağrısını adıyla ve başarı koduyla söyler. Fonksiyon içinde geçerlidir; ertele ve
// eşzamanlı blok programın en dışındaysa fonksiyon sayılmaz, orada da hatadır.
func TestProgramTopLevelReturnIsError(t *testing.T) {
	for _, codeVal := range []string{
		"TR_return 5",
		"TR_var a = 1\nTR_return",
		"TR_if TR_true { TR_return 1 }",
		"TR_for { TR_return 1 }",
		"TR_defer { TR_return }",
		"TR_async { TR_return 2 }",
	} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0171) {
			t.Errorf("%q: en dıştaki 'sonuç' hatası (H0171) bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	if h := parseErrs("TR_func f() { TR_return 1 }\nTR_var g = (x) => x"); len(h) > 0 {
		t.Errorf("fonksiyon içindeki sonuç geçerli olmalıydı: %v", h)
	}
}

// Modülün üst düzeyindeki 'sonuç' (ParseModule) derleme hatasıdır; fonksiyon içinde geçerlidir.
func TestModuleTopLevelReturnIsError(t *testing.T) {
	moduleErrors := func(codeVal string) []error {
		p := New(codedLexer(codeVal))
		p.ParseModule()
		return p.Errs()
	}
	for _, codeVal := range []string{"TR_var a = 1\nTR_return 5", "TR_if TR_true { TR_return }", "TR_for { TR_return 1 }"} {
		h := moduleErrors(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0154) {
			t.Errorf("%q: modül hatası (H0154) bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	if h := moduleErrors("TR_func f() { TR_return 1 }\nTR_var g = (x) => x"); len(h) > 0 {
		t.Errorf("fonksiyon içindeki sonuç geçerli olmalıydı: %v", h)
	}
}

// Çok derin iç içe girdi Go yığınını taşırıp süreci yakalanamaz biçimde çökertiyordu;
// iç içelik sınırı Türkçe bir sözdizimi hatası verir.
func TestNestingDepthLimit(t *testing.T) {
	for _, codeVal := range []string{
		strings.Repeat("[", 200000),
		"TR_var x = " + strings.Repeat("(", 5000) + "1" + strings.Repeat(")", 5000),
		strings.Repeat("TR_if TR_true { ", 3000) + strings.Repeat("}", 3000),
		strings.Repeat("-", 100000) + "1",
	} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0155) {
			t.Errorf("derinlik hatası (H0155) bekleniyordu (%d karakter): %.200v", len(codeVal), h)
		}
	}
	// Sınırın altındaki iç içelik geçerlidir
	if h := parseErrors("TR_var x = " + strings.Repeat("[", 500) + strings.Repeat("]", 500)); len(h) > 0 {
		t.Errorf("500 düzey geçerli olmalı: %.200v", h)
	}
}

// Metin içindeki yerleştirmenin ifadesi de dosyanın iç içelik sınırına sayılır: alt
// ayrıştırıcı sıfırdan başlasaydı iç içe metinlerle sınırın katları kadar derine inilirdi.
func TestInterpolationDepthLimited(t *testing.T) {
	codeVal := strings.Repeat("TR_if TR_true {\n", 480) +
		`TR_var s = "\(` + strings.Repeat("(", 900) + "1" + strings.Repeat(")", 900) + ")\"\n" +
		strings.Repeat("}\n", 480)
	h := parseErrs(codeVal)
	if len(h) != 1 || !hasErrAt(h, 481, 0, messages.H0155) {
		t.Errorf("yerleştirmede derinlik hatası (satır 481) bekleniyordu: %.300v", h)
	}
	// Aynı ifade üst düzeyde sınırın altındadır
	if h := parseErrors(`TR_var s = "\(` + strings.Repeat("(", 900) + "1" + strings.Repeat(")", 900) + `)"`); len(h) > 0 {
		t.Errorf("900 düzey yerleştirme geçerli olmalı: %.200v", h)
	}
}

// Çok yerleştirmeli bir metin doğrusal sürede ayrıştırılır (her yerleştirmenin konumu
// metnin başından yeniden sayılmaz; 200 000 yerleştirme önceden ~40 sn sürüyordu). Duvar saati
// tavanı yerine büyüme ölçülür (testkit.ScalesLinearly: 50 000 ve 200 000 yerleştirme).
func TestManyInterpolationsLinear(t *testing.T) {
	testkit.ScalesLinearly(t, "yerleştirmeli metin", 50000, func(n int) func() (int, error) {
		code := `TR_var s = "` + strings.Repeat(`\(a)`, n) + `"`
		return func() (int, error) {
			p := New(codedLexer(code))
			prog := p.ParseProgram()
			if err := errors.Join(p.Errs()...); err != nil {
				return 0, err
			}
			// Birim ayrıştırılan yerleştirme parçasıdır.
			if v, ok := prog.Statements[0].(*ast.VarStatement); ok {
				if s, ok := v.Value.(*ast.InterpolatedString); ok {
					return len(s.Parts), nil
				}
			}
			return 0, nil
		}
	})
}

// aktar yalnızca metin olarak yazılmış bir yol alır (§11): tırnaksız yazım sözdizimi hatasıdır.
func TestImportUnquotedPathIsError(t *testing.T) {
	for _, codeVal := range []string{"TR_import yardımcı", "TR_import 5", "TR_import"} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0143) {
			t.Errorf("%q: tırnaksız aktar hatası bekleniyordu: %v", codeVal, h)
		}
	}
	if h := parseErrors("TR_import \"yardımcı\" TR_as y"); len(h) > 0 {
		t.Errorf("tırnaklı aktar geçerli olmalı: %v", h)
	}
}

// Bağlamsal sözcükler (olarak, test) sözcük çözümleyicinin okuduğu dildedir, etkin dilde değil:
// Türkçe yazılmış dosya başka bir etkin dilde de Türkçe ayrıştırılır (17.7). Burada etkin dil
// Türkçeyken İngilizce gövde ayrıştırılır.
func TestContextWordsFollowLexer(t *testing.T) {
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	src := "import \"m\" as g\ntest \"t\" { }\n"
	p := New(lexer.NewBodyIn(src, false, en))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("İngilizce gövde ayrıştırılamadı: %v", errs)
	}
	got := prog.String()
	for _, want := range []string{`Alias: "g"`, `Name: "t"`} {
		if !strings.Contains(got, want) {
			t.Errorf("ağaçta %q yok:\n%s", want, got)
		}
	}
}

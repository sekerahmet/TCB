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
		"çık",
		"devam",
		"fonksiyon f() { çık }",
		"döngü { fonksiyon f() { çık } }",
		"[1, 2].haritala { eğer o > 1 { devam } }",
		"değişken f = x => { devam }",
		"değişken g = (a, b) => { çık }",
		"test \"t\" { çık }",
		"döngü { ertele { çık } }",
		"döngü { eşzamanlı { devam } }",
		"eğer doğru { çık }",
	}
	for _, codeVal := range bad {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0151) {
			t.Errorf("%q: H0151 bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	// Fonksiyonda 'çık' yazılırsa hata (H0151) çıkış için 'sonuç'u önerir; ad kod başvurusudur,
	// iletinin dilinde yazılır (Türkçede sonuç, İngilizcede return).
	h := parseErrs("fonksiyon f() { çık }")
	if len(h) == 0 || !messages.Has(h[0], messages.H0151) {
		t.Fatalf("'sonuç' önerisi (H0151) bekleniyordu, alınan: %v", h)
	}
	for _, lang := range []string{"tr", "en"} {
		if name := codes.NameRef(codes.KwReturn).MessageText(lang); !strings.Contains(messages.ErrorIn(h[0], lang), "'"+name+"'") {
			t.Errorf("%s iletisinde '%s' önerisi bekleniyordu: %s", lang, name, messages.ErrorIn(h[0], lang))
		}
	}
	valid := []string{
		"döngü { çık }",
		"döngü doğru { devam }",
		"döngü i içinde 1..3 { eğer i > 1 { çık } }",
		"döngü i içinde 1..3 { eşle i { 1 => { devam } _ => { çık } } }",
		"döngü x < 3 { dene { çık } yakala { devam } }",
		"fonksiyon f() { döngü { çık } }",
		"[1].haritala { döngü { çık }; o }",
		"döngü { döngü { çık }; çık }",
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
		"test \"t\" { sonuç }",
		"test \"t\" { eğer doğru { sonuç 1 } }",
		"test \"t\" { dene { yazdır(1) } yakala h { sonuç yok } }",
		"test \"t\" { eşle 1 { 1 => sonuç 2 } }",
	} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0153) {
			t.Errorf("%q: test gövdesi hatası (H0153) bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	for _, codeVal := range []string{
		"test \"t\" { fonksiyon f() { sonuç 1 }; eşit_mi(1, f()) }",
		"test \"t\" { değişken f = x => x + 1 }",
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
		"sonuç 5",
		"değişken a = 1\nsonuç",
		"eğer doğru { sonuç 1 }",
		"döngü { sonuç 1 }",
		"ertele { sonuç }",
		"eşzamanlı { sonuç 2 }",
	} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0171) {
			t.Errorf("%q: en dıştaki 'sonuç' hatası (H0171) bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	if h := parseErrs("fonksiyon f() { sonuç 1 }\ndeğişken g = x => x"); len(h) > 0 {
		t.Errorf("fonksiyon içindeki sonuç geçerli olmalıydı: %v", h)
	}
}

// Modülün üst düzeyindeki 'sonuç' (ParseModule) derleme hatasıdır; fonksiyon içinde geçerlidir.
func TestModuleTopLevelReturnIsError(t *testing.T) {
	moduleErrors := func(codeVal string) []error {
		p := New(trLexer(codeVal))
		p.ParseModule()
		return p.Errs()
	}
	for _, codeVal := range []string{"değişken a = 1\nsonuç 5", "eğer doğru { sonuç }", "döngü { sonuç 1 }"} {
		h := moduleErrors(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0154) {
			t.Errorf("%q: modül hatası (H0154) bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	if h := moduleErrors("fonksiyon f() { sonuç 1 }\ndeğişken g = x => x"); len(h) > 0 {
		t.Errorf("fonksiyon içindeki sonuç geçerli olmalıydı: %v", h)
	}
}

// Çok derin iç içe girdi Go yığınını taşırıp süreci yakalanamaz biçimde çökertiyordu;
// iç içelik sınırı Türkçe bir sözdizimi hatası verir.
func TestNestingDepthLimit(t *testing.T) {
	for _, codeVal := range []string{
		strings.Repeat("[", 200000),
		"değişken x = " + strings.Repeat("(", 5000) + "1" + strings.Repeat(")", 5000),
		strings.Repeat("eğer doğru { ", 3000) + strings.Repeat("}", 3000),
		strings.Repeat("-", 100000) + "1",
	} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0155) {
			t.Errorf("derinlik hatası (H0155) bekleniyordu (%d karakter): %.200v", len(codeVal), h)
		}
	}
	// Sınırın altındaki iç içelik geçerlidir
	if h := parseErrors("değişken x = " + strings.Repeat("[", 500) + strings.Repeat("]", 500)); len(h) > 0 {
		t.Errorf("500 düzey geçerli olmalı: %.200v", h)
	}
}

// Metin içindeki yerleştirmenin ifadesi de dosyanın iç içelik sınırına sayılır: alt
// ayrıştırıcı sıfırdan başlasaydı iç içe metinlerle sınırın katları kadar derine inilirdi.
func TestInterpolationDepthLimited(t *testing.T) {
	codeVal := strings.Repeat("eğer doğru {\n", 480) +
		`değişken s = "{` + strings.Repeat("(", 900) + "1" + strings.Repeat(")", 900) + "}\"\n" +
		strings.Repeat("}\n", 480)
	h := parseErrs(codeVal)
	if len(h) != 1 || !hasErrAt(h, 481, 0, messages.H0155) {
		t.Errorf("yerleştirmede derinlik hatası (satır 481) bekleniyordu: %.300v", h)
	}
	// Aynı ifade üst düzeyde sınırın altındadır
	if h := parseErrors(`değişken s = "{` + strings.Repeat("(", 900) + "1" + strings.Repeat(")", 900) + `}"`); len(h) > 0 {
		t.Errorf("900 düzey yerleştirme geçerli olmalı: %.200v", h)
	}
}

// Çok yerleştirmeli bir metin doğrusal sürede ayrıştırılır (her yerleştirmenin konumu
// metnin başından yeniden sayılmaz; 200 000 yerleştirme önceden ~40 sn sürüyordu). Duvar saati
// tavanı yerine büyüme ölçülür (testkit.ScalesLinearly: 50 000 ve 200 000 yerleştirme).
func TestManyInterpolationsLinear(t *testing.T) {
	testkit.ScalesLinearly(t, "yerleştirmeli metin", 50000, func(n int) func() (int, error) {
		code := `değişken s = "` + strings.Repeat("{a}", n) + `"`
		return func() (int, error) {
			p := New(trLexer(code))
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
	for _, codeVal := range []string{"aktar yardımcı", "aktar 5", "aktar"} {
		h := parseErrs(codeVal)
		if len(h) == 0 || !messages.Has(h[0], messages.H0143) {
			t.Errorf("%q: tırnaksız aktar hatası bekleniyordu: %v", codeVal, h)
		}
	}
	if h := parseErrors("aktar \"yardımcı\" olarak y"); len(h) > 0 {
		t.Errorf("tırnaklı aktar geçerli olmalı: %v", h)
	}
}

// Bağlamsal sözcükler (olarak, test, sondaki bloğun 'o'su, indirge'nin 'birikim'i) sözcük
// çözümleyicinin okuduğu dildedir, etkin dilde değil: Türkçe yazılmış dosya başka bir etkin
// dilde de Türkçe ayrıştırılır (17.7). Burada etkin dil Türkçeyken İngilizce gövde ayrıştırılır.
func TestContextWordsFollowLexer(t *testing.T) {
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	src := "import \"m\" as g\ntest \"t\" { }\nvar a = [1].map { it * 2 }\nvar b = [1].reduce(0) { acc + it }\n"
	p := New(lexer.NewBodyIn(src, false, en))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("İngilizce gövde ayrıştırılamadı: %v", errs)
	}
	got := prog.String()
	for _, want := range []string{`Name: "it"`, `Name: "acc"`} {
		if !strings.Contains(got, want) {
			t.Errorf("ağaçta %q yok:\n%s", want, got)
		}
	}
}

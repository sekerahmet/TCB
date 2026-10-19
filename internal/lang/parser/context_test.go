package parser

import (
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Döngü dışındaki 'çık'/'devam' çalışma anında sessizce yok sayılıyor (fonksiyon 'yok'
// dönüyor, testler yanlışlıkla geçiyordu). Derleme hatasıdır: fonksiyon, ok fonksiyonu,
// sondaki blok, test, ertele ve eşzamanlı gövdesi döngünün içini kesmez, sınır sayılır.
func TestBreakContinueOutsideLoopIsError(t *testing.T) {
	bad := map[string]string{
		"çık":                             "'çık' yalnızca bir döngünün içinde kullanılabilir",
		"devam":                           "'devam' yalnızca bir döngünün içinde kullanılabilir",
		"fonksiyon f() { çık }":           "fonksiyondan çıkmak için 'sonuç' yazın",
		"döngü { fonksiyon f() { çık } }": "'çık' yalnızca",
		"[1, 2].haritala { eğer o > 1 { devam } }": "'devam' yalnızca",
		"değişken f = x => { devam }":              "'devam' yalnızca",
		"değişken g = (a, b) => { çık }":           "'çık' yalnızca",
		"test \"t\" { çık }":                       "'çık' yalnızca",
		"döngü { ertele { çık } }":                 "'çık' yalnızca",
		"döngü { eşzamanlı { devam } }":            "'devam' yalnızca",
		"eğer doğru { çık }":                       "'çık' yalnızca",
	}
	for codeVal, wantSub := range bad {
		h := parseErrors(codeVal)
		if len(h) == 0 || !strings.Contains(h[0], wantSub) {
			t.Errorf("%q: %q içeren hata bekleniyordu, alınan: %v", codeVal, wantSub, h)
		}
	}
	valid := []string{
		"döngü { çık }",
		"iken doğru { devam }",
		"döngü i içinde 1..3 { eğer i > 1 { çık } }",
		"döngü i içinde 1..3 { eşle i { 1 => { devam } varsayılan => { çık } } }",
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
		h := parseErrors(codeVal)
		if len(h) == 0 || !strings.Contains(h[0], "'sonuç' test gövdesinde kullanılamaz") {
			t.Errorf("%q: test gövdesi hatası bekleniyordu, alınan: %v", codeVal, h)
		}
	}
	for _, codeVal := range []string{
		"test \"t\" { fonksiyon f() { sonuç 1 }; eşit_mi(1, f()) }",
		"test \"t\" { değişken f = x => x + 1 }",
		"sonuç 5", // programın üst düzeyinde 'sonuç' programı bitirir
	} {
		if h := parseErrors(codeVal); len(h) > 0 {
			t.Errorf("%q geçerli olmalıydı: %v", codeVal, h)
		}
	}
}

// Modülün üst düzeyindeki 'sonuç' (ParseModule) derleme hatasıdır; fonksiyon içinde geçerlidir.
func TestModuleTopLevelReturnIsError(t *testing.T) {
	moduleErrors := func(codeVal string) []string {
		p := New(lexer.New(codeVal))
		p.ParseModule()
		return p.Errors()
	}
	for _, codeVal := range []string{"değişken a = 1\nsonuç 5", "eğer doğru { sonuç }", "döngü { sonuç 1 }"} {
		h := moduleErrors(codeVal)
		if len(h) == 0 || !strings.Contains(h[0], "'sonuç' modülün üst düzeyinde kullanılamaz") {
			t.Errorf("%q: modül hatası bekleniyordu, alınan: %v", codeVal, h)
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
		h := parseErrors(codeVal)
		if len(h) == 0 || !strings.Contains(h[0], "çok derin iç içe") {
			t.Errorf("derinlik hatası bekleniyordu (%d karakter): %.200v", len(codeVal), h)
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
	h := parseErrors(codeVal)
	if len(h) != 1 || !strings.Contains(h[0], "çok derin iç içe") || !strings.HasPrefix(h[0], "Satır 481,") {
		t.Errorf("yerleştirmede derinlik hatası (satır 481) bekleniyordu: %.300v", h)
	}
	// Aynı ifade üst düzeyde sınırın altındadır
	if h := parseErrors(`değişken s = "{` + strings.Repeat("(", 900) + "1" + strings.Repeat(")", 900) + `}"`); len(h) > 0 {
		t.Errorf("900 düzey yerleştirme geçerli olmalı: %.200v", h)
	}
}

// Çok yerleştirmeli bir metin doğrusal sürede ayrıştırılır (her yerleştirmenin konumu
// metnin başından yeniden sayılmaz; 200 000 yerleştirme önceden ~40 sn sürüyordu).
func TestManyInterpolationsLinear(t *testing.T) {
	begin := time.Now()
	h := parseErrors(`değişken s = "` + strings.Repeat("{a}", 200000) + `"`)
	if len(h) > 0 {
		t.Fatalf("beklenmeyen hata: %.200v", h)
	}
	if elapsed := time.Since(begin); elapsed > 10*time.Second {
		t.Errorf("200 000 yerleştirmeli metin %v sürdü; ayrıştırma karesel olabilir", elapsed)
	}
}

// aktar yalnızca metin olarak yazılmış bir yol alır (§11): tırnaksız yazım sözdizimi hatasıdır.
func TestImportUnquotedPathIsError(t *testing.T) {
	for _, codeVal := range []string{"aktar yardımcı", "aktar 5", "aktar"} {
		h := parseErrors(codeVal)
		if len(h) == 0 || !strings.Contains(h[0], "modül yolunu metin olarak bekler") {
			t.Errorf("%q: tırnaksız aktar hatası bekleniyordu: %v", codeVal, h)
		}
	}
	if h := parseErrors("aktar \"yardımcı\" olarak y"); len(h) > 0 {
		t.Errorf("tırnaklı aktar geçerli olmalı: %v", h)
	}
}

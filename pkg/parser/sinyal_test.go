package parser

import (
	"strings"
	"testing"
	"tr_dili/pkg/lexer"
)

// Döngü dışındaki 'çık'/'devam' çalışma anında sessizce yok sayılıyor (fonksiyon 'yok'
// dönüyor, testler yanlışlıkla geçiyordu). Derleme hatasıdır: fonksiyon, ok fonksiyonu,
// sondaki blok, test, ertele ve eşzamanlı gövdesi döngünün içini kesmez, sınır sayılır.
func TestDonguDisiCikVeDevamHatadir(t *testing.T) {
	hatali := map[string]string{
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
	for kod, ister := range hatali {
		h := parseErrors(kod)
		if len(h) == 0 || !strings.Contains(h[0], ister) {
			t.Errorf("%q: %q içeren hata bekleniyordu, alınan: %v", kod, ister, h)
		}
	}
	gecerli := []string{
		"döngü { çık }",
		"iken doğru { devam }",
		"döngü i içinde 1..3 { eğer i > 1 { çık } }",
		"döngü i içinde 1..3 { eşle i { 1 => { devam } varsayılan => { çık } } }",
		"döngü x < 3 { dene { çık } yakala { devam } }",
		"fonksiyon f() { döngü { çık } }",
		"[1].haritala { döngü { çık }; o }",
		"döngü { döngü { çık }; çık }",
	}
	for _, kod := range gecerli {
		if h := parseErrors(kod); len(h) > 0 {
			t.Errorf("%q geçerli olmalıydı: %v", kod, h)
		}
	}
}

// Test gövdesinde fonksiyon dışındaki 'sonuç' derleme hatasıdır (testi sessizce geçiriyordu);
// test içindeki bir fonksiyonda geçerlidir.
func TestTestGovdesindeSonucHatadir(t *testing.T) {
	for _, kod := range []string{
		"test \"t\" { sonuç }",
		"test \"t\" { eğer doğru { sonuç 1 } }",
		"test \"t\" { dene { yazdır(1) } yakala h { sonuç yok } }",
		"test \"t\" { eşle 1 { 1 => sonuç 2 } }",
	} {
		h := parseErrors(kod)
		if len(h) == 0 || !strings.Contains(h[0], "'sonuç' test gövdesinde kullanılamaz") {
			t.Errorf("%q: test gövdesi hatası bekleniyordu, alınan: %v", kod, h)
		}
	}
	for _, kod := range []string{
		"test \"t\" { fonksiyon f() { sonuç 1 }; eşit_mi(1, f()) }",
		"test \"t\" { değişken f = x => x + 1 }",
		"sonuç 5", // programın üst düzeyinde 'sonuç' programı bitirir
	} {
		if h := parseErrors(kod); len(h) > 0 {
			t.Errorf("%q geçerli olmalıydı: %v", kod, h)
		}
	}
}

// Modülün üst düzeyindeki 'sonuç' (ParseModule) derleme hatasıdır; fonksiyon içinde geçerlidir.
func TestModulUstDuzeySonucHatadir(t *testing.T) {
	modulHatalari := func(kod string) []string {
		p := New(lexer.New(kod))
		p.ParseModule()
		return p.Errors()
	}
	for _, kod := range []string{"değişken a = 1\nsonuç 5", "eğer doğru { sonuç }", "döngü { sonuç 1 }"} {
		h := modulHatalari(kod)
		if len(h) == 0 || !strings.Contains(h[0], "'sonuç' modülün üst düzeyinde kullanılamaz") {
			t.Errorf("%q: modül hatası bekleniyordu, alınan: %v", kod, h)
		}
	}
	if h := modulHatalari("fonksiyon f() { sonuç 1 }\ndeğişken g = x => x"); len(h) > 0 {
		t.Errorf("fonksiyon içindeki sonuç geçerli olmalıydı: %v", h)
	}
}

// Çok derin iç içe girdi Go yığınını taşırıp süreci yakalanamaz biçimde çökertiyordu;
// iç içelik sınırı Türkçe bir sözdizimi hatası verir.
func TestIcIceDerinlikSiniri(t *testing.T) {
	for _, kod := range []string{
		strings.Repeat("[", 200000),
		"değişken x = " + strings.Repeat("(", 5000) + "1" + strings.Repeat(")", 5000),
		strings.Repeat("eğer doğru { ", 3000) + strings.Repeat("}", 3000),
		strings.Repeat("-", 100000) + "1",
	} {
		h := parseErrors(kod)
		if len(h) == 0 || !strings.Contains(h[0], "çok derin iç içe") {
			t.Errorf("derinlik hatası bekleniyordu (%d karakter): %.200v", len(kod), h)
		}
	}
	// Sınırın altındaki iç içelik geçerlidir
	if h := parseErrors("değişken x = " + strings.Repeat("[", 500) + strings.Repeat("]", 500)); len(h) > 0 {
		t.Errorf("500 düzey geçerli olmalı: %.200v", h)
	}
}

// aktar yalnızca metin olarak yazılmış bir yol alır (§11): tırnaksız yazım sözdizimi hatasıdır.
func TestAktarTirnaksizYolHatadir(t *testing.T) {
	for _, kod := range []string{"aktar yardımcı", "aktar 5", "aktar"} {
		h := parseErrors(kod)
		if len(h) == 0 || !strings.Contains(h[0], "modül yolunu metin olarak bekler") {
			t.Errorf("%q: tırnaksız aktar hatası bekleniyordu: %v", kod, h)
		}
	}
	if h := parseErrors("aktar \"yardımcı\" olarak y"); len(h) > 0 {
		t.Errorf("tırnaklı aktar geçerli olmalı: %v", h)
	}
}

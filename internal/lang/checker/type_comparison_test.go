package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// boolTextWarnings kanonik (Türkçe) kaynağı dosyanın dili language iken denetler ve H1016
// uyarılarının sayısını verir.
func boolTextWarnings(t *testing.T, language, source string) int {
	t.Helper()
	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v", p.Errors())
	}
	c := New()
	c.SetFileLanguage(language)
	if errs := c.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("hata beklenmiyordu: %v", errs)
	}
	n := 0
	for _, w := range c.Warnings() {
		if messages.Has(w, messages.H1016) {
			n++
		}
	}
	return n
}

// Mantık değerinin metni dosyanın dilindedir (3): metin(Mantık) başka bir yazım dilinin
// doğru/yanlış adıyla karşılaştırılırsa karşılaştırma hiç tutmaz, denetleyici uyarır (5. adım;
// 3.11'in tip(x) == "Tam" kuralıyla aynı aile). Dosyanın dilindeki ad ve başka metinler sessizdir.
func TestBoolTextComparisonWarning(t *testing.T) {
	// Tipi bilinen Mantık: sabit, tip açıklamalı değişken, karşılaştırma (açıklamasız değişken
	// dinamik dilde tipsiz sayılır, checker.go VarStatement).
	const decl = "sabit b = doğru\ndeğişken c: Mantık = yanlış\n"
	for _, c := range []struct {
		language, line string
		want           int
	}{
		{"en", `yazdır(metin(b) == "doğru")`, 1},
		{"en", `yazdır("yanlış" != metin(1 > 2))`, 1},
		{"en", `yazdır(metin(b) == "true")`, 0},
		{"tr", `yazdır(metin(b) == "true")`, 1},
		{"tr", `yazdır(metin(b) == "doğru")`, 0},
		{"", `yazdır(metin(b) == "false")`, 1}, // dili verilmemiş dosya Türkçedir
		{"en", `yazdır(metin(b) == "evet")`, 0},
		{"en", `yazdır(metin(3) == "doğru")`, 0}, // Mantık değil
		{"en", `yazdır(metin(c) == "yanlış")`, 1},
	} {
		if got := boolTextWarnings(t, c.language, decl+c.line+"\n"); got != c.want {
			t.Errorf("dil=%q %s: %d uyarı, beklenen %d", c.language, c.line, got, c.want)
		}
	}
	// metin gölgelenmişse kullanıcının fonksiyonudur.
	if got := boolTextWarnings(t, "en", decl+"fonksiyon metin(x) { sonuç \"doğru\" }\nyazdır(metin(b) == \"doğru\")\n"); got != 0 {
		t.Errorf("gölgelenen metin: %d uyarı", got)
	}
}

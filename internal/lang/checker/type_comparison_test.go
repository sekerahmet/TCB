package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// boolTextWarnings kodlu gövdeyi dosyanın dili language iken denetler ve H1016 uyarılarının
// sayısını verir.
func boolTextWarnings(t *testing.T, language, source string) int {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
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

// Mantık değerinin metni dilden bağımsızdır (codes.ValueNames; "true", "false"): TR_str(Mantık) bir
// yazım dilinin doğru/yanlış adıyla karşılaştırılırsa karşılaştırma hiç tutmaz, denetleyici uyarır
// (3.11'in TR_type(x) == "Tam" kuralıyla aynı aile). Değer metnindeki ad ve başka metinler sessizdir;
// dosyada da yazılan satırda (dil "tr", "en") da aynıdır.
func TestBoolTextComparisonWarning(t *testing.T) {
	// Tipi bilinen Mantık: sabit, tip açıklamalı değişken, karşılaştırma (açıklamasız değişken
	// dinamik dilde tipsiz sayılır, checker.go VarStatement).
	const decl = "TR_const b = TR_true\nTR_var c: TR_Bool = TR_false\n"
	for _, c := range []struct {
		line string
		want int
	}{
		{`TR_print(TR_str(b) == "doğru")`, 1},
		{`TR_print("yanlış" != TR_str(1 > 2))`, 1},
		{`TR_print(TR_str(b) == "true")`, 0},
		{`TR_print(TR_str(c) == "false")`, 0},
		{`TR_print(TR_str(b) == "evet")`, 0},
		{`TR_print(TR_str(b) == "True")`, 1}, // başka bir dilden tanıdık ad (Python)
		{`TR_print(TR_str(c) == "False")`, 1},
		{`TR_print(TR_str(3) == "doğru")`, 0}, // Mantık değil
		{`TR_print(TR_str(c) == "yanlış")`, 1},
	} {
		for _, language := range []string{"", "tr", "en"} {
			if got := boolTextWarnings(t, language, decl+c.line+"\n"); got != c.want {
				t.Errorf("dil=%q %s: %d uyarı, beklenen %d", language, c.line, got, c.want)
			}
		}
	}
	// metin gölgelenmişse kullanıcının fonksiyonudur.
	if got := boolTextWarnings(t, "en", decl+"TR_func metin(x) { TR_return \"doğru\" }\nTR_print(metin(b) == \"doğru\")\n"); got != 0 {
		t.Errorf("gölgelenen metin: %d uyarı", got)
	}
}

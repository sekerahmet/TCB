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

// Mantık değerinin metni dosyanın dilindedir (3): metin(Mantık) başka bir yazım dilinin
// doğru/yanlış adıyla karşılaştırılırsa karşılaştırma hiç tutmaz, denetleyici uyarır (5. adım;
// 3.11'in tip(x) == "Tam" kuralıyla aynı aile). Dosyanın dilindeki ad ve başka metinler sessizdir.
func TestBoolTextComparisonWarning(t *testing.T) {
	// Tipi bilinen Mantık: sabit, tip açıklamalı değişken, karşılaştırma (açıklamasız değişken
	// dinamik dilde tipsiz sayılır, checker.go VarStatement).
	const decl = "TR_kw_const b = TR_kw_true\nTR_kw_var c: TR_type_Bool = TR_kw_false\n"
	for _, c := range []struct {
		language, line string
		want           int
	}{
		{"en", `TR_fn_print(TR_fn_str(b) == "doğru")`, 1},
		{"en", `TR_fn_print("yanlış" != TR_fn_str(1 > 2))`, 1},
		{"en", `TR_fn_print(TR_fn_str(b) == "true")`, 0},
		{"tr", `TR_fn_print(TR_fn_str(b) == "true")`, 1},
		{"tr", `TR_fn_print(TR_fn_str(b) == "doğru")`, 0},
		{"en", `TR_fn_print(TR_fn_str(b) == "evet")`, 0},
		{"en", `TR_fn_print(TR_fn_str(3) == "doğru")`, 0}, // Mantık değil
		{"en", `TR_fn_print(TR_fn_str(c) == "yanlış")`, 1},
	} {
		if got := boolTextWarnings(t, c.language, decl+c.line+"\n"); got != c.want {
			t.Errorf("dil=%q %s: %d uyarı, beklenen %d", c.language, c.line, got, c.want)
		}
	}
	// metin gölgelenmişse kullanıcının fonksiyonudur.
	if got := boolTextWarnings(t, "en", decl+"TR_kw_func metin(x) { TR_kw_return \"doğru\" }\nTR_fn_print(metin(b) == \"doğru\")\n"); got != 0 {
		t.Errorf("gölgelenen metin: %d uyarı", got)
	}
}

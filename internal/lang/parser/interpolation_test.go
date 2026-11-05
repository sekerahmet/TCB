package parser

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerleştirmenin aralığını sözcük çözümleyici bulur: içindeki tırnaklı metinlerde süslü
// parantez sayılmaz (2.7). Eskiden ayrıştırıcı süslü parantezleri ayrıca sayıyordu; tırnaklı
// metindeki '}' yerleştirmeyi erken kapatıyor, '{' ise metni sessizce boş metne çeviriyordu.
func TestQuotedStringInInterpolation(t *testing.T) {
	if errs := parseErrors(`TR_print("x \("a)b") y")`); len(errs) != 0 {
		t.Errorf("tırnaklı metindeki ')' yerleştirmeyi kapatmamalı: %v", errs)
	}
	if errs := parseErrors(`TR_print("\("\(")")`); len(errs) == 0 {
		t.Error("kapanmayan iç yerleştirme sessizce boş metin olmamalı, hata vermeli")
	}
}

// Üye adı bir ad ya da demetin indeksidir (x.1, 3.6a): başka bir sözcük sessizce üye adı sayılmaz.
// İndeks yalnız '.'den sonra gelir, '?.'den sonra değil.
func TestMemberNameMustBeName(t *testing.T) {
	for _, codeVal := range []string{"x?.1", "x.$", "x?.TR_if", "x.1e5"} {
		if errs := parseErrors("TR_var x = 1\n" + codeVal); len(errs) == 0 {
			t.Errorf("%s hata vermeli", codeVal)
		}
	}
}

// Kural (6.6): metin yerleştirmesindeki TR_func da ifade yerindedir; hata dosyanın başka yerinde
// TR_func yazılmasına bağlı değildir.
func TestAFunctionInsideInterpolationIsAnExpression(t *testing.T) {
	for _, c := range []struct {
		code string
		want messages.Code
	}{
		{`TR_var s = "\(TR_func f() { })"`, messages.H0269},
		{`TR_var s = "\(TR_func(x) { })"`, messages.H0268},
	} {
		line, col := posOf(t, c.code, "TR_func")
		if errs := parseErrs(c.code); !hasErrAt(errs, line, col, c.want) {
			t.Errorf("%q: %d:%d'de %s bekleniyordu, alınan: %v", c.code, line, col, c.want, errs)
		}
	}
	if errs := parseErrs(`TR_var s = "\((x) => x)"`); len(errs) != 0 {
		t.Errorf("metindeki ok geçerlidir: %v", errs)
	}
}

package parser

import "testing"

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

// Üye adı bir ad olmalıdır: başka bir sözcük sessizce üye adı sayılmaz.
func TestMemberNameMustBeName(t *testing.T) {
	for _, codeVal := range []string{"x.1", "x.$", "x?.TR_if"} {
		if errs := parseErrors("TR_var x = 1\n" + codeVal); len(errs) == 0 {
			t.Errorf("%s hata vermeli", codeVal)
		}
	}
}

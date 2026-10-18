package ayristir

import "testing"

// Yerleştirmenin aralığını sözcük çözümleyici bulur: içindeki tırnaklı metinlerde süslü
// parantez sayılmaz (2.7). Eskiden ayrıştırıcı süslü parantezleri ayrıca sayıyordu; tırnaklı
// metindeki '}' yerleştirmeyi erken kapatıyor, '{' ise metni sessizce boş metne çeviriyordu.
func TestYerlestirmedekiTirnakliMetin(t *testing.T) {
	if errs := parseErrors(`yazdır("x {"a}b"} y")`); len(errs) != 0 {
		t.Errorf("tırnaklı metindeki '}' yerleştirmeyi kapatmamalı: %v", errs)
	}
	if errs := parseErrors(`yazdır("{"{"}")`); len(errs) == 0 {
		t.Error("kapanmayan iç yerleştirme sessizce boş metin olmamalı, hata vermeli")
	}
}

// Üye adı bir ad olmalıdır: başka bir sözcük sessizce üye adı sayılmaz.
func TestUyeAdiBirAdOlmali(t *testing.T) {
	for _, kod := range []string{"x.1", "x.$", "x?.eğer"} {
		if errs := parseErrors("değişken x = 1\n" + kod); len(errs) == 0 {
			t.Errorf("%s hata vermeli", kod)
		}
	}
}

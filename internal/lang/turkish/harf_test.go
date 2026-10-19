package turkish

import "testing"

func TestTurkceHarfDonusumu(t *testing.T) {
	kucuk := map[string]string{"IŞIK": "ışık", "İşler": "işler", "ISIK": "ısık", "Çay": "çay"}
	for g, b := range kucuk {
		if a := KucukHarf(g); a != b {
			t.Errorf("KucukHarf(%q) = %q, beklenen %q", g, a, b)
		}
	}
	buyuk := map[string]string{"ışık": "IŞIK", "işler": "İŞLER", "limit": "LİMİT", "seç": "SEÇ"}
	for g, b := range buyuk {
		if a := BuyukHarf(g); a != b {
			t.Errorf("BuyukHarf(%q) = %q, beklenen %q", g, a, b)
		}
	}
}

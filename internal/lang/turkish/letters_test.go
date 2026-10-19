package turkish

import "testing"

func TestTurkishCaseConversion(t *testing.T) {
	lower := map[string]string{"IŞIK": "ışık", "İşler": "işler", "ISIK": "ısık", "Çay": "çay"}
	for g, b := range lower {
		if a := Lower(g); a != b {
			t.Errorf("KucukHarf(%q) = %q, beklenen %q", g, a, b)
		}
	}
	upper := map[string]string{"ışık": "IŞIK", "işler": "İŞLER", "limit": "LİMİT", "seç": "SEÇ"}
	for g, b := range upper {
		if a := Upper(g); a != b {
			t.Errorf("BuyukHarf(%q) = %q, beklenen %q", g, a, b)
		}
	}
}

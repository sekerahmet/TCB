package checker

import "testing"

// Tip açıklamasında kabul edilen yazımlar sözlükten gelir (A2xx); her tipin tek yazımı
// vardır (büyük/küçük harf fark etmez).
func TestTipYazimlariAyni(t *testing.T) {
	beklenen := map[string]string{
		"tam": "Tam", "ondalık": "Ondalık", "metin": "Metin", "mantık": "Mantık",
		"liste": "Liste", "sözlük": "Sözlük", "tensör": "Tensör", "yok": "Yok",
	}
	if len(tipYazimlari) != len(beklenen) {
		t.Errorf("%d yazım bekleniyordu, %d var: %v", len(beklenen), len(tipYazimlari), tipYazimlari)
	}
	for y, tip := range beklenen {
		if got := normalizeTypeName(y); got != tip {
			t.Errorf("normalizeTypeName(%q) = %q, %q bekleniyordu", y, got, tip)
		}
	}
	for _, c := range []struct{ in, want string }{{"", "Herhangi"}, {"TAM", "Tam"}, {"Herhangi", "Herhangi"}, {"herhangi", "herhangi"}, {"Nokta", "Nokta"}} {
		if got := normalizeTypeName(c.in); got != c.want {
			t.Errorf("normalizeTypeName(%q) = %q, %q bekleniyordu", c.in, got, c.want)
		}
	}
}

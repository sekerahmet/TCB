package denetci

import "testing"

// Tip açıklamasında kabul edilen yazımlar sözlükten gelir (A2xx); her tipin tek yazımı
// vardır, büyük/küçük harfi farklı biçim kabul edilen bir yazım değildir (3.14).
func TestTipYazimlariAyni(t *testing.T) {
	beklenen := []string{"Tam", "Ondalık", "Metin", "Mantık", "Liste", "Sözlük", "Tensör", "Yok",
		"Aralık", "Fonksiyon", "YerleşikFonksiyon", "Tip"}
	if len(tipYazimlari) != len(beklenen) {
		t.Errorf("%d yazım bekleniyordu, %d var: %v", len(beklenen), len(tipYazimlari), tipYazimlari)
	}
	for _, tip := range beklenen {
		if got, ok := tipYazimlari[tip]; !ok || got != tip {
			t.Errorf("tipYazimlari[%q] = %q, %v; kendisi bekleniyordu", tip, got, ok)
		}
		if _, ok := tipYazimlari[kucukHarf(tip)]; ok {
			t.Errorf("%q kabul edilen bir yazım olmamalı (3.14)", kucukHarf(tip))
		}
	}
	for _, c := range []struct{ in, want string }{{"", "Herhangi"}, {"TAM", "Tam"}, {"Herhangi", "Herhangi"}, {"herhangi", "herhangi"}, {"Nokta", "Nokta"}} {
		if got := normalizeTypeName(c.in); got != c.want {
			t.Errorf("normalizeTypeName(%q) = %q, %q bekleniyordu", c.in, got, c.want)
		}
	}
}

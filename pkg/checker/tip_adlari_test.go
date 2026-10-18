package checker

import "testing"

// Tip açıklamasında kabul edilen yazımlar sözlükten gelir (A2xx ve eş yazımları); kabul
// edilen küme sözlüğe taşınmadan önceki kümeyle aynı olmalı.
func TestTipYazimlariAyni(t *testing.T) {
	beklenen := map[string]string{
		"tam": "Tam", "tamsayı": "Tam", "int": "Tam",
		"ondalık": "Ondalık", "sayı": "Ondalık", "float": "Ondalık",
		"metin": "Metin", "yazı": "Metin", "str": "Metin", "string": "Metin",
		"mantık": "Mantık", "mantıksal": "Mantık", "bool": "Mantık",
		"liste": "Liste", "dizi": "Liste", "list": "Liste",
		"sözlük": "Sözlük", "harita": "Sözlük", "dict": "Sözlük", "map": "Sözlük",
		"tensör": "Tensör", "matris": "Tensör", "matrix": "Tensör",
		"yok": "Yok", "boş": "Yok", "null": "Yok",
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

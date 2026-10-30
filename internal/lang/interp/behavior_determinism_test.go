package interp

import "testing"

// Sıra belirlenimlidir: sözlükler ekleme sırasını korur, yapı alanları tanım sırasıyla gezilir;
// çıktı Go harita gezinme sırasına bağlı olmaz. Go haritaları her gezişte farklı sıra verebilir;
// bu yüzden aynı program birçok kez çalıştırılır ve çıktı her seferinde aynı, beklenen sırada
// olmalıdır. Sözlük sırasının dil kuralı ayrıca tests/tipler_test.tr'de (3.7) sınanır.
func TestOutputOrderIsDeterministic(t *testing.T) {
	const program = `
TR_struct Nokta { zz: TR_Int = 1; a: TR_Int = 2; m: TR_Int = 3; b: TR_Int = 4; k: TR_Int = 5; c: TR_Int = 6 }
TR_var p = Nokta()
TR_var d = {"z": 1, "a": 2, "m": 3, "b": 4, "k": 5, "c": 6}
d["en_son"] = 7
TR_print(p)
TR_print(TR_String(p))
TR_print(d)
TR_print(d.TR_keys())
TR_print(d.TR_values())
TR_for k TR_in d { TR_print(k) }
TR_var q = Nokta(c: 60, a: 20)
TR_print(q)
`
	const want = "Nokta(zz: 1, a: 2, m: 3, b: 4, k: 5, c: 6)\n" +
		"Nokta(zz: 1, a: 2, m: 3, b: 4, k: 5, c: 6)\n" +
		`{"z": 1, "a": 2, "m": 3, "b": 4, "k": 5, "c": 6, "en_son": 7}` + "\n" +
		`["z", "a", "m", "b", "k", "c", "en_son"]` + "\n" +
		"[1, 2, 3, 4, 5, 6, 7]\nz\na\nm\nb\nk\nc\nen_son\n" +
		"Nokta(zz: 1, a: 20, m: 3, b: 4, k: 5, c: 60)\n"
	first, _, err := runTROut(t, program)
	if err != nil {
		t.Fatal(err)
	}
	if first != want {
		t.Fatalf("sıra tanım/ekleme sırasında olmalı:\n%s", first)
	}
	for i := 0; i < 40; i++ {
		got, _, err := runTROut(t, program)
		if err != nil || got != first {
			t.Fatalf("%d. çalıştırmada çıktı değişti (err %v):\n%s\n---\n%s", i+2, err, first, got)
		}
	}
}

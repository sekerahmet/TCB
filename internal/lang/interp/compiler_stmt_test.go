package interp

import (
	"strings"
	"testing"
)

// Kural (9.1, 9.2): ertelenin kapsamı onu içeren en yakın bloktur: dal, döngünün turu, koşullu
// döngünün turu, dene ve yakala bloğu; iş blok nasıl biterse bitsin (son deyim, TR_continue,
// TR_break, TR_return, hata) o bloğun sonunda çalışır. Eskiden fonksiyonun sonunu bekliyordu.
func TestDeferScopeIsTheNearestBlock(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"TR_func f() {\n    TR_if TR_true {\n        TR_defer { TR_print(\"dal bitti\") }\n        TR_print(\"dal\")\n    }\n    TR_print(\"son\")\n}\nf()", "dal\ndal bitti\nson"},
		{"TR_for n TR_in 1..=2 {\n    TR_defer { TR_print(\"\\(n) bitti\") }\n    TR_print(n)\n}", "1\n1 bitti\n2\n2 bitti"},
		{"TR_var i = 0\nTR_for i < 3 {\n    TR_defer { TR_print(\"tur \\(i)\") }\n    i += 1\n    TR_if i == 1 { TR_continue }\n    TR_if i == 2 { TR_break }\n}", "tur 1\ntur 2"},
		{"TR_func f() -> TR_Int {\n    TR_for n TR_in 1..=3 {\n        TR_defer { TR_print(\"\\(n) bitti\") }\n        TR_if n == 2 { TR_return n }\n    }\n    TR_return 0\n}\nTR_print(f())", "1 bitti\n2 bitti\n2"},
		{"TR_try {\n    TR_defer { TR_print(\"dene bitti\") }\n    TR_throw \"x\"\n} TR_catch e {\n    TR_defer { TR_print(\"yakala bitti\") }\n    TR_print(e)\n}", "dene bitti\nx\nyakala bitti"},
	} {
		out, _, err := runTROut(t, c.source+"\n")
		if err != nil || strings.TrimSpace(out) != c.want {
			t.Errorf("%s:\n%q beklenirdi, gelen %q (%v)", c.source, c.want, out, err)
		}
	}
}

// Kural (9.1): ertelenen bloğun kendisi bir kapsamdır; içinde kaydedilen ertele o bloğun sonunda,
// ters sırayla çalışır (eskiden sessizce kayboluyordu). Fonksiyonda, programın en dışında ve iç içe
// üç düzeyde; iç ertelenin hatası yutulmaz.
func TestDeferInsideADeferredBlockRunsAtItsEnd(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"TR_func f() {\n    TR_defer {\n        TR_defer { TR_print(\"iç\") }\n        TR_print(\"dış\")\n    }\n    TR_print(\"gövde\")\n}\nf()\nTR_print(\"son\")", "gövde\ndış\niç\nson"},
		{"TR_defer {\n    TR_defer { TR_print(\"iç\") }\n    TR_print(\"dış\")\n}\nTR_print(\"gövde\")", "gövde\ndış\niç"},
		{"TR_defer {\n    TR_defer {\n        TR_defer { TR_print(3) }\n        TR_print(2)\n    }\n    TR_defer { TR_print(1) }\n    TR_print(0)\n}", "0\n1\n2\n3"},
	} {
		out, _, err := runTROut(t, c.source+"\n")
		if err != nil || strings.TrimSpace(out) != c.want {
			t.Errorf("%s:\n%q beklenirdi, gelen %q (%v)", c.source, c.want, out, err)
		}
	}
	_, _, err := runTROut(t, "TR_defer {\n    TR_defer { TR_throw \"iç hata\" }\n    TR_print(1)\n}\n")
	if err == nil || !strings.Contains(err.Error(), "iç hata") {
		t.Errorf("iç ertelenin hatası yutulmamalı: %v", err)
	}
}

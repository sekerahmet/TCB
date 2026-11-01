package interp

import (
	"strings"
	"testing"
)

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

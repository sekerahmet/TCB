package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (10.1): TR_async f(x) ve TR_async { ... } görev değeri verir. g.TR_wait() görev bitene kadar
// bekler ve sonucunu verir (blokta TR_none); g.TR_done() beklemeden görevin durup durmadığını söyler.
// Görev değerinin metni dilden bağımsızdır (3.14): tipinin adı.
func TestATaskValueGivesItsResult(t *testing.T) {
	const source = `TR_func kare(x: TR_Int) -> TR_Int {
    TR_sleep(0.02)
    TR_return x * x
}
TR_var g = TR_async kare(3)
TR_print(g.TR_done())
TR_print(g.TR_wait(), g.TR_done(), g.TR_wait())
TR_var b = TR_async { TR_print("blok") }
TR_print(b.TR_wait())
TR_print(g, TR_String(g))
`
	out, _, err := runTROut(t, source)
	const want = "false\n9 true 9\nblok\nnone\nTask Task\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (10.1, 10.6): görev hatayla bittiyse g.TR_wait() o hatayı fırlatır; TR_catch onu yakalar.
func TestWaitThrowsTheTasksError(t *testing.T) {
	const source = `TR_func böl(x: TR_Int) -> TR_Int {
    TR_return 10 // x
}
TR_var g = TR_async böl(0)
TR_print(TR_error_code { g.TR_wait() })
`
	out, _, err := runTROut(t, source)
	if err != nil || out != "H2470\n" {
		t.Fatalf("\"H2470\" beklenirdi, gelen %q (%v)", out, err)
	}
}

// Kural (10.1): görev kendi durmasını bekleyemez: kendi değerinin .TR_wait()'i sonsuza dek beklerdi,
// hatadır (H2918); hata satırıyla görevin hatası olur.
func TestATaskCannotWaitForItself(t *testing.T) {
	const source = `TR_var h = TR_async { TR_print(h.TR_wait()) }
TR_print(TR_error_code { h.TR_wait() })
`
	out, _, err := runTROut(t, source)
	if err != nil || out != "H2918\n" {
		t.Fatalf("\"H2918\" beklenirdi, gelen %q (%v)", out, err)
	}
	_, _, err = runTROut(t, "TR_var h = TR_async { TR_print(1) }\nh.TR_wait()\nh.TR_done(1)\n")
	if err == nil {
		t.Fatal("argümanlı TR_done hata vermeli")
	}
}

// Kural (13.11): görevde olmayan üye, denetimdeki hatanın kendisidir (H2474).
func TestATaskHasOnlyItsMembers(t *testing.T) {
	_, err := runTR(t, "TR_var g = TR_async TR_sum([1])\ng.TR_wait()\ng.TR_append(1)\n")
	if !messages.Has(err, messages.H2474) {
		t.Fatalf("H2474 beklenirdi, gelen %v", err)
	}
}

// Kural (10.5): TR_wait_all(görevler) görevlerin hepsini bekler, sonuçları verilen sırayla bir liste
// olarak verir. Hatalarda verilen sıradaki ilk hatalı görevin hatası fırlar (bitme sırası değiştirmez),
// öbürleri onun TR_suppressed'ine eklenir. Görev olmayan öğe H2919.
func TestWaitAllGivesResultsAndTheFirstErrorInTheGivenOrder(t *testing.T) {
	const source = `TR_func iş(x: TR_Int, süre: TR_Float) -> TR_Int {
    TR_sleep(süre)
    TR_return x * 10
}
TR_func hata(m: TR_String, süre: TR_Float) -> TR_Int {
    TR_sleep(süre)
    TR_throw m
}
TR_print(TR_wait_all([TR_async iş(1, 0.03), TR_async iş(2, 0.0), TR_async iş(3, 0.01)]), TR_wait_all([]))
TR_try {
    TR_wait_all([TR_async iş(1, 0.0), TR_async hata("ikinci", 0.03), TR_async hata("üçüncü", 0.0)])
} TR_catch e {
    TR_print(e, e.TR_suppressed)
}
TR_print(TR_error_code { TR_wait_all([TR_async iş(1, 0.0), 5]) })
`
	out, _, err := runTROut(t, source)
	const want = "[10, 20, 30] []\nikinci [üçüncü]\nH2919\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

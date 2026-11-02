package interp

import "testing"

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

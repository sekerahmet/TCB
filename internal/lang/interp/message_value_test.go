package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ileti(H…, değerler) katalog iletisini bir değer olarak kurar; fırlat ona verilince hata o
// kodla fırlar. Yakalanan hatanın metni iletiyi kuran çağrıdır (3.14, 8.3); kataloğun metni yalnız
// gösterilen hatadadır (eskiden yakalanan metin kataloğun metniydi).
func TestMessageThrown(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_try { TR_throw TR_Message("H6903", -1) } TR_catch h { TR_print(h) }
		TR_print(TR_Type(TR_Message("H6903", 2)))
	`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Message(\"H6903\", -1)\nMessage\n" {
		t.Errorf("çıktı: %q", out)
	}
	_, _, err = runTROut(t, "TR_throw TR_Message(\"H6903\", -1)")
	if !messages.Has(err, messages.H6903) {
		t.Errorf("yakalanmayan hata H6903 kodunu taşımalı: %v", err)
	}
}

// doğrula(koşul, ileti(…)) koşul yanlışken iletiyi kendi koduyla fırlatır (H2456'ya sarmaz).
func TestMessageAssert(t *testing.T) {
	_, _, err := runTROut(t, "TR_assert(1 > 2, TR_Message(\"H6903\", 5))")
	if !messages.Has(err, messages.H6903) || messages.Has(err, messages.H2456) {
		t.Errorf("doğrula iletiyi olduğu gibi fırlatmalı: %v", err)
	}
	if _, _, err := runTROut(t, "TR_assert(2 > 1, TR_Message(\"H6903\", 5))"); err != nil {
		t.Errorf("doğru koşul hata vermemeli: %v", err)
	}
}

// hata_kodu(f) f'yi çalıştırır; fırlayan iletinin kodunu verir, hata yoksa yok.
func TestErrorCode(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_print(TR_error_code { TR_throw TR_Message("H6903", -1) })
		TR_print(TR_error_code { TR_assert(TR_false, TR_Message("H6900", 10, 4)) })
		TR_print(TR_error_code { 1 + 1 })
		TR_print(TR_error_code { TR_throw TR_Message("H6903", -1) } == "H6903")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "H6903\nH6900\nnone\ntrue\n"; out != want {
		t.Errorf("çıktı %q, %q bekleniyordu", out, want)
	}
}

// Kural (8.5): düz metin hatanın kodu yoktur; kodsuz hatada TR_error_code koda dönmez, hatayı olduğu
// gibi yeniden fırlatır (eskiden her değer H2441 veriyordu): metin, durumlu tip değeri. Fırlatılamayan
// değer (sayı, sözlük) kendi çalışma hatasıdır (H2911, 8.2).
func TestErrorCodeRethrowsACodelessError(t *testing.T) {
	for thrown, want := range map[string]string{`"düz metin"`: "\ndüz metin\n", "Renk.Kırmızı": "\nRenk.Kırmızı\n", "5": "H2911\n\n", `{"kod": 5}`: "H2911\n\n"} {
		source := "TR_enum Renk { Kırmızı }\nTR_var m = \"\"\nTR_try { TR_print(TR_error_code { TR_throw " + thrown + " }) } TR_catch h { m = TR_String(h) }\nTR_print(m)\n"
		out, _, err := runTROut(t, source)
		if err != nil || out != strings.TrimPrefix(want, "\n") {
			t.Errorf("TR_throw %s: çıktı %q, beklenen %q, hata %v", thrown, out, strings.TrimPrefix(want, "\n"), err)
		}
	}
}

// Kural (8.5): TR_error_code hatanın kodunu, o yoksa nedenler zincirinde (TR_cause) dıştan içe ilk
// kodu verir; zincirde kod yoksa hata, nedeniyle birlikte yeniden fırlar. Eskiden kodsuz bir hatanın
// nedenine bakılmıyordu.
func TestErrorCodeFollowsTheCauseChain(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_func zincir(iç: TR_Bool) {
			TR_try {
				TR_try { TR_if iç { TR_throw TR_Message("H6903", -1) } TR_else { TR_throw "iç" } } TR_catch e { TR_throw TR_Error("orta", TR_cause: e) }
			} TR_catch f {
				TR_throw TR_Error("dış", TR_cause: f)
			}
		}
		TR_print(TR_error_code { zincir(TR_true) })
		TR_try { TR_print(TR_error_code { zincir(TR_false) }) } TR_catch h { TR_print(h, h.TR_cause, h.TR_cause.TR_cause) }
	`)
	if want := "H6903\ndış orta iç\n"; err != nil || out != want {
		t.Errorf("çıktı %q, beklenen %q, hata %v", out, want, err)
	}
}

// ileti tanımsız kodu ve yanlış değer sayısını kurulurken bildirir.
func TestMessageErrors(t *testing.T) {
	_, _, err := runTROut(t, `TR_Message("H9998", 1)`)
	if !messages.Has(err, messages.H2528) {
		t.Errorf("tanımsız kod H2528 vermeli: %v", err)
	}
	_, _, err = runTROut(t, "TR_Message(\"H6903\")")
	if !messages.Has(err, messages.H2529) {
		t.Errorf("yanlış değer sayısı H2529 vermeli: %v", err)
	}
	_, _, err = runTROut(t, "TR_Message()")
	if err == nil {
		t.Error("kodsuz TR_Message() hata vermeli")
	}
}

// Kural (3.14, 8.5): iletinin değer olarak metni onu kuran çağrıdır, tip adıyla, her dilde aynı
// (eskiden metnin istendiği dilin katalog metniydi); okuyanın dilindeki metin yalnız gösterilen
// hatada kurulur.
func TestMessageValueTextIsItsCall(t *testing.T) {
	m := &MessageVal{Code: messages.H6903, Args: []Value{&IntVal{Val: -1}}}
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []string{m.Inspect(), valueText(m, en), valueText(m, codes.Source())} {
		if got != `Message("H6903", -1)` {
			t.Errorf("değerin metni %q", got)
		}
	}
	if got := messages.ErrorIn(m.Error(), "en"); !strings.Contains(got, "cannot be negative, got -1") {
		t.Errorf("gösterilen hata, en: %q", got)
	}
	if got := messages.ErrorIn(m.Error(), "tr"); !strings.Contains(got, "negatif olamaz, -1 verildi") {
		t.Errorf("gösterilen hata, tr: %q", got)
	}
}

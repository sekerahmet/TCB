package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ileti(H…, değerler) katalog iletisini bir değer olarak kurar; fırlat ona verilince hata o
// kodla fırlar, yakalanan metni kataloğun metnidir ve içindeki kitaplık adları (ad yuvaları)
// dosyanın dilinin adlarıdır.
func TestMessageThrown(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_try { TR_throw TR_Message("H6903", -1) } TR_catch h { TR_print(h) }
		TR_print(TR_Type(TR_Message("H6903", 2)))
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cannot be negative, got -1") || !strings.Contains(out, "Message") {
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
		TR_print(TR_error_code { TR_throw "metin" } == "H2441")
		TR_print(TR_error_code { TR_throw TR_Message("H6903", -1) } == "H6903")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "H6903\nH6900\nnone\ntrue\ntrue\n"; out != want {
		t.Errorf("çıktı %q, %q bekleniyordu", out, want)
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

// Bir iletinin metni, metnin istendiği dilde kurulur: İngilizce dosyada İngilizce katalog ve
// İngilizce adlar.
func TestMessageTextLanguage(t *testing.T) {
	m := &MessageVal{Code: messages.H6903, Args: []Value{&IntVal{Val: -1}}}
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Text(en); !strings.Contains(got, "cannot be negative, got -1") {
		t.Errorf("en: %q", got)
	}
	if got := m.Text(codes.Source()); !strings.Contains(got, "negatif olamaz, -1 verildi") {
		t.Errorf("tr: %q", got)
	}
}

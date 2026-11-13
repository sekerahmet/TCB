package process_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ortam_değişkeni değeri ya da yok verir; gizli adlı değişkenin (sayı olmayan) değeri süzgece girer.
func TestEnvVar(t *testing.T) {
	t.Setenv("TR_TEST_DIR", "D:/veri")
	t.Setenv("TR_TEST_HF_TOKEN", "hf_ortamgizli99")
	t.Setenv("TR_TEST_MAX_TOKENS", "4096000")
	t.Setenv("TR_TEST_TOKENIZER", "D:/veri/tokenizer.json")
	got, m, err := commandLine(t, `[TR_env("TR_TEST_DIR"), TR_env("TR_TEST_NO_SUCH") ?? "varsayılan",
  TR_env("TR_TEST_HF_TOKEN"), TR_env("TR_TEST_MAX_TOKENS"), TR_env("TR_TEST_TOKENIZER")]`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `["D:/veri", "varsayılan", "hf_ortamgizli99", "4096000", "D:/veri/tokenizer.json"]`; got != want {
		t.Errorf("değerler %s, beklenen %s", got, want)
	}
	if s := m.Mask("hf_ortamgizli99 4096000 D:/veri/tokenizer.json"); s != interp.SecretMask+" 4096000 D:/veri/tokenizer.json" {
		t.Errorf("maskeleme: %q", s)
	}
	for _, d := range []struct{ code, want string }{
		{`TR_env(5)`, "1. argüman olarak Metin bekler"},
		{`TR_env()`, "1 argüman bekler"},
	} {
		if _, _, err := commandLine(t, d.code); err == nil || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.code, d.want, err)
		}
	}
	for _, code := range []string{`TR_env("")`, `TR_env("A=B")`} {
		if _, _, err := commandLine(t, code); !messages.Has(err, messages.H7231) {
			t.Errorf("%s: %s bekleniyordu: %v", code, messages.H7231, err)
		}
	}
}

// Ortamın her değerindeki adresin kimlik kısmı, adı ne olursa olsun, program başlarken süzgece girer
// (19.4): program değişkeni okumasa da; gizli adlı değişkenin değeri de taranır. Okunan değer gerçektir.
func TestEnvironmentAddressIsMaskedAtStart(t *testing.T) {
	t.Setenv("TR_TEST_PROXY", "http://ali:Vekil2026@vekil:8080")
	t.Setenv("TR_TEST_DB_PASSWORD", "postgres://u:Parola2026@db")
	_, m, err := commandLine(t, `1`)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Mask("Vekil2026 Parola2026"); got != interp.SecretMask+" "+interp.SecretMask {
		t.Errorf("okunmayan değişkenin parolası örtülmeli: %q", got)
	}
	got, m, err := commandLine(t, `TR_env("TR_TEST_PROXY")`)
	if err != nil || got != "http://ali:Vekil2026@vekil:8080" {
		t.Errorf("değer gerçek olmalı: %s (%v)", got, err)
	}
	if s := m.Mask(got); s != "http://ali:***@vekil:8080" {
		t.Errorf("maske: %q", s)
	}
}

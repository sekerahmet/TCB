package process_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ortam_değişkeni değeri ya da yok verir; gizli adlı değişkenin (sayı olmayan) değeri süzgece girer.
func TestEnvVar(t *testing.T) {
	t.Setenv("TR_DENEME_KLASOR", "D:/veri")
	t.Setenv("TR_DENEME_HF_TOKEN", "hf_ortamgizli99")
	t.Setenv("TR_DENEME_MAX_TOKENS", "4096000")
	t.Setenv("TR_DENEME_TOKENIZER", "D:/veri/tokenizer.json")
	got, err, m := commandLine(t, `[ortam_değişkeni("TR_DENEME_KLASOR"), ortam_değişkeni("TR_DENEME_YOK_BOYLE") ?? "varsayılan",
  ortam_değişkeni("TR_DENEME_HF_TOKEN"), ortam_değişkeni("TR_DENEME_MAX_TOKENS"), ortam_değişkeni("TR_DENEME_TOKENIZER")]`)
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
		{`ortam_değişkeni(5)`, "1. argüman olarak Metin bekler"},
		{`ortam_değişkeni()`, "1 argüman bekler"},
	} {
		if _, err, _ := commandLine(t, d.code); err == nil || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.code, d.want, err)
		}
	}
	for _, code := range []string{`ortam_değişkeni("")`, `ortam_değişkeni("A=B")`} {
		if _, err, _ := commandLine(t, code); !messages.Has(err, messages.H7231) {
			t.Errorf("%s: %s bekleniyordu: %v", code, messages.H7231, err)
		}
	}
}

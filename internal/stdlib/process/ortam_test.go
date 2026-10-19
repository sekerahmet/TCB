package process_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// ortam_değişkeni değeri ya da yok verir; gizli adlı değişkenin (sayı olmayan) değeri süzgece girer.
func TestOrtamDegiskeni(t *testing.T) {
	t.Setenv("TR_DENEME_KLASOR", "D:/veri")
	t.Setenv("TR_DENEME_HF_TOKEN", "hf_ortamgizli99")
	t.Setenv("TR_DENEME_MAX_TOKENS", "4096000")
	t.Setenv("TR_DENEME_TOKENIZER", "D:/veri/tokenizer.json")
	got, err, m := komutSatiri(t, `[ortam_değişkeni("TR_DENEME_KLASOR"), ortam_değişkeni("TR_DENEME_YOK_BOYLE") ?? "varsayılan",
  ortam_değişkeni("TR_DENEME_HF_TOKEN"), ortam_değişkeni("TR_DENEME_MAX_TOKENS"), ortam_değişkeni("TR_DENEME_TOKENIZER")]`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `["D:/veri", "varsayılan", "hf_ortamgizli99", "4096000", "D:/veri/tokenizer.json"]`; got != want {
		t.Errorf("değerler %s, beklenen %s", got, want)
	}
	if s := m.Maskele("hf_ortamgizli99 4096000 D:/veri/tokenizer.json"); s != interp.GizliMaske+" 4096000 D:/veri/tokenizer.json" {
		t.Errorf("maskeleme: %q", s)
	}
	for _, d := range []struct{ kod, want string }{
		{`ortam_değişkeni(5)`, "1. argüman olarak Metin bekler"},
		{`ortam_değişkeni("")`, "geçerli bir ortam değişkeni adı değil"},
		{`ortam_değişkeni("A=B")`, "'=' içeremez"},
		{`ortam_değişkeni()`, "1 argüman bekler"},
	} {
		if _, err, _ := komutSatiri(t, d.kod); err == nil || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.kod, d.want, err)
		}
	}
}

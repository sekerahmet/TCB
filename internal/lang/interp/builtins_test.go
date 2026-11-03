package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (10.5): TR_wait_all() argümansız ya da bir görev listesiyle çağrılır. Görev olmayan öğe H2919,
// birden çok argüman H2012; argüman sessizce yok sayılmaz (eskiden liste kabul edilip TR_true dönüyordu).
func TestWaitAllTakesNoneOrATaskList(t *testing.T) {
	for source, code := range map[string]messages.Code{"TR_print(TR_wait_all([1, 2]))": messages.H2919, "TR_wait_all(1, 2)": messages.H2012} {
		if _, err := runTR(t, source+"\n"); !messages.Has(err, code) {
			t.Errorf("%s: %s beklenirdi, gelen %v", source, code, err)
		}
	}
	if _, err := runTR(t, "TR_wait_all()\n"); err != nil {
		t.Errorf("TR_wait_all(): %v", err)
	}
}

// Kural (13.1): TR_read_file geçerli UTF-8 olmayan dosyada hatadır (H2621); ileti çağrının kendi
// yoluyla TR_read_bytes'ı önerir ve öneri kopyalanınca aynı dosyanın baytlarını okur. İki ayrı yol
// (biri klasörde) önerinin sabit bir ad değil programın yolu olduğunu sınar. .gz'de açılmış içerik
// geçersizse öneri yoktur (H2622): TR_read_bytes sıkıştırılmış baytları verir.
func TestReadFileInvalidUTF8SuggestsReadBytesWithItsPath(t *testing.T) {
	files := map[string]string{"ikili.bin": "\x00\xff\x80", "veri/ses.raw": "ab\xc3"}
	for path, want := range map[string]string{"ikili.bin": "Bytes([0, 255, 128])\n", "veri/ses.raw": "Bytes([97, 98, 195])\n"} {
		_, err := runCodedErr(t, "TR_var yol = \""+path+"\"\nTR_print(TR_read_file(yol))\n", files)
		if !messages.Has(err, messages.H2621) {
			t.Fatalf("%s: H2621 beklenirdi, gelen %v", path, err)
		}
		suggestion := "TR_read_bytes(\"" + path + "\")"
		for _, lang := range []string{"tr", "en"} {
			if text := messages.ErrorIn(err, lang); !strings.Contains(text, "`"+suggestion+"`") {
				t.Errorf("%s (%s): ileti %s önerisini taşımıyor: %q", path, lang, suggestion, text)
			}
		}
		if out := runCoded(t, "TR_print("+suggestion+")\n", files); out != want {
			t.Errorf("%s: önerinin çıktısı %q, beklenen %q", path, out, want)
		}
	}
	_, err := runCodedErr(t, "TR_write_file(\"b.gz\", TR_Bytes([255]))\nTR_print(TR_read_file(\"b.gz\"))\n", nil)
	if !messages.Has(err, messages.H2622) {
		t.Fatalf(".gz: H2622 beklenirdi, gelen %v", err)
	}
	if text := messages.ErrorIn(err, "en"); strings.Contains(text, "TR_read_bytes(") {
		t.Errorf(".gz: ileti TR_read_bytes çağrısını önermemeli: %q", text)
	}
}

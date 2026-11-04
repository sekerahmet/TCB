package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Değiştiren işler göreli yolu yalnız çalışma klasörüne göre çözer (13.1a, 18.16): program başka
// bir klasörden çalışırken programın yanındaki aynı adlı dosyayı silmez, taşımaz, üstüne yazmaz,
// klasörünün içine kopyalamaz. Silinecek ya da taşınacak dosya yalnız orada varsa ileti onu ve
// çalışma klasöründen ona giden açık yolu verir (H2630); o yol kopyalanınca çalışır.
func TestWritersDoNotTouchTheProgramsFile(t *testing.T) {
	root := t.TempDir()
	work, program := filepath.Join(root, "cwd"), filepath.Join(root, "program")
	for _, name := range []string{"sil.txt", "tasi.txt", "hedef/a.txt"} {
		testkit.WritePath(t, filepath.Join(program, filepath.FromSlash(name)), "eski")
	}
	testkit.WritePath(t, filepath.Join(work, "k.txt"), "kopya")
	t.Chdir(work)
	unchanged := func(name string) {
		t.Helper()
		if b, err := os.ReadFile(filepath.Join(program, filepath.FromSlash(name))); err != nil || string(b) != "eski" {
			t.Errorf("programın %s dosyası değişmemeliydi: %q %v", name, b, err)
		}
	}
	for _, c := range []struct{ code, name string }{
		{`TR_remove("sil.txt")`, "sil.txt"},
		{`TR_move("tasi.txt", "yeni.txt")`, "tasi.txt"},
	} {
		r := runTR(t, program, c.code)
		if !messages.Has(r.failure, messages.H2630) {
			t.Fatalf("%s: H2630 bekleniyordu: %v", c.code, r.failure)
		}
		if want := `"../program/` + c.name + `"`; !strings.Contains(r.failure.Error(), "`"+want+"`") {
			t.Errorf("%s: ileti açık yolu (%s) vermeli: %v", c.code, want, r.failure)
		}
		unchanged(c.name)
	}
	// Önerilen açık yol çalışır: programın dosyası ancak böyle silinir
	if r := runTR(t, program, `TR_print(TR_remove("../program/sil.txt"))`); r.failure != nil || r.output != "1\n" {
		t.Errorf("açık yolla silme: %q %v", r.output, r.failure)
	}
	if _, err := os.Stat(filepath.Join(program, "sil.txt")); !os.IsNotExist(err) {
		t.Errorf("açık yolla silinmeliydi: %v", err)
	}
	// Hedef ve yazma: programın hedef/ klasörüne değil, çalışma klasörüne
	for _, code := range []string{
		`TR_copy("k.txt", "hedef/")`,
		`TR_write_file("hedef/a.txt", "yeni")`,
		`TR_save_tensors("hedef/w.trt", {"w": TR_Tensor([1.0])})`,
	} {
		if r := runTR(t, program, code); r.failure == nil {
			t.Errorf("%s: çalışma klasöründe hedef/ yokken hata bekleniyordu", code)
		}
	}
	unchanged("hedef/a.txt")
	for _, name := range []string{"k.txt", "w.trt"} {
		if _, err := os.Stat(filepath.Join(program, "hedef", name)); !os.IsNotExist(err) {
			t.Errorf("programın hedef/ klasörüne %s yazılmamalıydı: %v", name, err)
		}
	}
	if r := runTR(t, program, `TR_make_dir("hedef/alt")`); r.failure != nil {
		t.Fatal(r.failure)
	}
	if _, err := os.Stat(filepath.Join(work, "hedef", "alt")); err != nil {
		t.Errorf("klasör çalışma klasöründe oluşmalıydı: %v", err)
	}
	if _, err := os.Stat(filepath.Join(program, "hedef", "alt")); !os.IsNotExist(err) {
		t.Errorf("programın klasöründe klasör oluşmamalıydı: %v", err)
	}
	// TR_copy'nin kaynağı okunur: programın yanındaki dosya bulunur
	if r := runTR(t, program, `TR_copy("tasi.txt", "kopya.txt")`); r.failure != nil {
		t.Fatal(r.failure)
	}
	if b, _ := os.ReadFile(filepath.Join(work, "kopya.txt")); string(b) != "eski" {
		t.Errorf("TR_copy programın dosyasını çalışma klasörüne kopyalamalıydı: %q", b)
	}
}

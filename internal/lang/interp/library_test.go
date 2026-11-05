package interp

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// withLibrary f kitaplığını test boyunca bağlar.
func withLibrary(t *testing.T, f fstest.MapFS) {
	t.Helper()
	prev := libraryFiles
	SetLibrary(f)
	t.Cleanup(func() { SetLibrary(prev) })
}

// Kural (2.3a): modül çalışma anında da okunurken kapıdan geçer; çağıran önceden denetlemese de
// (CheckImportedModules) yerleşiği ezemez. Hiçbir satırı çalışmaz.
func TestAModuleLoadedAtRunTimeCannotReplaceABuiltin(t *testing.T) {
	module := source.Header + "\nTR_print(\"modül çalıştı\")\nTR_func TR_print(x: TR_Int) {\n}\n"
	out, err := runCodedErr(t, "TR_import \"m\"\nTR_print(1)\n", map[string]string{"m.tr": module})
	if !messages.Has(err, messages.H0004) || out != "" {
		t.Fatalf("H0004 bekleniyordu: %v\n%s", err, out)
	}
}

// Kural (11.6): kitaplığın dosyası kapıya takılmaz: metni kitaplıkla aynı dosya kendi kodlarını
// tanımlar; aynı adlı başka metin kitaplık değildir.
func TestTheLibraryLoadsItsOwnNames(t *testing.T) {
	text := source.Header + "\nTR_const TR_pi = 3\n"
	withLibrary(t, fstest.MapFS{"kat.tr": {Data: []byte(text)}})
	if got := runCoded(t, "TR_import \"tr/kat\" TR_as k\nTR_print(k.TR_pi)\n", nil); got != "3\n" {
		t.Errorf("kitaplığın adı: %q", got)
	}
	out, err := runCodedErr(t, "TR_import \"kat\"\n", map[string]string{"kat.tr": text + "TR_const TR_e = 2\n"})
	if !messages.Has(err, messages.H0004) {
		t.Errorf("kullanıcının dosyası kitaplık değildir: %v\n%s", err, out)
	}
}

// Güvenlik bulgusu O2: göreli 'aktar' çalışma klasöründe aynı adlı bir dosya varken
// başka klasördeki programın (ya da modülün) yanındaki dosyayı değil onu yüklüyordu. Kural
// 11.1: önce aktaran dosyanın klasörü, sonra programın klasörü; çalışma klasörüne bakılmaz.
func TestImportIgnoresWorkingDir(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "scratch")
	// Çalışma klasöründeki dosyalar geçerli TR dosyalarıdır: yüklenselerdi çalışırlardı (gerileme
	// "başlık yok" diye değil, yanlış dosyanın okunmasıyla görünür).
	testkit.WritePath(t, filepath.Join(execution, "yardimci.tr"), source.Header+"\nTR_var kimden = \"çalışma klasörü\"\n")
	testkit.WritePath(t, filepath.Join(execution, "ozel.tr"), source.Header+"\nTR_var ozel = \"çalışma klasörü\"\n")
	runInDir(t, execution)
	prog := filepath.Join(root, "program")
	out, err := runWithModules(t, prog, map[string]string{
		"yardimci.tr":     "TR_var kimden = \"programın yanı\"\n",
		"alt/modul.tr":    "TR_import \"yardimci\"\nTR_var alt_kimden = kimden\n",
		"alt/yardimci.tr": "TR_var kimden = \"alt modülün yanı\"\n",
		"ana.tr":          "TR_import \"yardimci\"\nTR_import \"alt/modul\" TR_as m\nTR_print(kimden)\nTR_print(m.alt_kimden)\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "programın yanı\nalt modülün yanı\n" {
		t.Fatalf("modüller aktaranın klasöründen yüklenmeli:\n%s", out)
	}
	// Yalnızca çalışma klasöründe olan modül bulunmaz; hata bakılan yerleri söyler. Aktarma dosyanın
	// başındadır (2.1), hata yakalanamaz: ayrı bir programla sınanır.
	_, err = runWithModules(t, prog, map[string]string{"ana.tr": "TR_import \"ozel\"\n"})
	if err == nil || !strings.Contains(err.Error(), "ozel.tr") || strings.Contains(err.Error(), execution) {
		t.Fatalf("çalışma klasöründeki modül yüklenmemeli, hata yalnızca programın klasörünü göstermeli: %v", err)
	}
}

// Kitaplık modülünün hatası "tr/..." adıyla ve kendi satırıyla yazılır; göreli dosya yolları
// kitaplığın (diskte olmayan) klasörüne göre aranmaz.
func TestLibraryModuleError(t *testing.T) {
	withLibrary(t, fstest.MapFS{"m.tr": {Data: []byte(source.Header + "\nTR_func böl(a: TR_Int) -> TR_Float {\n    TR_return a / 0\n}\n")}})
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{"ana.tr": "TR_import \"tr/m\"\nböl(1)\n"})
	if err == nil || !strings.Contains(err.Error(), "tr/m.tr, Satır 3: Sıfıra bölme hatası") {
		t.Fatalf("hata kitaplık modülünün adı ve satırıyla gelmeli: %v", err)
	}
}

// Kural (6.1, 7.1): kitaplığın istisnası yalnız gömülü dosyanın kendi metnine verilir: aynı adlı ama
// başka metinli kullanıcı dosyası kitaplık dosyası değildir; test dosyası da kitaplığındır.
func TestLibraryFileNameIsOnlyTheEmbeddedText(t *testing.T) {
	withLibrary(t, fstest.MapFS{"layers.tr": {Data: []byte("a")}, "layers_test.tr": {Data: []byte("b")}})
	for _, c := range []struct{ path, text, want string }{
		{"lib/layers.tr", "a", "layers.tr"},
		{`C:\lib\layers.tr`, "a", "layers.tr"},
		{"lib/layers_test.tr", "b", "layers_test.tr"},
		{"proje/layers.tr", "a ", ""},
		{"lib/optimizers.tr", "a", ""},
	} {
		if got := LibraryFileName(c.path, c.text); got != c.want {
			t.Errorf("LibraryFileName(%q, %q) = %q, %q bekleniyordu", c.path, c.text, got, c.want)
		}
	}
}

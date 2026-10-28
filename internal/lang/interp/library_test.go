package interp

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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

// Güvenlik bulgusu O2: göreli 'aktar' çalışma klasöründe aynı adlı bir dosya varken
// başka klasördeki programın (ya da modülün) yanındaki dosyayı değil onu yüklüyordu. Kural
// 11.1: önce aktaran dosyanın klasörü, sonra programın klasörü; çalışma klasörüne bakılmaz.
func TestImportIgnoresWorkingDir(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "scratch")
	testkit.WritePath(t, filepath.Join(execution, "yardimci.tr"), "TR_var kimden = \"çalışma klasörü\"\n")
	testkit.WritePath(t, filepath.Join(execution, "ozel.tr"), "TR_var ozel = \"çalışma klasörü\"\n")
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
	withLibrary(t, fstest.MapFS{"m.tr": {Data: []byte(source.Header + "\nTR_func böl(a) {\n    TR_return a / 0\n}\n")}})
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{"ana.tr": "TR_import \"tr/m\"\nböl(1)\n"})
	if err == nil || !strings.Contains(err.Error(), "tr/m.tr, Satır 3: Sıfıra bölme hatası") {
		t.Fatalf("hata kitaplık modülünün adı ve satırıyla gelmeli: %v", err)
	}
}

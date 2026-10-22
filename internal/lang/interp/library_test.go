package interp

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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
	testkit.WritePath(t, filepath.Join(execution, "yardimci.tr"), "değişken kimden = \"çalışma klasörü\"\n")
	testkit.WritePath(t, filepath.Join(execution, "ozel.tr"), "değişken ozel = \"çalışma klasörü\"\n")
	runInDir(t, execution)
	prog := filepath.Join(root, "program")
	out, err := runWithModules(t, prog, map[string]string{
		"yardimci.tr":     "değişken kimden = \"programın yanı\"\n",
		"alt/modul.tr":    "aktar \"yardimci\"\ndeğişken alt_kimden = kimden\n",
		"alt/yardimci.tr": "değişken kimden = \"alt modülün yanı\"\n",
		"ana.tr": "aktar \"yardimci\"\nyazdır(kimden)\naktar \"alt/modul\" olarak m\nyazdır(m.alt_kimden)\n" +
			"dene { aktar \"ozel\" } yakala h { yazdır(h) }\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || lines[0] != "programın yanı" || lines[1] != "alt modülün yanı" {
		t.Fatalf("modüller aktaranın klasöründen yüklenmeli:\n%s", out)
	}
	// Yalnızca çalışma klasöründe olan modül bulunmaz; hata bakılan yerleri söyler.
	if !strings.Contains(lines[2], "bulunamadı") && !strings.Contains(lines[2], "yok") || strings.Contains(lines[2], execution) {
		t.Fatalf("çalışma klasöründeki modül yüklenmemeli, hata yalnızca programın klasörünü göstermeli:\n%s", lines[2])
	}
}

// Kural 11.6: "tr/" önekli yol dilin gömülü kitaplığındandır; diskteki tr/ klasörü onu
// gölgeleyemez, "./tr/" ise diskteki klasördür. Kitaplık modülünün içindeki göreli aktar
// kitaplıkta çözülür; olmayan modülün hatası kitaplıktakileri ve en yakın adı söyler.
func TestLibraryPrefix(t *testing.T) {
	withLibrary(t, fstest.MapFS{
		"katman.tr":       {Data: []byte("aktar \"ortak\"\ndeğişken kaynak = \"kitaplık\" + ek\n")},
		"ortak.tr":        {Data: []byte("değişken ek = \"!\"\n")},
		"iyilestirici.tr": {Data: []byte("değişken iy = 1\n")},
		"katman_test.tr":  {Data: []byte("test \"x\" { }\n")},
	})
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
		"tr/katman.tr": "değişken kaynak = \"disk\"\n",
		"ana.tr": "aktar \"tr/katman\" olarak k\nyazdır(k.kaynak)\naktar \"./tr/katman\" olarak d\nyazdır(d.kaynak)\n" +
			"dene { aktar \"tr/iyileştirici\" } yakala h { yazdır(h) }\n" +
			"dene { aktar \"tr/katman_test\" } yakala h { yazdır(h) }\n" +
			"dene { aktar \"tr/../ana\" } yakala h { yazdır(h) }\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 || lines[0] != "kitaplık!" || lines[1] != "disk" {
		t.Fatalf("tr/ kitaplıktan, ./tr/ diskten yüklenmeli:\n%s", out)
	}
	if !strings.Contains(lines[2], `"tr/iyileştirici" dilin kitaplığında yok`) || !strings.Contains(lines[2], `Bunu mu demek istediniz: "tr/iyilestirici"?`) {
		t.Fatalf("en yakın kitaplık modülü önerilmeli:\n%s", lines[2])
	}
	for _, s := range lines[3:] {
		if !strings.Contains(s, "dilin kitaplığında yok; kitaplıktaki modüller: tr/iyilestirici, tr/katman, tr/ortak") {
			t.Fatalf("test dosyası ve kitaplık dışı yol modül değildir:\n%s", s)
		}
	}
}

// Kitaplık modülünün hatası "tr/..." adıyla ve kendi satırıyla yazılır; göreli dosya yolları
// kitaplığın (diskte olmayan) klasörüne göre aranmaz.
func TestLibraryModuleError(t *testing.T) {
	withLibrary(t, fstest.MapFS{"m.tr": {Data: []byte("fonksiyon böl(a) {\n    sonuç a / 0\n}\n")}})
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{"ana.tr": "aktar \"tr/m\"\nböl(1)\n"})
	if err == nil || !strings.Contains(err.Error(), "tr/m.tr, Satır 2: Sıfıra bölme hatası") {
		t.Fatalf("hata kitaplık modülünün adı ve satırıyla gelmeli: %v", err)
	}
}

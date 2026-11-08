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
// 11.1: yalnız aktaran dosyanın klasörü; çalışma klasörüne ve programın klasörüne bakılmaz.
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

// Kural (11.1): göreli aktarma yolu yalnız aktaran dosyanın klasörüne göre çözülür ve harf büyüklüğü
// diskteki adla her sistemde aynı olmalıdır. Harf farkında ileti diskteki adı (H2721), aktaranın
// klasöründe olmayıp programın klasöründe olan dosyada açık yolu (H2722) önerir; iki öneri de
// kopyalanınca yüklenir. Aynı dosya iki yazımla iki modül örneği olamaz.
func TestARelativeImportUsesOnlyTheImportersFolderAndTheDisksCase(t *testing.T) {
	counter := "TR_var sayaç = 0\nTR_func artır() -> TR_Int {\n    sayaç += 1\n    TR_return sayaç\n}\n"
	cases := []struct {
		files      map[string]string
		code       messages.Code
		suggestion string
		fixed      map[string]string
		want       string
	}{
		{map[string]string{"ortak.tr": counter, "ana.tr": "TR_import \"Ortak\" TR_as a\nTR_print(a.artır())\n"},
			messages.H2721, "TR_import \"ortak\"",
			map[string]string{"ana.tr": "TR_import \"ortak\" TR_as a\nTR_import \"ortak\" TR_as b\nTR_print(a.artır(), b.artır())\n"}, "1 2\n"},
		{map[string]string{"ortak.tr": counter, "ana.tr": "TR_import \"ORTAK.tr\" TR_as a\nTR_print(a.artır())\n"},
			messages.H2721, "TR_import \"ortak.tr\"", nil, ""},
		{map[string]string{"ortak.tr": counter, "alt/m.tr": "TR_import \"ortak\" TR_as o\nTR_func x() -> TR_Int {\n    TR_return o.artır()\n}\n",
			"ana.tr": "TR_import \"alt/m\" TR_as m\nTR_print(m.x())\n"},
			messages.H2722, "TR_import \"../ortak\"",
			map[string]string{"alt/m.tr": "TR_import \"../ortak\" TR_as o\nTR_func x() -> TR_Int {\n    TR_return o.artır()\n}\n"}, "1\n"},
		{map[string]string{"Alt/m.tr": "TR_func x() -> TR_Int {\n    TR_return 7\n}\n", "ana.tr": "TR_import \"alt/m\" TR_as m\nTR_print(m.x())\n"},
			messages.H2721, "TR_import \"Alt/m\"", nil, ""},
	}
	for _, tc := range cases {
		root := t.TempDir()
		runInDir(t, root)
		out, err := runWithModules(t, root, tc.files)
		if !messages.Has(err, tc.code) || !strings.Contains(err.Error(), tc.suggestion) || out != "" {
			t.Errorf("%q: %s ve %s önerisi bekleniyordu: %v\n%s", tc.files["ana.tr"], tc.code, tc.suggestion, err, out)
			continue
		}
		if tc.fixed == nil {
			continue
		}
		for name, content := range tc.files {
			if _, replaced := tc.fixed[name]; !replaced {
				tc.fixed[name] = content
			}
		}
		if out, err := runWithModules(t, root, tc.fixed); err != nil || out != tc.want {
			t.Errorf("önerilen yazım yüklenmeli: %v\n%q", err, out)
		}
	}
	// Harf büyüklüğüne duyarlı dosya sisteminde iki ayrı dosya varsa yazılan ad kendi dosyasını yükler.
	root := t.TempDir()
	runInDir(t, root)
	testkit.WritePath(t, filepath.Join(root, "ortak.tr"), source.Header+"\nTR_var kim = \"küçük\"\n")
	testkit.WritePath(t, filepath.Join(root, "Ortak.tr"), source.Header+"\nTR_var kim = \"büyük\"\n")
	if names, _ := filepath.Glob(filepath.Join(root, "*.tr")); len(names) == 2 {
		out, err := runWithModules(t, root, map[string]string{"ana.tr": "TR_import \"Ortak\" TR_as a\nTR_import \"ortak\" TR_as b\nTR_print(a.kim, b.kim)\n"})
		if err != nil || out != "büyük küçük\n" {
			t.Errorf("iki ayrı dosya kendi yazımıyla yüklenmeli: %v\n%q", err, out)
		}
	}
}

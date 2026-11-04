package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// runInDir testi geçici bir çalışma klasöründe çalıştırır (pakette paralel test yok).
func runInDir(t *testing.T, folder string) {
	t.Helper()
	prevVal, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(folder); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevVal) })
}

// Okumanın kuralı: önce çalışma klasörü, sonra programın klasörü; dosya hiçbir yerde yoksa
// klasörünün bulunduğu ilk yer (hata o klasördeki eksik dosyayı söyler).
func TestResolvePathSingleRule(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "scratch")
	program := filepath.Join(root, "program")
	testkit.WritePath(t, filepath.Join(execution, "ikisinde.txt"), "çalışma")
	testkit.WritePath(t, filepath.Join(program, "ikisinde.txt"), "program")
	testkit.WritePath(t, filepath.Join(program, "yalniz_programda.txt"), "program")
	testkit.WritePath(t, filepath.Join(program, "cikti", "bos.txt"), "")
	runInDir(t, execution)

	m := New()
	m.SourceFile = filepath.Join(program, "ana.tr")
	caseList := map[string]string{
		"ikisinde.txt":         "ikisinde.txt",
		"yalniz_programda.txt": filepath.Join(program, "yalniz_programda.txt"),
		"yeni.txt":             "yeni.txt",
		"cikti/rapor.txt":      filepath.Join(program, "cikti", "rapor.txt"),
		"olmayan/rapor.txt":    "olmayan/rapor.txt",
	}
	for filePath, wantSub := range caseList {
		if got, searched := m.ResolvePath(filePath); got != wantSub || len(searched) != 2 {
			t.Errorf("ResolvePath(%q) = %q %v, %q bekleniyordu", filePath, got, searched, wantSub)
		}
	}
	absolute := filepath.Join(program, "ikisinde.txt")
	if got, searched := m.ResolvePath(absolute); got != absolute || searched != nil {
		t.Errorf("mutlak yol olduğu gibi kalmalı: %q %v", got, searched)
	}
	// Dosyadan çalışmayan program yalnızca çalışma klasörüne bakar
	m.SourceFile = "<derlenmiş-uygulama>"
	if got, searched := m.ResolvePath("yalniz_programda.txt"); got != "yalniz_programda.txt" || len(searched) != 1 {
		t.Errorf("program klasörü olmadan: %q %v", got, searched)
	}
}

// Okuyan iş programın yanındaki dosyayı bulur, değiştiren iş göreli yolu yalnız çalışma klasörüne
// göre çözer (13.1a, 18.16): güncellenen sayaç çalışma klasörüne yazılır, programınki değişmez.
func TestFileBuiltinsReadAndWriteRules(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "scratch")
	program := filepath.Join(root, "program")
	testkit.WritePath(t, filepath.Join(program, "sayac.txt"), "1")
	if err := os.MkdirAll(execution, 0o755); err != nil {
		t.Fatal(err)
	}
	runInDir(t, execution)

	m := New()
	m.SourceFile = filepath.Join(program, "ana.tr")
	m.Out = &strings.Builder{}
	run := func(codeVal string) (Value, error) {
		t.Helper()
		prog := mustParseProgram(t, codeVal)
		return m.Run(prog)
	}
	// Programın yanındaki dosya okunur; yazma çalışma klasörüne gider
	if _, err := run(`TR_write_file("sayac.txt", TR_String(TR_Int(TR_read_file("sayac.txt")) + 1))`); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(program, "sayac.txt")); string(b) != "1" {
		t.Errorf("programın sayac.txt'si değişmemeliydi: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(execution, "sayac.txt")); string(b) != "2" {
		t.Errorf("sayac.txt çalışma klasörüne yazılmalıydı: %q", b)
	}
	// Yeni dosya çalışma klasöründe oluşur
	if _, err := run(`TR_write_file("yeni.txt", "x")`); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(execution, "yeni.txt")); err != nil {
		t.Errorf("yeni dosya çalışma klasöründe oluşmalıydı: %v", err)
	}
	_, err := run(`TR_read_file("yok.txt")`)
	if err == nil || !strings.Contains(err.Error(), `dosya_oku(): "yok.txt" okunamadı: böyle bir dosya ya da klasör yok`) ||
		!strings.Contains(err.Error(), "bakılan yerler: ") || strings.Count(err.Error(), "yok.txt") != 3 {
		t.Errorf("beklenmeyen hata: %v", err)
	}
	// Hatadan sonra VM durur; yeni program yeni VM'de.
	fresh := func(codeVal string) (Value, error) {
		t.Helper()
		m := New()
		m.SourceFile = filepath.Join(program, "ana.tr")
		return m.Run(mustParseProgram(t, codeVal))
	}
	// Programın yanında var olan bir klasöre göreli yazma da çalışma klasörüne göredir: orada klasör yoksa hata
	testkit.WritePath(t, filepath.Join(program, "cikti", "bos.txt"), "")
	if _, err := fresh(`TR_write_file("cikti/rapor.txt", "x")`); !messages.Has(err, messages.H2131) {
		t.Errorf("çalışma klasöründe cikti/ yokken H2131 bekleniyordu: %v", err)
	}
	if _, err := os.Stat(filepath.Join(program, "cikti", "rapor.txt")); err == nil {
		t.Error("programın cikti/ klasörüne yazılmamalıydı")
	}
}

func TestResolveWritePath(t *testing.T) {
	var v *VM // VM'siz de çalışır
	if got, searched := v.ResolveWritePath("a/b.txt"); got != "a/b.txt" || len(searched) != 1 || !filepath.IsAbs(searched[0]) {
		t.Errorf("göreli yol olduğu gibi, bakılan yer çalışma klasöründe olmalı: %q %v", got, searched)
	}
	abs := filepath.Join(t.TempDir(), "a.txt")
	for _, p := range []string{abs, ""} {
		if got, searched := v.ResolveWritePath(p); got != p || searched != nil {
			t.Errorf("%q olduğu gibi kalmalı: %q %v", p, got, searched)
		}
	}
}

// Modülün yazdığı göreli yol çalışma klasörüne düşer, modülün klasöründeki aynı adlı dosya
// değişmez (13.1a, 18.16; program başka bir klasörden çalışır). Okuma ise modülün dosyasını bulur.
func TestWritersResolveOnlyInTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "cwd")
	program := filepath.Join(root, "program")
	testkit.WritePath(t, filepath.Join(program, "modul", "ortak.txt"), "eski")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	runInDir(t, work)
	out, err := runWithModules(t, program, map[string]string{
		"modul/yazici.tr": "TR_func oku() -> TR_String { TR_return TR_read_file(\"ortak.txt\") }\nTR_func yaz() { TR_write_file(\"ortak.txt\", \"modülden yazıldı\") }\n",
		"ana.tr":          "TR_import \"modul/yazici\"\nTR_print(oku())\nyaz()\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "eski\n" {
		t.Errorf("okuma modülün dosyasını bulmalıydı: %q", out)
	}
	if b, _ := os.ReadFile(filepath.Join(program, "modul", "ortak.txt")); string(b) != "eski" {
		t.Errorf("modülün ortak.txt'si değişmemeliydi: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(work, "ortak.txt")); string(b) != "modülden yazıldı" {
		t.Errorf("yazma çalışma klasörüne düşmeliydi: %q", b)
	}
}

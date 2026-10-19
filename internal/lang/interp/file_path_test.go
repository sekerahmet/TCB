package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
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

func writeFile(t *testing.T, filePath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Göreli yolların tek kuralı: önce çalışma klasörü, sonra programın klasörü; yeni dosya
// klasörünün bulunduğu ilk yere (düz ad için çalışma klasörüne) yazılır.
func TestResolvePathSingleRule(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "scratch")
	program := filepath.Join(root, "program")
	writeFile(t, filepath.Join(execution, "ikisinde.txt"), "çalışma")
	writeFile(t, filepath.Join(program, "ikisinde.txt"), "program")
	writeFile(t, filepath.Join(program, "yalniz_programda.txt"), "program")
	writeFile(t, filepath.Join(program, "cikti", "bos.txt"), "")
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
			t.Errorf("YolCoz(%q) = %q %v, %q bekleniyordu", filePath, got, searched, wantSub)
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

func TestFileBuiltinsUseSingleRule(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "scratch")
	program := filepath.Join(root, "program")
	writeFile(t, filepath.Join(program, "sayac.txt"), "1")
	if err := os.MkdirAll(execution, 0o755); err != nil {
		t.Fatal(err)
	}
	runInDir(t, execution)

	m := New()
	m.SourceFile = filepath.Join(program, "ana.tr")
	m.Out = &strings.Builder{}
	run := func(codeVal string) (Value, error) {
		t.Helper()
		p := parser.New(lexer.New(codeVal))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("sözdizimi hatası: %v", p.Errors())
		}
		return m.Run(prog)
	}
	// Programın yanındaki dosya okunur ve güncellenince yine oraya yazılır
	if _, err := run(`dosya_yaz("sayac.txt", metin(tam(dosya_oku("sayac.txt")) + 1))`); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(program, "sayac.txt")); string(b) != "2" {
		t.Errorf("sayac.txt programın klasöründe güncellenmeliydi: %q", b)
	}
	// Yeni dosya çalışma klasöründe oluşur
	if _, err := run(`dosya_yaz("yeni.txt", "x")`); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(execution, "yeni.txt")); err != nil {
		t.Errorf("yeni dosya çalışma klasöründe oluşmalıydı: %v", err)
	}
	_, err := run(`dosya_oku("yok.txt")`)
	if err == nil || !strings.Contains(err.Error(), `dosya_oku(): "yok.txt" okunamadı: böyle bir dosya ya da klasör yok`) ||
		!strings.Contains(err.Error(), "bakılan yerler: ") || strings.Count(err.Error(), "yok.txt") != 3 {
		t.Errorf("beklenmeyen hata: %v", err)
	}
}

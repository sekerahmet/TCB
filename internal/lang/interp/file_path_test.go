package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// klasordeCalis testi geçici bir çalışma klasöründe çalıştırır (pakette paralel test yok).
func klasordeCalis(t *testing.T, klasor string) {
	t.Helper()
	eski, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(klasor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(eski) })
}

func dosyaYaz(t *testing.T, yol, icerik string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(yol), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yol, []byte(icerik), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Göreli yolların tek kuralı: önce çalışma klasörü, sonra programın klasörü; yeni dosya
// klasörünün bulunduğu ilk yere (düz ad için çalışma klasörüne) yazılır.
func TestYolCozTekKural(t *testing.T) {
	kok := t.TempDir()
	calisma := filepath.Join(kok, "scratch")
	program := filepath.Join(kok, "program")
	dosyaYaz(t, filepath.Join(calisma, "ikisinde.txt"), "çalışma")
	dosyaYaz(t, filepath.Join(program, "ikisinde.txt"), "program")
	dosyaYaz(t, filepath.Join(program, "yalniz_programda.txt"), "program")
	dosyaYaz(t, filepath.Join(program, "cikti", "bos.txt"), "")
	klasordeCalis(t, calisma)

	m := New()
	m.SourceFile = filepath.Join(program, "ana.tr")
	durumlar := map[string]string{
		"ikisinde.txt":         "ikisinde.txt",
		"yalniz_programda.txt": filepath.Join(program, "yalniz_programda.txt"),
		"yeni.txt":             "yeni.txt",
		"cikti/rapor.txt":      filepath.Join(program, "cikti", "rapor.txt"),
		"olmayan/rapor.txt":    "olmayan/rapor.txt",
	}
	for yol, ister := range durumlar {
		if got, bakilanlar := m.ResolvePath(yol); got != ister || len(bakilanlar) != 2 {
			t.Errorf("YolCoz(%q) = %q %v, %q bekleniyordu", yol, got, bakilanlar, ister)
		}
	}
	mutlak := filepath.Join(program, "ikisinde.txt")
	if got, bakilanlar := m.ResolvePath(mutlak); got != mutlak || bakilanlar != nil {
		t.Errorf("mutlak yol olduğu gibi kalmalı: %q %v", got, bakilanlar)
	}
	// Dosyadan çalışmayan program yalnızca çalışma klasörüne bakar
	m.SourceFile = "<derlenmiş-uygulama>"
	if got, bakilanlar := m.ResolvePath("yalniz_programda.txt"); got != "yalniz_programda.txt" || len(bakilanlar) != 1 {
		t.Errorf("program klasörü olmadan: %q %v", got, bakilanlar)
	}
}

func TestDosyaYerlesikleriTekKuraliKullanir(t *testing.T) {
	kok := t.TempDir()
	calisma := filepath.Join(kok, "scratch")
	program := filepath.Join(kok, "program")
	dosyaYaz(t, filepath.Join(program, "sayac.txt"), "1")
	if err := os.MkdirAll(calisma, 0o755); err != nil {
		t.Fatal(err)
	}
	klasordeCalis(t, calisma)

	m := New()
	m.SourceFile = filepath.Join(program, "ana.tr")
	m.Out = &strings.Builder{}
	calistir := func(kod string) (Value, error) {
		t.Helper()
		p := parser.New(lexer.New(kod))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("sözdizimi hatası: %v", p.Errors())
		}
		return m.Run(prog)
	}
	// Programın yanındaki dosya okunur ve güncellenince yine oraya yazılır
	if _, err := calistir(`dosya_yaz("sayac.txt", metin(tam(dosya_oku("sayac.txt")) + 1))`); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(program, "sayac.txt")); string(b) != "2" {
		t.Errorf("sayac.txt programın klasöründe güncellenmeliydi: %q", b)
	}
	// Yeni dosya çalışma klasöründe oluşur
	if _, err := calistir(`dosya_yaz("yeni.txt", "x")`); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(calisma, "yeni.txt")); err != nil {
		t.Errorf("yeni dosya çalışma klasöründe oluşmalıydı: %v", err)
	}
	_, err := calistir(`dosya_oku("yok.txt")`)
	if err == nil || !strings.Contains(err.Error(), `dosya_oku(): "yok.txt" okunamadı: böyle bir dosya ya da klasör yok`) ||
		!strings.Contains(err.Error(), "bakılan yerler: ") || strings.Count(err.Error(), "yok.txt") != 3 {
		t.Errorf("beklenmeyen hata: %v", err)
	}
}

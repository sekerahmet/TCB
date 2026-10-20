package testrunner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func writeFile(t *testing.T, dir, ad, content string) string {
	t.Helper()
	fpath := filepath.Join(dir, ad)
	if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return fpath
}

const exampleTests = `fonksiyon topla(a, b) { sonuç a + b }
değişken sayac = 0

test "toplama doğru" {
    doğrula(topla(2, 2) == 4)
}

test "bölme hatası" {
    değişken x = 10 / 0
}

test "eşitlik kalır" {
    eşit_mi(5, topla(2, 2))
}

test "yerel tanım sızmaz" {
    değişken gizli = 1
    doğrula(gizli == 1)
}

test "önceki testin yereli görünmez" {
    değişken görünür = doğru; dene { yazdır(gizli) } yakala h { görünür = yanlış }
    eğer görünür { fırlat "gizli görünür olmamalıydı" }; değişken gizli = 2 // sonra tanımlı: sızma çalışma anında sınanır (5.14)
}

test "mesajlı doğrulama" {
    doğrula(yanlış, "özel mesaj")
}
`

func TestRunFile(t *testing.T) {
	fpath := writeFile(t, t.TempDir(), "ornek_test.tr", exampleTests)
	var out bytes.Buffer
	summary := RunFile(fpath, &out)
	text := out.String()

	if summary.Total != 6 || summary.Passed != 3 || summary.Failed != 3 {
		t.Fatalf("özet yanlış: %+v\n%s", summary, text)
	}
	expected := []string{
		"✓ toplama doğru",
		"✗ bölme hatası — Satır 9",
		"Sıfıra bölme",
		"✗ eşitlik kalır — Satır 13: eşitlik başarısız: beklenen 5 (Tam), gerçek 4 (Tam)",
		" 13 |     eşit_mi(5, topla(2, 2))", // kod çerçevesi
		"✓ yerel tanım sızmaz",
		"✓ önceki testin yereli görünmez",
		"✗ mesajlı doğrulama — Satır 27: doğrulama başarısız: özel mesaj",
	}
	for _, b := range expected {
		if !strings.Contains(text, b) {
			t.Errorf("çıktıda %q yok:\n%s", b, text)
		}
	}
	if summary.OK() {
		t.Error("kalan test varken Basarili() doğru döndü")
	}
}

func TestSummaryLine(t *testing.T) {
	o := Summary{Total: 12, Passed: 11, Failed: 1}
	if o.String() != "12 test, 11 geçti, 1 kaldı" {
		t.Fatalf("özet satırı: %q", o.String())
	}
}

func TestDirScan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a_test.tr", `test "a" { doğrula(doğru) }`)
	writeFile(t, root, "alt/b_test.tr", `test "b" { eşit_mi("x", "x") }`)
	writeFile(t, root, "yardimci.tr", `test "çalışmamalı" { doğrula(yanlış) }`)
	writeFile(t, root, ".gizli/c_test.tr", `test "gizli" { doğrula(yanlış) }`)

	var out bytes.Buffer
	summary, err := Run(root, &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Passed != 2 || summary.File != 2 || !summary.OK() {
		t.Fatalf("özet yanlış: %+v\n%s", summary, out.String())
	}
	if !strings.Contains(out.String(), "✅ 2 test, 2 geçti, 0 kaldı") {
		t.Errorf("özet satırı yok:\n%s", out.String())
	}
}

func TestEmptyDirErrors(t *testing.T) {
	if _, err := Run(t.TempDir(), &bytes.Buffer{}); !messages.Has(err, messages.H8503) {
		t.Fatalf("test dosyası olmayan klasör H8503 vermeli: %v", err)
	}
	if _, err := Run(filepath.Join(t.TempDir(), "yok"), &bytes.Buffer{}); !messages.Has(err, messages.H8502) {
		t.Fatalf("olmayan hedef H8502 vermeli: %v", err)
	}
}

func TestUnloadableFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "bozuk_test.tr", "test \"a\" { doğrula(doğru) }\ndeğişken = 5\n")
	writeFile(t, root, "ustduzey_test.tr", "fırlat \"yükleme hatası\"\ntest \"b\" { doğrula(doğru) }\n")
	writeFile(t, root, "iyi_test.tr", "test \"c\" { doğrula(doğru) }\n")
	var out bytes.Buffer
	summary, err := Run(root, &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Unloadable != 2 || summary.Passed != 1 || summary.OK() {
		t.Fatalf("özet yanlış: %+v\n%s", summary, out.String())
	}
	if !strings.Contains(out.String(), "2 dosya yüklenemedi") {
		t.Errorf("yükleme hatası özetlenmedi:\n%s", out.String())
	}
}

// Test içindeki 'ertele' test bitince çalışmalı; testin kendisi kalsa bile.
func TestDeferAndOutput(t *testing.T) {
	fpath := writeFile(t, t.TempDir(), "e_test.tr", `test "ertele" {
    ertele yazdır("temizlik")
    yazdır("gövde")
    doğrula(yanlış)
}
`)
	var out bytes.Buffer
	RunFile(fpath, &out)
	m := out.String()
	g, tz := strings.Index(m, "gövde"), strings.Index(m, "temizlik")
	if g < 0 || tz < 0 || tz < g {
		t.Fatalf("ertele sırası yanlış:\n%s", m)
	}
}

// Testin başlattığı eşzamanlı görevin hatası raporda görünür (standart hataya kaçmaz).
func TestTaskErrorShownInReport(t *testing.T) {
	fpath := writeFile(t, t.TempDir(), "g_test.tr", `test "görev" {
    eşzamanlı { değişken x = 1 / 0 }
}
`)
	var out bytes.Buffer
	RunFile(fpath, &out)
	if !strings.Contains(out.String(), "Eşzamanlı görev hatası") || !strings.Contains(out.String(), "Sıfıra bölme") {
		t.Fatalf("görev hatası raporda yok:\n%s", out.String())
	}
}

// Testin 'ertele' blokları, programın üst düzeyinde olduğu gibi, testin başlattığı görevler
// bittikten sonra çalışır.
func TestDeferRunsAfterTasks(t *testing.T) {
	f := writeFile(t, t.TempDir(), "ertele_test.tr", `test "ertele görevden sonra" {
    değişken l = [1]
    ertele { doğrula(l.uzunluk == 2, "görev erteleden önce bitmeli") }
    eşzamanlı {
        bekle(0.05)
        l.ekle(2)
    }
}
`)
	var b bytes.Buffer
	if s := RunFile(f, &b); s.Passed != 1 || s.Failed != 0 {
		t.Fatalf("test geçmeliydi: %+v\n%s", s, b.String())
	}
}

// Kullanıcı iletisinde geçen "Satır " sözcüğü konum sayılmaz: konumsuz hataya testin satırı eklenir.
func TestUserMessageWithSatirGetsLine(t *testing.T) {
	f := writeFile(t, t.TempDir(), "satır_test.tr", "test \"satır kelimesi\" {\n    fırlat \"Satır sayısı yanlış\"\n}\n")
	var b bytes.Buffer
	RunFile(f, &b)
	if out := b.String(); !strings.Contains(out, "Satır 1: Satır sayısı yanlış") || !strings.Contains(out, `1 | test "satır kelimesi" {`) {
		t.Fatalf("testin satırı eklenmeli ve çerçevesi çizilmeli:\n%s", out)
	}
}

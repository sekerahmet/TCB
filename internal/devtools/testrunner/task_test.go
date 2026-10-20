package testrunner

import (
	"bytes"
	"strings"
	"testing"
)

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

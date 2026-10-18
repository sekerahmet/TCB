package testkosucu

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dosyaYaz(t *testing.T, klasor, ad, icerik string) string {
	t.Helper()
	yol := filepath.Join(klasor, ad)
	if err := os.MkdirAll(filepath.Dir(yol), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yol, []byte(icerik), 0644); err != nil {
		t.Fatal(err)
	}
	return yol
}

const ornekTestler = `fonksiyon topla(a, b) { sonuç a + b }
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
    eğer görünür { fırlat "gizli görünür olmamalıydı" }
}

test "mesajlı doğrulama" {
    doğrula(yanlış, "özel mesaj")
}
`

func TestDosyaCalistir(t *testing.T) {
	yol := dosyaYaz(t, t.TempDir(), "ornek_test.tr", ornekTestler)
	var cikti bytes.Buffer
	ozet := DosyaCalistir(yol, &cikti)
	metin := cikti.String()

	if ozet.Toplam != 6 || ozet.Gecen != 3 || ozet.Kalan != 3 {
		t.Fatalf("özet yanlış: %+v\n%s", ozet, metin)
	}
	beklenenler := []string{
		"✓ toplama doğru",
		"✗ bölme hatası — Satır 9",
		"Sıfıra bölme",
		"✗ eşitlik kalır — Satır 13: eşitlik başarısız: beklenen 5 (Tam), gerçek 4 (Tam)",
		" 13 |     eşit_mi(5, topla(2, 2))", // kod çerçevesi
		"✓ yerel tanım sızmaz",
		"✓ önceki testin yereli görünmez",
		"✗ mesajlı doğrulama — Satır 27: doğrulama başarısız: özel mesaj",
	}
	for _, b := range beklenenler {
		if !strings.Contains(metin, b) {
			t.Errorf("çıktıda %q yok:\n%s", b, metin)
		}
	}
	if ozet.Basarili() {
		t.Error("kalan test varken Basarili() doğru döndü")
	}
}

func TestOzetSatiri(t *testing.T) {
	o := Ozet{Toplam: 12, Gecen: 11, Kalan: 1}
	if o.String() != "12 test, 11 geçti, 1 kaldı" {
		t.Fatalf("özet satırı: %q", o.String())
	}
}

func TestKlasorTarama(t *testing.T) {
	kok := t.TempDir()
	dosyaYaz(t, kok, "a_test.tr", `test "a" { doğrula(doğru) }`)
	dosyaYaz(t, kok, "alt/b_test.tr", `test "b" { eşit_mi("x", "x") }`)
	dosyaYaz(t, kok, "yardimci.tr", `test "çalışmamalı" { doğrula(yanlış) }`)
	dosyaYaz(t, kok, ".gizli/c_test.tr", `test "gizli" { doğrula(yanlış) }`)

	var cikti bytes.Buffer
	ozet, err := Calistir(kok, &cikti)
	if err != nil {
		t.Fatal(err)
	}
	if ozet.Toplam != 2 || ozet.Gecen != 2 || ozet.Dosya != 2 || !ozet.Basarili() {
		t.Fatalf("özet yanlış: %+v\n%s", ozet, cikti.String())
	}
	if !strings.Contains(cikti.String(), "✅ 2 test, 2 geçti, 0 kaldı") {
		t.Errorf("özet satırı yok:\n%s", cikti.String())
	}
}

func TestBosKlasorHataVerir(t *testing.T) {
	if _, err := Calistir(t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Fatal("test dosyası olmayan klasör hata vermedi")
	}
}

func TestYuklenemeyenDosya(t *testing.T) {
	kok := t.TempDir()
	dosyaYaz(t, kok, "bozuk_test.tr", "test \"a\" { doğrula(doğru) }\ndeğişken = 5\n")
	dosyaYaz(t, kok, "ustduzey_test.tr", "fırlat \"yükleme hatası\"\ntest \"b\" { doğrula(doğru) }\n")
	dosyaYaz(t, kok, "iyi_test.tr", "test \"c\" { doğrula(doğru) }\n")
	var cikti bytes.Buffer
	ozet, err := Calistir(kok, &cikti)
	if err != nil {
		t.Fatal(err)
	}
	if ozet.Yuklenemez != 2 || ozet.Gecen != 1 || ozet.Basarili() {
		t.Fatalf("özet yanlış: %+v\n%s", ozet, cikti.String())
	}
	if !strings.Contains(cikti.String(), "2 dosya yüklenemedi") {
		t.Errorf("yükleme hatası özetlenmedi:\n%s", cikti.String())
	}
}

// Test içindeki 'ertele' test bitince çalışmalı; testin kendisi kalsa bile.
func TestErteleVeCikti(t *testing.T) {
	yol := dosyaYaz(t, t.TempDir(), "e_test.tr", `test "ertele" {
    ertele yazdır("temizlik")
    yazdır("gövde")
    doğrula(yanlış)
}
`)
	var cikti bytes.Buffer
	DosyaCalistir(yol, &cikti)
	m := cikti.String()
	g, tz := strings.Index(m, "gövde"), strings.Index(m, "temizlik")
	if g < 0 || tz < 0 || tz < g {
		t.Fatalf("ertele sırası yanlış:\n%s", m)
	}
}

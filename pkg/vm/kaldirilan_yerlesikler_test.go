package vm

import (
	"strings"
	"testing"
	"time"

	"tr_dili/pkg/kod"
	"tr_dili/pkg/turkce"
)

// Kaldırılan eş adlar kayıtlı değildir; kullanılırsa hata asıl adı önerir.
func TestKaldirilanYerlesikIpucu(t *testing.T) {
	for _, c := range []struct{ kod, want string }{
		{"uyut(1)", "'uyut' TR'de yok; 'bekle' yazmak mı istediniz?"},
		{"yaz(1)", "'yaz' TR'de yok; 'yazdır' yazmak mı istediniz?"},
		{"değişken x = boy([1])", "'boy(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın"},
		{"tur(1)", "'tur' TR'de yok; 'tip' yazmak mı istediniz?"},
		{"en_büyük(1, 2)", "'en_büyük' TR'de yok; 'maks' yazmak mı istediniz?"},
		{"medyan([1, 2])", "'medyan' TR'de yok; 'ortanca' yazmak mı istediniz?"},
		{"[1].dönüştür { o }", "'dönüştür' TR'de yok; 'haritala' yazmak mı istediniz?"},
		{"\"abc\".boy", "'boy' TR'de yok; 'uzunluk' yazmak mı istediniz?"},
		// ASCII biçimler kabul edilmez; Türkçe ad önerilir (ASCII biçim asla önerilmez).
		{"yazdir(1)", "'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?"},
		{"sirala([2, 1])", "'sirala(...)' TR'de bir fonksiyon değil; 'liste.sırala()' yazın"},
		{"donustur([1], 1)", "'donustur(...)' TR'de bir fonksiyon değil; 'liste.haritala(...)' yazın"},
		{"\"abc\".buyuk_harf()", "'buyuk_harf' TR'de yok; 'büyük_harf' yazmak mı istediniz?"},
		{"[1, 2].ters_cevir()", "'ters_cevir' TR'de yok; 'ters_çevir' yazmak mı istediniz?"},
		{"değişken x = dogru", "'dogru' TR'de anahtar kelime değil; 'doğru' yazmak mı istediniz?"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.kod)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.kod, c.want, err)
		}
	}
	m := New()
	for _, k := range kod.Kodlar() {
		g, _ := kod.Etkin().Girdisi(k)
		if !k.Yerlesik() {
			continue
		}
		for _, ad := range g.Eski {
			for _, y := range []string{ad, turkce.ASCII(ad)} {
				if _, ok := m.GlobalEnv.Get(y); ok {
					t.Errorf("%q (%s'in eski adı) kaldırıldı, kayıtlı olmamalı", y, k)
				}
				if KaldirilanYerlesikIpucu(y) == "" {
					t.Errorf("%q için ipucu bekleniyordu", y)
				}
			}
		}
	}
}

// Gölgelenmiş bir yerleşik çağrılırsa hata, değerin bir fonksiyon olmadığını ve yerleşiğin
// hangi satırdaki tanımla gölgelendiğini söyler.
func TestGolgelenmisYerlesikCagrisi(t *testing.T) {
	for _, c := range []struct{ kod, want string }{
		{"değişken tip = 5\ntip([1, 2])", "Satır 2: 'tip' bir fonksiyon değil (Tam); 1. satırda bir değişkene verildi, yerleşik 'tip' bu kapsamda gölgelendi"},
		{"fonksiyon f(toplam) {\n    sonuç toplam([1])\n}\nf(\"a\")", "Satır 2: 'toplam' bir fonksiyon değil (Metin); 1. satırda bir parametreye verildi, yerleşik 'toplam' bu kapsamda gölgelendi"},
		{"döngü metin içinde [1] {\n    metin(1)\n}", "'metin' bir fonksiyon değil (Tam); 1. satırda bir döngü değişkenine verildi"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.kod)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.kod, c.want, err)
		}
	}
	// Gölgelemeyen kapsamda yerleşik çalışır; fonksiyonla gölgelemek de çalışır.
	val, err := runTR(t, "fonksiyon f() { değişken uzunluk = 1\n sonuç uzunluk }\nfonksiyon toplam(x) { sonuç 7 }\nsonuç [f(), [1, 2].uzunluk, toplam([1])]")
	if err != nil || val.Inspect() != "[1, 2, 7]" {
		t.Errorf("[1, 2, 7] bekleniyordu: %v %v", val, err)
	}
}

// Kaldırılan adlar artık sıradan adlardır.
func TestKaldirilanYerlesikAdlariSerbest(t *testing.T) {
	val, err := runTR(t, `
		değişken boy = 180
		değişken tur = "kısa"
		fonksiyon yaz(x) { sonuç x * 2 }
		sonuç [boy, tur, yaz(4)]`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[180, "kısa", 8]`; val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

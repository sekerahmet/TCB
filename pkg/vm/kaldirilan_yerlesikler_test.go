package vm

import (
	"strings"
	"testing"
	"time"

	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/turkce"
)

// İkinci ad yalnızca Türkçe karaktersiz karşılık olabilir; eş anlamlı ad kayıtta panikler.
func TestRegisterBuiltinYalnizcaAsciiKarsilik(t *testing.T) {
	bos := func(*VM, []Value, map[string]Value) (Value, error) { return NULL, nil }
	for _, c := range []struct{ ad, karsilik string }{
		{"sırala", "sirala"}, {"öklid_uzaklığı", "oklid_uzakligi"}, {"tip", ""}, {"karekök", ""},
	} {
		New().RegisterBuiltin(c.ad, c.karsilik, bos)
	}
	for _, c := range []struct{ ad, karsilik string }{
		{"bekle", "uyut"},        // eş anlamlı
		{"maks", "en_büyük"},     // eş anlamlı
		{"maks", "maks"},         // ad zaten ASCII
		{"sırala", "sıralama"},   // ASCII biçimi değil
		{"karekök", "karekök_x"}, // ASCII biçimi değil
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterBuiltin(%q, %q) panik bekleniyordu", c.ad, c.karsilik)
				}
			}()
			New().RegisterBuiltin(c.ad, c.karsilik, bos)
		}()
	}
}

// Kaldırılan eş adlar kayıtlı değildir; kullanılırsa hata asıl adı önerir.
func TestKaldirilanYerlesikIpucu(t *testing.T) {
	for _, c := range []struct{ kod, want string }{
		{"uyut(1)", "'uyut' TR'de yok; 'bekle' yazmak mı istediniz?"},
		{"yaz(1)", "'yaz' TR'de yok; 'yazdır' yazmak mı istediniz?"},
		{"değişken x = boy([1])", "'boy' TR'de yok; 'uzunluk' yazmak mı istediniz?"},
		{"tur(1)", "'tur' TR'de yok; 'tip' yazmak mı istediniz?"},
		{"en_büyük(1, 2)", "'en_büyük' TR'de yok; 'maks' yazmak mı istediniz?"},
		{"medyan([1, 2])", "'medyan' TR'de yok; 'ortanca' yazmak mı istediniz?"},
		{"[1].dönüştür { o }", "'dönüştür' TR'de yok; 'haritala' yazmak mı istediniz?"},
		{"\"abc\".boy", "'boy' TR'de yok; 'uzunluk' yazmak mı istediniz?"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.kod)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.kod, c.want, err)
		}
	}
	m := New()
	for _, k := range sozluk.Kodlar() {
		g, _ := sozluk.Etkin().Girdisi(k)
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

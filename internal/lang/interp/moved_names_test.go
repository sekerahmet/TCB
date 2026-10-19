package interp

import (
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Kaldırılan eş adlar kayıtlı değildir; kullanılırsa hata asıl adı önerir.
func TestRemovedBuiltinHint(t *testing.T) {
	for _, c := range []struct{ codeVal, want string }{
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
		_, _, err := runTRWithin(t, 5*time.Second, c.codeVal)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.codeVal, c.want, err)
		}
	}
	m := New()
	for _, k := range codes.Codes() {
		g, _ := codes.Active().Entry(k)
		if !k.IsBuiltin() {
			continue
		}
		for _, name := range g.Old {
			for _, y := range []string{name, turkish.ASCII(name)} {
				if _, ok := m.GlobalEnv.Get(y); ok {
					t.Errorf("%q (%s'in eski adı) kaldırıldı, kayıtlı olmamalı", y, k)
				}
				if hint.BuiltinSpellingHint(y) == "" {
					t.Errorf("%q için ipucu bekleniyordu", y)
				}
			}
		}
	}
}

// Kaldırılan adlar artık sıradan adlardır.
func TestRemovedBuiltinNamesAreFree(t *testing.T) {
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

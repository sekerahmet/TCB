package interp

import (
	"strings"
	"testing"
	"time"
)

// Faz 2 — dilin imzası: 'o' zamiri ve Türkçe hata mesajları

func TestTrailingBlockWithOPronoun(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		değişken sayılar = [1, 2, 3, 4, 5, 6]
		değişken a = sayılar.filtrele { o > 2 }.haritala { o * o }
		değişken b = sayılar.filtrele { o % 2 == 0 }
		değişken c = [{"ad": "Ali"}, {"ad": "Ayşe"}].haritala { o["ad"] }
		değişken d = [[1, 2], [3, 4]].haritala { o.haritala { o * 10 } }
		fonksiyon uygula(f) { sonuç f(21) }
		değişken e = uygula { o * 2 }
		sonuç [a, b, c, d, e]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[[9, 16, 25, 36], [2, 4, 6], ["Ali", "Ayşe"], [[10, 20], [30, 40]], 42]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

func TestTrailingBlockEquivalentToLambda(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		değişken l = [5, 3, 8, 1]
		sonuç l.filtrele { o > 2 } == l.filtrele((x) => x > 2)`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Inspect() != "doğru" {
		t.Fatalf("sondaki blok ile ok fonksiyonu aynı sonucu vermeli, %s alındı", val.Inspect())
	}
}

// Kontrol yapısı başlıklarında '{' gövdenin başlangıcıdır; sondaki blok olarak okunmamalı.
func TestControlHeadersDoNotTakeTrailingBlocks(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		değişken sayılar = [1, 2, 3, 4, 5, 6]
		değişken izler = []
		eğer sayılar.uzunluk > 3 {
			izler.ekle("eğer")
		}
		döngü s içinde sayılar.filtrele((x) => x < 3) {
			izler.ekle("döngü-{s}")
		}
		değişken i = 0
		iken i < sayılar.uzunluk {
			i += 3
		}
		izler.ekle("iken-{i}")
		değişken sonuç_eşle = eşle sayılar.uzunluk {
			6 => "altı"
			varsayılan => "başka"
		}
		izler.ekle(sonuç_eşle)
		// Parantez içinde sondaki blok başlıkta da kullanılabilir
		eğer (sayılar.filtrele { o > 5 }).uzunluk == 1 {
			izler.ekle("parantez")
		}
		sonuç izler`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["eğer", "döngü-1", "döngü-2", "iken-6", "altı", "parantez"]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

func TestUnknownNameErrorHasColumnAndSuggestion(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{"değişken toplam = 1\nyazdır(toplm + 1)", "Satır 2, Sütun 8: 'toplm' adında bir değişken ya da fonksiyon yok. Bunu mu demek istediniz: 'toplam'?"},
		{"değişken değişken_adı = 1\ndegisken_adi = 2", "Bunu mu demek istediniz: 'değişken_adı'?"},
		{"değişken sayaç = 1\nsayac += 1", "Satır 2, Sütun 1: 'sayac' adında bir değişken yok. Bunu mu demek istediniz: 'sayaç'?"},
		{"yazdırr(1)", "Bunu mu demek istediniz: 'yazdır'?"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, tc.code)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", tc.want, err)
		}
	}
}

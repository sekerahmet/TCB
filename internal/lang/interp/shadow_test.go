package interp

import (
	"strings"
	"testing"
	"time"
)

// Gölgelenmiş bir yerleşik çağrılırsa hata, değerin bir fonksiyon olmadığını ve yerleşiğin
// hangi satırdaki tanımla gölgelendiğini söyler.
func TestShadowedBuiltinCall(t *testing.T) {
	for _, c := range []struct{ codeVal, want string }{
		{"değişken tip = 5\ntip([1, 2])", "Satır 2: 'tip' bir fonksiyon değil (Tam); 1. satırda bir değişkene verildi, yerleşik 'tip' bu kapsamda gölgelendi"},
		{"fonksiyon f(toplam) {\n    sonuç toplam([1])\n}\nf(\"a\")", "Satır 2: 'toplam' bir fonksiyon değil (Metin); 1. satırda bir parametreye verildi, yerleşik 'toplam' bu kapsamda gölgelendi"},
		{"döngü metin içinde [1] {\n    metin(1)\n}", "'metin' bir fonksiyon değil (Tam); 1. satırda bir döngü değişkenine verildi"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.codeVal)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.codeVal, c.want, err)
		}
	}
	// Gölgelemeyen kapsamda yerleşik çalışır; fonksiyonla gölgelemek de çalışır.
	val, err := runTR(t, "fonksiyon f() { değişken uzunluk = 1\n sonuç uzunluk }\nfonksiyon toplam(x) { sonuç 7 }\nsonuç [f(), [1, 2].uzunluk, toplam([1])]")
	if err != nil || val.Inspect() != "[1, 2, 7]" {
		t.Errorf("[1, 2, 7] bekleniyordu: %v %v", val, err)
	}
}

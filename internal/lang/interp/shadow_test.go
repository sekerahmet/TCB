package interp

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Gölgelenmiş bir yerleşik çağrılırsa hata, değerin bir fonksiyon olmadığını ve yerleşiğin
// hangi satırdaki tanımla gölgelendiğini söyler.
func TestShadowedBuiltinCall(t *testing.T) {
	for _, c := range []struct {
		codeVal string
		line    int
		message messages.Code // gölgeleyen tanımın türünü anlatan ileti
	}{
		{"değişken tip = 5\ntip([1, 2])", 2, messages.H2450},
		{"fonksiyon f(toplam) {\n    sonuç toplam([1])\n}\nf(\"a\")", 2, messages.H2451},
		{"döngü metin içinde [1] {\n    metin(1)\n}", 2, messages.H2452},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.codeVal)
		if !codeAt(err, c.message, c.line) {
			t.Errorf("%s: %s (satır %d) bekleniyordu, alınan: %v", c.codeVal, c.message, c.line, err)
		}
	}
	// Gölgelemeyen kapsamda yerleşik çalışır; fonksiyonla gölgelemek de çalışır.
	val, err := runTR(t, "fonksiyon f() { değişken uzunluk = 1\n sonuç uzunluk }\nfonksiyon toplam(x) { sonuç 7 }\nsonuç [f(), [1, 2].uzunluk, toplam([1])]")
	if err != nil || val.Inspect() != "[1, 2, 7]" {
		t.Errorf("[1, 2, 7] bekleniyordu: %v %v", val, err)
	}
}

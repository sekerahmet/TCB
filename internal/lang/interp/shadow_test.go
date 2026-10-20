package interp

import (
	"strings"
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
		what    string // gölgeleyen tanımın türü (iletinin parçası)
	}{
		{"değişken tip = 5\ntip([1, 2])", 2, "bir değişkene"},
		{"fonksiyon f(toplam) {\n    sonuç toplam([1])\n}\nf(\"a\")", 2, "bir parametreye"},
		{"döngü metin içinde [1] {\n    metin(1)\n}", 2, "bir döngü değişkenine"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.codeVal)
		if !codeAt(err, messages.H2449, c.line) || !strings.Contains(err.Error(), c.what) {
			t.Errorf("%s: %s (satır %d, %q) bekleniyordu, alınan: %v", c.codeVal, messages.H2449, c.line, c.what, err)
		}
	}
	// Gölgelemeyen kapsamda yerleşik çalışır; fonksiyonla gölgelemek de çalışır.
	val, err := runTR(t, "fonksiyon f() { değişken uzunluk = 1\n sonuç uzunluk }\nfonksiyon toplam(x) { sonuç 7 }\nsonuç [f(), [1, 2].uzunluk, toplam([1])]")
	if err != nil || val.Inspect() != "[1, 2, 7]" {
		t.Errorf("[1, 2, 7] bekleniyordu: %v %v", val, err)
	}
}

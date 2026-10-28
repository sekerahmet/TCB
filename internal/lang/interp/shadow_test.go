package interp

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kullanıcının adı yerleşiği gölgelemez (2.1): yerleşikle aynı yazılan ad kullanıcının adıdır,
// fonksiyon olmayan değeri çağırmak H2421'dir.
func TestShadowedBuiltinCall(t *testing.T) {
	for _, c := range []struct {
		codeVal string
		line    int
		message messages.Code // gölgeleyen tanımın türünü anlatan ileti
	}{
		{"TR_var tip = 5\ntip([1, 2])", 2, messages.H2421},
		{"TR_func f(toplam) {\n    TR_return toplam([1])\n}\nf(\"a\")", 2, messages.H2421},
		{"TR_for metin TR_in [1] {\n    metin(1)\n}", 2, messages.H2421},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, c.codeVal)
		if !codeAt(err, c.message, c.line) {
			t.Errorf("%s: %s (satır %d) bekleniyordu, alınan: %v", c.codeVal, c.message, c.line, err)
		}
	}
	// Gölgelemeyen kapsamda yerleşik çalışır; fonksiyonla gölgelemek de çalışır.
	val, err := runTR(t, "TR_func f() { TR_var uzunluk = 1\n TR_return uzunluk }\nTR_func toplam(x) { TR_return 7 }\nTR_var result = [f(), [1, 2].TR_length, toplam([1])]")
	if err != nil || val.Inspect() != "[1, 2, 7]" {
		t.Errorf("[1, 2, 7] bekleniyordu: %v %v", val, err)
	}
}

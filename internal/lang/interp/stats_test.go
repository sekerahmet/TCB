package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (B13): standart_sapma ve ortanca sayı olmayan değeri girdinin biçimiyle anlatır: tek liste
// argümanında "listenin N. elemanı" (H2486), argümanların kendisinde "N. argüman" (H2423).
// Önceden liste olmayan argüman da (metin, tensör) "listenin 1. elemanı" diyordu.
func TestStatsNameTheBadValueByItsPlace(t *testing.T) {
	for _, c := range []struct {
		code string
		want messages.Code
	}{
		{`standart_sapma("abc")`, messages.H2423},
		{`ortanca(1, "a")`, messages.H2423},
		{`standart_sapma([1, "a"])`, messages.H2486},
		{`ortanca(["a"])`, messages.H2486},
	} {
		_, err := New().Run(mustParseProgram(t, c.code+"\n"))
		if !messages.Has(err, c.want) {
			t.Errorf("%s: %v (beklenen %s)", c.code, err, c.want)
		}
	}
}

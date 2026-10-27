package interp

import (
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
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
		{`TR_std("abc")`, messages.H2423},
		{`TR_median(1, "a")`, messages.H2423},
		{`TR_std([1, "a"])`, messages.H2486},
		{`TR_median(["a"])`, messages.H2486},
	} {
		_, err := New().Run(mustParseProgram(t, c.code+"\n"))
		if !messages.Has(err, c.want) {
			t.Errorf("%s: %v (beklenen %s)", c.code, err, c.want)
		}
	}
}

// Kural: 1. argüman yanlışsa beklenen tipler imzanın 1. parametresidir (definitions.json:
// Liste, Tam ya da Ondalık); sonraki argümanlar sayıdır.
func TestStatsFirstArgumentExpectsTheSignature(t *testing.T) {
	for _, c := range []struct {
		code     string
		wantList bool
	}{{`TR_median("abc")`, true}, {`TR_std(TR_true)`, true}, {`TR_median(1, "a")`, false}} {
		_, err := New().Run(mustParseProgram(t, c.code+"\n"))
		m, ok := messages.As(err)
		if !ok || m.Code != messages.H2423 {
			t.Fatalf("%s: %v (beklenen H2423)", c.code, err)
		}
		types, _ := m.Args[2].(typeNames)
		if got := slices.Contains(types, codes.TypeList); got != c.wantList {
			t.Errorf("%s: beklenen tipler %v; Liste içermesi %v bekleniyordu", c.code, []codes.Code(types), c.wantList)
		}
	}
}

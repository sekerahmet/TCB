package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestMessageConsistency(t *testing.T) {
	cases := []struct {
		code string
		want messages.Code
	}{
		{"AdamW([tensör(3.0)])", messages.H6513},
		{"AdamW([3.0])", messages.H6512},
		{"gömme(tensör(3.0), [0])", messages.H6303},
		{"gömme(3.0, [0])", messages.H6302},
		{"geri_yayılım(3.0)", messages.H6030},
		{"geri_yayılım(tensör(3.0) * 2)", messages.H6402},
		{"çapraz_entropi(sıfırlar([3, 4]), [[0, 1, 0, 0], [1, 0, 0, 0], [0, 0, 0, 1]])", messages.H6312},
		{"log_softmax([1, 2], eksen: 0)", messages.H6104},
		{"relu([[1, 2]])", messages.H6104},
	}
	for _, d := range cases {
		_, err := run(t, d.code)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.code, err, d.want)
		}
		if err != nil && strings.Contains(err.Error(), "Tensör verildi") {
			t.Errorf("%s: çelişkili mesaj: %v", d.code, err)
		}
	}
}

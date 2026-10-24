package logging_test

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Çubuktaki sayılar 3 anlamlı basamakla, Türkçe ondalık virgülüyle yazılır.
func TestFormatNumber(t *testing.T) {
	for _, d := range []struct {
		f    float64
		want string
	}{
		{2.31, "2,31"}, {2.3149, "2,31"}, {85.24, "85,2"}, {0.001, "0,001"}, {0.000312, "3,12e-4"},
		{0.0003, "3e-4"}, {1234.6, "1235"}, {1234567, "1,23e6"}, {0, "0"}, {-0.5, "-0,5"}, {1, "1"},
		{math.NaN(), "NaN"}, {math.Inf(1), "sonsuz"}, {math.Inf(-1), "-sonsuz"}, // görüntü: TR_DIL'in değer adları
	} {
		if got := logging.FormatNumber(d.f); got != d.want {
			t.Errorf("FormatNumber(%v) = %q, beklenen %q", d.f, got, d.want)
		}
	}
}

func TestFormatDurationAndBar(t *testing.T) {
	for _, d := range []struct {
		sec  float64
		want string
	}{{0.2, "0 sn"}, {45, "45 sn"}, {182, "3 dk 2 sn"}, {3663.3, "61 dk 3 sn"}, {600, "10 dk"}, {7500, "2 sa 5 dk"}} {
		if got := logging.FormatDuration(d.sec); got != d.want {
			t.Errorf("FormatDuration(%v) = %q, beklenen %q", d.sec, got, d.want)
		}
	}
	for _, d := range []struct {
		ratio float64
		want  string
	}{{0, "▕░░░░░░░░░░░░░░░░▏"}, {0.24, "▕███▊░░░░░░░░░░░░▏"}, {1, "▕████████████████▏"}, {2, "▕████████████████▏"}} {
		if got := logging.Bar(d.ratio); got != d.want {
			t.Errorf("Cubuk(%v) = %q, beklenen %q", d.ratio, got, d.want)
		}
	}
}

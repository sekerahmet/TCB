package gunluk_test

import (
	"math"
	"testing"

	"tr_dili/internal/motor/gunluk"
)

// Çubuktaki sayılar 3 anlamlı basamakla, Türkçe ondalık virgülüyle yazılır.
func TestSayiYazimi(t *testing.T) {
	for _, d := range []struct {
		f    float64
		want string
	}{
		{2.31, "2,31"}, {2.3149, "2,31"}, {85.24, "85,2"}, {0.001, "0,001"}, {0.000312, "3,12e-4"},
		{0.0003, "3e-4"}, {1234.6, "1235"}, {1234567, "1,23e6"}, {0, "0"}, {-0.5, "-0,5"}, {1, "1"},
		{math.NaN(), "NaN"}, {math.Inf(1), "+Sonsuz"}, {math.Inf(-1), "-Sonsuz"},
	} {
		if got := gunluk.SayiYazimi(d.f); got != d.want {
			t.Errorf("SayiYazimi(%v) = %q, beklenen %q", d.f, got, d.want)
		}
	}
}

func TestSureYazimiVeCubuk(t *testing.T) {
	for _, d := range []struct {
		sn   float64
		want string
	}{{0.2, "0 sn"}, {45, "45 sn"}, {182, "3 dk 2 sn"}, {3663.3, "61 dk 3 sn"}, {600, "10 dk"}, {7500, "2 sa 5 dk"}} {
		if got := gunluk.SureYazimi(d.sn); got != d.want {
			t.Errorf("SureYazimi(%v) = %q, beklenen %q", d.sn, got, d.want)
		}
	}
	for _, d := range []struct {
		oran float64
		want string
	}{{0, "▕░░░░░░░░░░░░░░░░▏"}, {0.24, "▕███▊░░░░░░░░░░░░▏"}, {1, "▕████████████████▏"}, {2, "▕████████████████▏"}} {
		if got := gunluk.Cubuk(d.oran); got != d.want {
			t.Errorf("Cubuk(%v) = %q, beklenen %q", d.oran, got, d.want)
		}
	}
}

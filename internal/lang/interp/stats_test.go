package interp

import (
	"math"
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

// Kural (13.1): tam sayı listesinde ortalama, sapma ve ortanca kesin değerin en yakın ondalığıdır, bir
// kez yuvarlanır. Beklenenler CPython 3.12'nin statistics modülünden: mean, stdev, median.
func TestExactStatsMatchCPython(t *testing.T) {
	const most, least = math.MaxInt64, math.MinInt64
	for _, c := range []struct {
		ints              []int64
		mean, std, median float64
	}{
		{[]int64{453326, 303265, 255301}, 337297.3333333333, 103305.9772730181, 303265.0},
		{[]int64{9007199254740993, 1}, 4503599627370497.0, 6369051672525773.0, 4503599627370497.0},
		{[]int64{9007199254740993, 9007199254740995}, 9007199254740994.0, 1.4142135623730951, 9007199254740994.0},
		{[]int64{most, most}, 9.223372036854776e+18, 0.0, 9.223372036854776e+18},
		{[]int64{most, least}, -0.5, 1.3043817825332783e+19, -0.5},
		{[]int64{least, least, -9223372036854775807}, -9.223372036854776e+18, 0.5773502691896257, -9.223372036854776e+18},
		{[]int64{most, 9223372036854775806, -9223372036854775807, 9007199254740993}, 2.308094809027379e+18, 8.829923550641227e+18, 4.616189618054758e+18},
		{[]int64{9007199254740993, 3, 5, 9007199254740995}, 4503599627370499.0, 5200308914369307.0, 4503599627370499.0},
		{[]int64{7, 8, 10}, 8.333333333333334, 1.5275252316519468, 8.0},
		{[]int64{1000000000000001, 1000000000000002, 1000000000000004}, 1000000000000002.4, 1.5275252316519468, 1000000000000002.0},
		{[]int64{2, 4, 4, 4, 5, 5, 7, 9}, 5.0, 2.138089935299395, 4.5},
		{[]int64{0, 0}, 0.0, 0.0, 0.0},
	} {
		if got := exactMean(c.ints); got != c.mean {
			t.Errorf("exactMean(%v) = %v, %v bekleniyordu", c.ints, got, c.mean)
		}
		if got := exactStd(c.ints); got != c.std {
			t.Errorf("exactStd(%v) = %v, %v bekleniyordu", c.ints, got, c.std)
		}
		if got := exactMedian(slices.Clone(c.ints)); got != c.median {
			t.Errorf("exactMedian(%v) = %v, %v bekleniyordu", c.ints, got, c.median)
		}
	}
}

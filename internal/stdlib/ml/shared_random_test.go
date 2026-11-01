package ml

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Kural (20.17, 20.5): paylaşılan üreteç her programda "pcg:00000000000054520000000000002017"
// durumuyla başlar; tohumsuz TR_randn ve TR_sample iki koşuda aynı sayıları verir.
func TestSharedRandomStartsFixed(t *testing.T) {
	if got := SharedRandomState(interp.New()); got != "pcg:00000000000054520000000000002017" {
		t.Fatalf("başlangıç durumu %q", got)
	}
	program := `TR_print(TR_randn([4]).TR_tolist, TR_sample(TR_Tensor([[0.1, 0.2, 0.3, 0.4]])).TR_tolist)`
	if a, b := mustRun(t, program), mustRun(t, program); a != b {
		t.Fatalf("iki koşu ayrı: %q %q", a, b)
	}
	var want *tensor.Tensor
	newSharedRandom().draw(func(s *rand.Rand) { want = tensor.Randn(s, 4) })
	got, _ := tensorData(t, value(t, `TR_var cikti = TR_randn([4])`))
	if !slices.Equal(got, want.Data()) {
		t.Fatalf("ilk çekme sabit durumdan gelmeli: %v, beklenen %v", got, want.Data())
	}
}

// Kural (20.17): durum yazılıp geri konunca aynı sayılar yeniden çekilir; yazım iki durum
// sözcüğüdür, önce yüksek, 16'şar küçük onaltılık rakam.
func TestSharedRandomStateRoundTrip(t *testing.T) {
	m := interp.New()
	r := sharedRandomOf(m)
	r.draw(func(s *rand.Rand) { s.Float64(); s.NormFloat64() })
	saved := SharedRandomState(m)
	draw := func() (out [3]float64) {
		r.draw(func(s *rand.Rand) {
			for i := range out {
				out[i] = s.Float64()
			}
		})
		return out
	}
	a := draw()
	hi, lo, ok := ParseSharedRandomState(saved)
	if !ok {
		t.Fatalf("kendi yazımı çözülmedi: %q", saved)
	}
	SetSharedRandomState(m, hi, lo)
	if b := draw(); a != b || SharedRandomState(m) == saved {
		t.Fatalf("geri konan durum aynı sayıları vermeli: %v %v", a, b)
	}
	if h, l, ok := ParseSharedRandomState("pcg:00000000000054520000000000002017"); !ok || h != 0x5452 || l != 0x2017 {
		t.Fatalf("başlangıç yazımı %x %x %v", h, l, ok)
	}
	for _, bad := range []string{"", "pcg:", "pcg:0000000000005452000000000000201", "pcg:000000000000545200000000000020AB",
		"PCG:00000000000054520000000000002017", "pcg:-000000000005452000000000000201", " pcg:00000000000054520000000000002017"} {
		if _, _, ok := ParseSharedRandomState(bad); ok {
			t.Errorf("%q kabul edildi", bad)
		}
	}
}

// Kural (20.17): tohumlu rastgelelik değişmez; TR_seed: verilen TR_randn Go'nun math/rand
// kaynağıyla önceki sayıları verir ve paylaşılan üreteci ilerletmez.
func TestSeededRandomUnchanged(t *testing.T) {
	want := tensor.Randn(rand.New(rand.NewSource(7)), 3)
	got, _ := tensorData(t, value(t, `TR_var a = TR_randn([3], TR_seed: 7)
TR_var cikti = TR_randn([3], TR_seed: 7)`))
	if !slices.Equal(got, want.Data()) {
		t.Fatalf("tohumlu sayılar değişti: %v, beklenen %v", got, want.Data())
	}
	unseeded, _ := tensorData(t, value(t, `TR_var a = TR_randn([3], TR_seed: 7)
TR_var cikti = TR_randn([3])`))
	var first *tensor.Tensor
	newSharedRandom().draw(func(s *rand.Rand) { first = tensor.Randn(s, 3) })
	if !slices.Equal(unseeded, first.Data()) {
		t.Fatalf("tohumlu çekme paylaşılan üreteci ilerletmemeli: %v, beklenen %v", unseeded, first.Data())
	}
}

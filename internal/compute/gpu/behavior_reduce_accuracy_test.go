package gpu

import (
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// unitRoundoff float32'nin birim yuvarlaması (u = 2⁻²⁴).
const unitRoundoff = 1.0 / (1 << 24)

// Kural: GPU toplamları float64 referansa terim sayısından bağımsız bir hatayla yaklaşır.
// İş parçacığının sıralı toplamı telafilidir (kernel_reduce.go compensatedSum); düz float32
// toplamda hata terim sayısıyla büyürdü (2000 satırlık sütunda referansa göre ~1.9e-5). Veri
// pozitiftir (U(0, 1)): Σ|x| = |Σx|, göreli hata doğrudan u cinsinden okunur. Her durum
// indirgemenin bir yolunu seçer: sütun başına iş parçacığı (tek ve iki aşamalı), bloklu,
// hepsinin toplamı.
func TestSumsMatchFloat64Reference(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	for _, c := range []struct {
		ad    string
		R, D  int
		axis  int // -1: hepsi
		tolU  float64
		short bool // kısa kipte de koşar
	}{
		{"sütun, tek aşama", 2000, 512, 0, 2, true},
		{"sütun, iki aşama", 8192, 256, 0, 2, false},
		{"bloklu satır", 64, 3000, 1, 16, true},
		{"hepsi", 1, 100000, -1, 16, true},
	} {
		if !c.short && testing.Short() {
			continue
		}
		v := make([]float32, c.R*c.D)
		for i := range v {
			v[i] = rng.Float32()
		}
		var ref []float64
		switch c.axis {
		case 0:
			ref = make([]float64, c.D)
			for i, x := range v {
				ref[i%c.D] += float64(x)
			}
		case 1:
			ref = make([]float64, c.R)
			for i, x := range v {
				ref[i/c.D] += float64(x)
			}
		default:
			ref = []float64{0}
			for _, x := range v {
				ref[0] += float64(x)
			}
		}
		for _, x := range twin(v, false, c.R, c.D) {
			var s *tensor.Tensor
			if c.axis < 0 {
				s = x.SumAll()
			} else {
				s = x.Sum(c.axis, false)
			}
			got := s.Data()
			worst, at := 0.0, 0
			for j := range ref {
				if e := math.Abs(float64(got[j])-ref[j]) / ref[j]; e > worst {
					worst, at = e, j
				}
			}
			if worst > c.tolU*unitRoundoff {
				t.Errorf("%s %d×%d (%s): göreli hata %.3g = %.1fu (izin %gu) %d. çıktıda: %v, referans %v",
					c.ad, c.R, c.D, x.Device(), worst, worst/unitRoundoff, c.tolU, at, got[at], ref[at])
			}
		}
	}
}

// Kural: telafili toplam özel değerleri düz toplam gibi verir: sonsuz terimli toplam sonsuz,
// +∞ ile −∞ NaN, NaN terimli NaN, yalnızca -0'lardan oluşan toplam -0 (tek terimli de).
// 256 sütun sütun başına iş parçacığı yolunu seçer.
func TestSumsKeepSpecialValues(t *testing.T) {
	inf, nan, negZero := float32(math.Inf(1)), float32(math.NaN()), float32(math.Copysign(0, -1))
	for _, R := range []int{1, 100} {
		const D = 256
		v := make([]float32, R*D)
		for i := range v {
			v[i] = float32(i%7) - 3
		}
		for i := 0; i < R; i++ {
			v[i*D+0] = negZero
			v[i*D+4] = 1e30
		}
		v[(R-1)*D+1] = inf // 1. sütun: +∞
		v[(R-1)*D+2] = -inf
		if R > 1 {
			v[2] = inf // 2. sütun: +∞ ile −∞ → NaN (tek satırda −∞)
		}
		v[(R/2)*D+3] = nan // 3. sütun: NaN
		x := twin(v, false, R, D)
		cpu, gpu := x[0].Sum(0, false).Data(), x[1].Sum(0, false).Data()
		// 0. sütun (-0'lar) yalnızca GPU'da sınanır: CPU toplamı float64'te +0'dan başlar ve +0
		// verir (ayrı bulgu).
		for j := 1; j < D; j++ {
			c, g := float64(cpu[j]), float64(gpu[j])
			// Özel sütunlar (1–3) bit bit aynı olmalı (NaN'lar NaN), öbürleri yuvarlama içinde.
			same := math.Float64bits(c) == math.Float64bits(g) || (math.IsNaN(c) && math.IsNaN(g))
			if j >= 4 {
				same = same || math.Abs(c-g) <= 1e-6*(1+math.Abs(c))
			}
			if !same {
				t.Errorf("R=%d, %d. sütun: GPU %v (işaret %v), CPU %v (işaret %v)", R, j, g, math.Signbit(g), c, math.Signbit(c))
			}
		}
		if g := float64(gpu[0]); g != 0 || !math.Signbit(g) {
			t.Errorf("R=%d: -0'ların toplamı %v (işaret %v), -0 olmalı", R, g, math.Signbit(g))
		}
		want2 := math.NaN()
		if R == 1 {
			want2 = math.Inf(-1)
		}
		if g1, g2, g3 := float64(gpu[1]), float64(gpu[2]), float64(gpu[3]); !math.IsInf(g1, 1) ||
			(math.IsNaN(want2) != math.IsNaN(g2)) || (!math.IsNaN(want2) && g2 != want2) || !math.IsNaN(g3) {
			t.Errorf("R=%d: sonsuz ve NaN sütunları %v %v %v (beklenen +Inf %v NaN)", R, g1, g2, g3, want2)
		}
	}
}

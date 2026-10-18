package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Test yardımcıları.

func yakin(t *testing.T, ad string, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(got), len(want))
	}
	for i := range got {
		g, w := float64(got[i]), float64(want[i])
		if math.IsNaN(g) || math.Abs(g-w) > tol*(1+math.Abs(w)) {
			t.Fatalf("%s: [%d] = %v, beklenen %v (tüm sonuç %v)", ad, i, got[i], want[i], got)
		}
	}
}

func sekilEsit(t *testing.T, ad string, got, want []int) {
	t.Helper()
	if !sameShape(got, want) {
		t.Fatalf("%s: şekil %v, beklenen %v", ad, got, want)
	}
}

func panikBekle(t *testing.T, ad string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: panik bekleniyordu", ad)
		}
	}()
	fn()
}

// sirali 1, 2, 3, ... değerleriyle tensör.
func sirali(shape ...int) *Tensor {
	d := make([]float32, numelOf(shape))
	for i := range d {
		d[i] = float32(i + 1)
	}
	return New(d, shape...)
}

// naifMatMul float64 birikimli başvuru çarpımı.
func naifMatMul(a, b []float32, M, N, K int) []float32 {
	c := make([]float32, M*N)
	for i := 0; i < M; i++ {
		for j := 0; j < N; j++ {
			var s float64
			for k := 0; k < K; k++ {
				s += float64(a[i*K+k]) * float64(b[k*N+j])
			}
			c[i*N+j] = float32(s)
		}
	}
	return c
}

// rastgele [lo,hi) aralığında; mutlak değeri enAz'dan küçük değerleri iter
// (türevi süreksiz noktalardan uzak durmak için).
func rastgele(rng *rand.Rand, lo, hi, enAz float32, shape ...int) *Tensor {
	d := make([]float32, numelOf(shape))
	for i := range d {
		for {
			v := lo + (hi-lo)*rng.Float32()
			if v < 0 && -v < enAz || v >= 0 && v < enAz {
				continue
			}
			d[i] = v
			break
		}
	}
	return New(d, shape...)
}

// Sayısal gradyan denetimi ayarları (float32 için uygun tolerans).
const (
	gcEps  = 5e-3
	gcAtol = 3e-3
	gcRtol = 2e-2
)

// gradDenetle, f'nin analitik gradyanlarını merkezi sonlu farklarla
// karşılaştırır. Kayıp L = Σ f(xs)·w (w sabit rastgele) float64 ile
// hesaplanır; böylece her çıktı elemanının türevi denetlenmiş olur.
func gradDenetle(t *testing.T, ad string, f func(xs []*Tensor) *Tensor, xs ...*Tensor) {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	for _, x := range xs {
		x.SetRequiresGrad(true)
		x.grad = nil
	}
	out := f(xs)
	if !out.requiresGrad {
		t.Fatalf("%s: çıktı gradyan istemiyor", ad)
	}
	w := Randn(rng, out.shape...)
	wd := w.Data()
	out.BackwardWith(w)
	analitik := make([][]float32, len(xs))
	for k, x := range xs {
		analitik[k] = make([]float32, x.Numel())
		if x.grad != nil {
			copy(analitik[k], cd(x.grad))
		}
	}
	kayip := func() float64 {
		var o *Tensor
		NoGrad(func() { o = f(xs) })
		d := o.Data()
		var s float64
		for i, v := range d {
			s += float64(v) * float64(wd[i])
		}
		return s
	}
	for k, x := range xs {
		if !x.IsContiguous() {
			t.Fatalf("%s: girdi %d bitişik olmalı", ad, k)
		}
		d := cd(x.storage)[x.offset : x.offset+x.Numel()]
		for j := range d {
			orig := d[j]
			xp, xm := orig+gcEps, orig-gcEps
			d[j] = xp
			lp := kayip()
			d[j] = xm
			lm := kayip()
			d[j] = orig
			num := (lp - lm) / (float64(xp) - float64(xm))
			an := float64(analitik[k][j])
			if math.IsNaN(an) || math.Abs(num-an) > gcAtol+gcRtol*math.Max(math.Abs(num), math.Abs(an)) {
				t.Fatalf("%s: girdi %d eleman %d: analitik %.6g, sayısal %.6g", ad, k, j, an, num)
			}
		}
	}
}

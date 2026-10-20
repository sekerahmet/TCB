package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Geri yayılımda iade edilen ara gradyanlar havuzdan yeniden kullanıldığında
// sonuçlar değişmemeli (yeniden kullanılan depolama sıfırlanmış olmalı).
func TestPoolReuse(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	x := Randn(rng, 64, 128)
	W := Randn(rng, 128, 96).SetRequiresGrad(true)
	b := Randn(rng, 96).SetRequiresGrad(true)
	step := func() ([]float32, []float32) {
		W.n.grad, b.n.grad = nil, nil
		h := Add(MatMul(x, W), b).Gelu()
		h.Reshape(32, -1).Square().Mean(1, false).SumAll().Backward()
		return W.Grad().Data(), b.Grad().Data()
	}
	w0, b0 := step()
	for i := 0; i < 5; i++ {
		w1, b1 := step()
		near(t, "W gradyanı", w1, w0, 0)
		near(t, "b gradyanı", b1, b0, 0)
	}

	be := cpuBackend{}
	s := be.Alloc(poolMin, Float32)
	be.Fill(s, 7)
	be.Release(s)
	expectPanic(t, "iki kez iade", func() { be.Release(s) })
	for i := 0; i < 3; i++ {
		y := be.Alloc(poolMin, Float32)
		for _, v := range cd(y) {
			if v != 0 {
				t.Fatal("havuzdan gelen depolama sıfırlanmamış")
			}
		}
		be.Release(y)
	}
}

// Sıfırlanmadan ayrılan (emptyOf) çıktılar: havuzda NaN dolu depolamalar
// bekliyorken işlemler tüm çıktıyı yazmalı; sonuç taze ayırmayla aynı olmalı.
func TestEmptyAllocPoolGarbage(t *testing.T) {
	rng := rand.New(rand.NewSource(22))
	const R, D = 64, 128 // R*D = 8192 >= havuzEnAz
	x := Randn(rng, R, D)
	y := Randn(rng, R, D)
	g := Randn(rng, D)
	bt := Randn(rng, D)
	idx := make([]int, R)
	for i := range idx {
		idx[i] = (i * 7) % D
	}
	w := Randn(rng, D, D)
	ops := []struct {
		ad string
		f  func() *Tensor
	}{
		{"add", func() *Tensor { return Add(x, y) }},
		{"mul_yayin", func() *Tensor { return Mul(x, g) }},
		{"gelu", func() *Tensor { return x.Gelu() }},
		{"softmax", func() *Tensor { return x.Softmax(-1) }},
		{"softmax_eksen0", func() *Tensor { return x.Softmax(0) }},
		{"layer_norm", func() *Tensor { return LayerNorm(x, g, bt, 1e-5) }},
		{"rms_norm", func() *Tensor { return RMSNorm(x, g, 1e-5) }},
		{"embedding", func() *Tensor { return Embedding(w, idx, R) }},
		{"contiguous", func() *Tensor { return x.Reshape(D, R).T().Contiguous() }},
		{"clone", func() *Tensor { return x.Clone() }},
		{"matmul_devrik", func() *Tensor { return MatMul(x.Reshape(D, R).T(), w) }},
		{"bf16", func() *Tensor { return x.Cast(BFloat16).Cast(Float32) }},
	}
	be := cpuBackend{}
	nan := float32(math.NaN())
	for _, is := range ops {
		want := is.f().Data()
		for _, dt := range []DType{Float32, BFloat16} {
			var storages []Storage
			for k := 0; k < 4; k++ {
				s := be.Alloc(R*D, dt)
				be.Fill(s, nan)
				storages = append(storages, s)
			}
			for _, s := range storages {
				be.Release(s)
			}
		}
		got := is.f().Data()
		near(t, is.ad, got, want, 0)
	}
}

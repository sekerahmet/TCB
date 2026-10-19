package tensor

import (
	"math/rand"
	"testing"
)

// Hakem bulguları 8 ve 18 için ölçümler: 16 bitlik yol, karışık hassasiyetli matris
// çarpımı ve büyük toplu çarpım (ayırma sayısı/baytı ve süre).

// Karışık hassasiyet: float32 girdiler bf16'ya yuvarlanıp float32 biriktirmeyle çarpılır;
// ileri + geri.
func BenchmarkMixedMatMulForwardBackward(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	x := Randn(rng, 256, 512).SetRequiresGrad(true)
	w := Randn(rng, 512, 512).SetRequiresGrad(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMulOut(x.Cast(BFloat16), w.Cast(BFloat16), Float32).SumAll().Backward()
		x.ZeroGrad()
		w.ZeroGrad()
	}
}

// bf16 tensörlerle eleman bazlı zincir ve geri yayılım (her işlem 16 bitlik girdiyi
// float32'ye açar, çıktıyı float32 geçici tamponda hesaplar).
func BenchmarkBF16ElementChain(b *testing.B) {
	rng := rand.New(rand.NewSource(2))
	x := Randn(rng, 512, 1024).Cast(BFloat16).SetRequiresGrad(true)
	y := Randn(rng, 512, 1024).Cast(BFloat16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Add(x, y).Mul(x).Gelu().SumAll().Backward()
		x.ZeroGrad()
	}
}

// bf16 parametreli küçük bir eğitim adımı (matmul + layer norm + çapraz entropi + AdamW).
func BenchmarkBF16TrainStep(b *testing.B) {
	rng := rand.New(rand.NewSource(3))
	w1 := Randn(rng, 256, 256).MulScalar(0.05).Cast(BFloat16).SetRequiresGrad(true)
	w2 := Randn(rng, 256, 64).MulScalar(0.05).Cast(BFloat16).SetRequiresGrad(true)
	g := Ones(256).Cast(BFloat16).SetRequiresGrad(true)
	x := Randn(rng, 128, 256).Cast(BFloat16)
	target := make([]int, 128)
	for i := range target {
		target[i] = rng.Intn(64)
	}
	opt := NewAdamW([]*Tensor{w1, w2, g}, 1e-3, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opt.ZeroGrad()
		h := LayerNorm(x.MatMul(w1).Gelu(), g, nil, 1e-5)
		CrossEntropy(h.MatMul(w2), target).Backward()
		opt.Step()
	}
}

// Dikkat biçimli büyük toplu çarpım: 64 × (1024×64)·(64×1024), ileri + geri.
func BenchmarkBatchedMatMulLarge(b *testing.B) {
	rng := rand.New(rand.NewSource(4))
	q := Randn(rng, 64, 1024, 64).SetRequiresGrad(true)
	k := Randn(rng, 64, 64, 1024).SetRequiresGrad(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(q, k).SumAll().Backward()
		q.ZeroGrad()
		k.ZeroGrad()
	}
}

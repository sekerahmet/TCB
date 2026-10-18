package tensor

import (
	"math/rand"
	"testing"
)

// Çekirdek düzeyi hız ölçümleri (eğitimde sık geçen boyutlar):
//
//	go test ./pkg/tensor/ -run '^$' -bench 'Hiz' -benchmem -count 3

const (
	hizR = 2048 // satır (toplu × dizi)
	hizD = 512  // gizli boyut
)

func hizRand(shape ...int) *Tensor { return Randn(rand.New(rand.NewSource(3)), shape...) }

func hizDilim(rng *rand.Rand, n int) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(rng.NormFloat64())
	}
	return d
}

func BenchmarkHizSumSonEksen(b *testing.B) {
	x := hizRand(hizR, hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Sum(-1, false)
	}
}

func BenchmarkHizSumEksen0(b *testing.B) {
	x := hizRand(hizR, hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Sum(0, false)
	}
}

func BenchmarkHizSumAll(b *testing.B) {
	x := hizRand(hizR, hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.SumAll()
	}
}

func BenchmarkHizFill(b *testing.B) {
	x := hizRand(hizR, hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.be.Fill(x.storage, 0)
		x.be.Fill(x.storage, 1)
	}
}

func BenchmarkHizDevrikBitisik(b *testing.B) {
	x := hizRand(1024, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.T().Contiguous()
	}
}

func BenchmarkHizScatterAdd(b *testing.B) {
	// Dilim görünümünün geri yayılımı: (R, 3D) içinden orta D sütun.
	x := hizRand(hizR, 3*hizD)
	g := hizRand(hizR, hizD)
	hedef := x.be.Alloc(x.Numel(), Float32)
	st := contiguousStrides(x.shape)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.be.ScatterAdd(hedef, hizD, st, g.storage, g.shape)
	}
}

func BenchmarkHizExpandAdd(b *testing.B) {
	// Satır toplamının geri yayılımı: (R, 1) → (R, D).
	g := hizRand(hizR, 1)
	hedef := hizRand(hizR, hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hedef.be.ExpandAdd(hedef.storage, hedef.shape, g.storage, g.shape, 1)
	}
}

func BenchmarkHizSapmaGradyani(b *testing.B) {
	// BinaryGrad genel yolu: (R, D) + (D) toplamının sapma gradyanı.
	g := hizRand(hizR, hizD)
	hedef := g.be.Alloc(hizD, Float32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.be.BinaryGrad(GradPlus, hedef, []int{hizD}, g.shape, g.storage, View{}, nil)
	}
}

func BenchmarkHizEmbeddingGrad(b *testing.B) {
	V := 8192
	g := hizRand(hizR, hizD)
	idx := make([]int, hizR)
	for i := range idx {
		idx[i] = i * 31 % V
	}
	hedef := g.be.Alloc(V*hizD, Float32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.be.EmbeddingGrad(hedef, g.storage, idx, hizD)
	}
}

func BenchmarkHizClipGradNorm(b *testing.B) {
	p := hizRand(4*hizR, hizD).SetRequiresGrad(true)
	p.accGrad()
	p.be.Fill(p.grad, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClipGradNorm([]*Tensor{p}, 1)
	}
}

func BenchmarkHizReshapeGeri(b *testing.B) {
	// Görünüm zinciri: reshape → contiguous(devrik) → clone, geri yayılımla.
	x := hizRand(hizR, hizD).SetRequiresGrad(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Reshape(hizR/2, 2*hizD).Clone().Reshape(hizR, hizD).SumAll().Backward()
	}
}

func BenchmarkHizGelu(b *testing.B) {
	x := hizRand(hizR, 4*hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Gelu()
	}
}

func BenchmarkHizRelu(b *testing.B) {
	x := hizRand(hizR, 4*hizD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Relu()
	}
}

// Tekli çekirdekler, ayırma dışında (çıktı önceden ayrılmış).
func BenchmarkHizTekliCekirdek(b *testing.B) {
	x := hizRand(hizR, 4*hizD)
	out := x.be.Alloc(x.Numel(), Float32)
	g := x.be.Alloc(x.Numel(), Float32)
	for _, op := range []struct {
		ad string
		op UnOp
	}{{"relu", OpRelu}, {"gelu", OpGelu}, {"mul_scalar", OpMulScalar}, {"exp", OpExp}} {
		b.Run(op.ad, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				x.be.Unary(op.op, 0.5, out, x.view())
				x.be.UnaryGrad(op.op, 0.5, g, out, x.view(), out)
			}
		})
	}
}

func BenchmarkHizKucukIslem(b *testing.B) {
	// paralelEsik'in biraz üstünde: işçi sayısı işe göre sınırlanmalı.
	x := hizRand(64, 1024)
	y := hizRand(64, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Add(x, y)
	}
}

func BenchmarkHizKucukMatMul(b *testing.B) {
	// tekCekirdekEsik sınırında (64³ = 2^18).
	x := hizRand(64, 64)
	y := hizRand(64, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y)
	}
}

func BenchmarkHizMatMul256(b *testing.B) {
	x := hizRand(256, 256)
	y := hizRand(256, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y)
	}
}

func BenchmarkHizLayerNormGeri(b *testing.B) {
	x := hizRand(hizR, hizD).SetRequiresGrad(true)
	gamma := Ones(hizD).SetRequiresGrad(true)
	beta := Zeros(hizD).SetRequiresGrad(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LayerNorm(x, gamma, beta, 1e-5).SumAll().Backward()
	}
}

// Küçük bir MLP eğitim adımı (ileri + geri + AdamW): ayırma ve havuzun etkisi.
func BenchmarkHizEgitimAdimi(b *testing.B) {
	rng := rand.New(rand.NewSource(5))
	x := Randn(rng, 512, 256)
	hedef := make([]int, 512)
	for i := range hedef {
		hedef[i] = i % 64
	}
	W1 := Randn(rng, 256, 1024).MulScalar(0.05).SetRequiresGrad(true)
	b1 := Zeros(1024).SetRequiresGrad(true)
	W2 := Randn(rng, 1024, 64).MulScalar(0.05).SetRequiresGrad(true)
	g := Ones(256).SetRequiresGrad(true)
	opt := NewAdamW([]*Tensor{W1, b1, W2, g}, 1e-3, 0.01)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opt.ZeroGrad()
		h := Add(MatMul(LayerNorm(x, g, nil, 1e-5), W1), b1).Gelu()
		CrossEntropy(MatMul(h, W2), hedef).Backward()
		opt.Step()
	}
}

// Karakter düzeyi küçük dil modelindeki çarpım boyutları (B=16, T=64, D=64, H=4).
func BenchmarkHizModelMatMul(b *testing.B) {
	rng := rand.New(rand.NewSource(33))
	for _, d := range []struct {
		ad             string
		batch, M, N, K int
		transA, transB bool
	}{
		{"1024x64x64", 1, 1024, 64, 64, false, false},
		{"1024x64x256", 1, 1024, 256, 64, false, false},
		{"1024x256x64", 1, 1024, 64, 256, false, false},
		{"dA_1024x256_Bt", 1, 1024, 64, 256, false, true},
		{"dB_At_64x256x1024", 1, 64, 256, 1024, true, false},
		{"dikkat_QKt_64x64x16", 64, 64, 64, 16, false, true},
		{"dikkat_AV_64x16x64", 64, 64, 16, 64, false, false},
	} {
		a := hizDilim(rng, d.batch*d.M*d.K)
		bb := hizDilim(rng, d.batch*d.K*d.N)
		c := make([]float32, d.batch*d.M*d.N)
		b.Run(d.ad, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				gemm(d.batch, d.M, d.N, d.K, a, d.transA, bb, d.transB, c)
			}
			b.ReportMetric(2*float64(d.batch*d.M*d.N*d.K)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOPS")
		})
	}
}

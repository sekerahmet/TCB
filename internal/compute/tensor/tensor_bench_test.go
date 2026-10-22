package tensor

import (
	"fmt"
	"math/rand"
	"testing"
)

// Çekirdek düzeyi hız ölçümleri (eğitimde sık geçen boyutlar):
//
//	go test ./internal/compute/tensor/ -run '^$' -bench 'Hiz' -benchmem -count 3

const (
	speedR = 2048 // satır (toplu × dizi)
	speedD = 512  // gizli boyut
)

func speedRand(shape ...int) *Tensor { return Randn(rand.New(rand.NewSource(3)), shape...) }

func BenchmarkSpeedSumLastAxis(b *testing.B) {
	x := speedRand(speedR, speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Sum(-1, false)
	}
}

func BenchmarkSpeedSumAxis0(b *testing.B) {
	x := speedRand(speedR, speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Sum(0, false)
	}
}

func BenchmarkSpeedSumAll(b *testing.B) {
	x := speedRand(speedR, speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.SumAll()
	}
}

func BenchmarkSpeedFill(b *testing.B) {
	x := speedRand(speedR, speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.be.Fill(x.storage, 0)
		x.be.Fill(x.storage, 1)
	}
}

func BenchmarkSpeedTransposeContiguous(b *testing.B) {
	x := speedRand(1024, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.T().Contiguous()
	}
}

func BenchmarkSpeedScatterAdd(b *testing.B) {
	// Dilim görünümünün geri yayılımı: (R, 3D) içinden orta D sütun.
	x := speedRand(speedR, 3*speedD)
	g := speedRand(speedR, speedD)
	target := x.be.Alloc(x.Numel(), Float32)
	st := contiguousStrides(x.shape)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.be.ScatterAdd(target, speedD, st, g.storage, g.shape)
	}
}

func BenchmarkSpeedExpandAdd(b *testing.B) {
	// Satır toplamının geri yayılımı: (R, 1) → (R, D).
	g := speedRand(speedR, 1)
	target := speedRand(speedR, speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target.be.ExpandAdd(target.storage, target.shape, g.storage, g.shape, 1)
	}
}

func BenchmarkSpeedBiasGrad(b *testing.B) {
	// BinaryGrad genel yolu: (R, D) + (D) toplamının sapma gradyanı.
	g := speedRand(speedR, speedD)
	target := g.be.Alloc(speedD, Float32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.be.BinaryGrad(GradPlus, target, []int{speedD}, g.shape, g.storage, View{}, nil)
	}
}

func BenchmarkSpeedEmbeddingGrad(b *testing.B) {
	V := 8192
	g := speedRand(speedR, speedD)
	idx := make([]int, speedR)
	for i := range idx {
		idx[i] = i * 31 % V
	}
	target := g.be.Alloc(V*speedD, Float32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.be.EmbeddingGrad(target, g.storage, idx, speedD)
	}
}

func BenchmarkSpeedClipGradNorm(b *testing.B) {
	p := speedRand(4*speedR, speedD).SetRequiresGrad(true)
	p.n.accGrad()
	p.be.Fill(p.n.grad, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClipGradNorm([]*Tensor{p}, 1)
	}
}

func BenchmarkSpeedReshapeBackward(b *testing.B) {
	// Görünüm zinciri: reshape → contiguous(devrik) → clone, geri yayılımla.
	x := speedRand(speedR, speedD).SetRequiresGrad(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Reshape(speedR/2, 2*speedD).Clone().Reshape(speedR, speedD).SumAll().Backward()
	}
}

func BenchmarkSpeedGelu(b *testing.B) {
	x := speedRand(speedR, 4*speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Gelu()
	}
}

func BenchmarkSpeedRelu(b *testing.B) {
	x := speedRand(speedR, 4*speedD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Relu()
	}
}

// Tekli çekirdekler, ayırma dışında (çıktı önceden ayrılmış).
func BenchmarkSpeedUnaryKernel(b *testing.B) {
	x := speedRand(speedR, 4*speedD)
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

func BenchmarkSpeedSmallOp(b *testing.B) {
	// paralelEsik'in biraz üstünde: işçi sayısı işe göre sınırlanmalı.
	x := speedRand(64, 1024)
	y := speedRand(64, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Add(x, y)
	}
}

// BenchmarkSpeedAdd64 en küçük sıcak yol: ayırma denetiminin payı burada görünür.
func BenchmarkSpeedAdd64(b *testing.B) {
	x := speedRand(64, 64)
	y := speedRand(64, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Add(x, y)
	}
}

func BenchmarkSpeedSmallMatMul(b *testing.B) {
	// tekCekirdekEsik sınırında (64³ = 2^18).
	x := speedRand(64, 64)
	y := speedRand(64, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y)
	}
}

func BenchmarkSpeedMatMul256(b *testing.B) {
	x := speedRand(256, 256)
	y := speedRand(256, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y)
	}
}

func BenchmarkSpeedLayerNormBackward(b *testing.B) {
	x := speedRand(speedR, speedD).SetRequiresGrad(true)
	gamma := Ones(speedD).SetRequiresGrad(true)
	beta := Zeros(speedD).SetRequiresGrad(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LayerNorm(x, gamma, beta, 1e-5).SumAll().Backward()
	}
}

// Küçük bir MLP eğitim adımı (ileri + geri + AdamW): ayırma ve havuzun etkisi.
func BenchmarkSpeedTrainStep(b *testing.B) {
	rng := rand.New(rand.NewSource(5))
	x := Randn(rng, 512, 256)
	target := make([]int, 512)
	for i := range target {
		target[i] = i % 64
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
		CrossEntropy(MatMul(h, W2), target).Backward()
		opt.Step()
	}
}

// Karakter düzeyi küçük dil modelindeki çarpım boyutları (B=16, T=64, D=64, H=4).
func BenchmarkSpeedModelMatMul(b *testing.B) {
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
		a := randomSlice(rng, d.batch*d.M*d.K)
		bb := randomSlice(rng, d.batch*d.K*d.N)
		c := make([]float32, d.batch*d.M*d.N)
		b.Run(d.ad, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				gemm(d.batch, d.M, d.N, d.K, a, d.transA, bb, d.transB, c)
			}
			b.ReportMetric(2*float64(d.batch*d.M*d.N*d.K)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOPS")
		})
	}
}

// Karakter modelindeki çarpım dışı çekirdekler (B·T = 1024 satır, D = 64, 4D = 256,
// V = 131), ayırma ve türev çizgesi dahil:
//
//	GOMAXPROCS=2 go test ./internal/compute/tensor/ -run '^$' -bench 'SpeedModelKernel' -count 5
func BenchmarkSpeedModelKernel(b *testing.B) {
	rng := rand.New(rand.NewSource(35))
	x := Randn(rng, 1024, 64).SetRequiresGrad(true)
	g := Ones(64).SetRequiresGrad(true)
	s := Zeros(64).SetRequiresGrad(true)
	h := Randn(rng, 1024, 256).SetRequiresGrad(true)
	l := Randn(rng, 1024, 131).SetRequiresGrad(true)
	target := make([]int, 1024)
	for i := range target {
		target[i] = i * 7 % 131
	}
	for _, d := range []struct {
		ad string
		f  func()
	}{
		{"katman_normu_ileri", func() { LayerNorm(x, g, s, 1e-5) }},
		{"katman_normu_ileri_geri", func() { LayerNorm(x, g, s, 1e-5).SumAll().Backward() }},
		{"rms_normu_ileri_geri", func() { RMSNorm(x, g, 1e-5).SumAll().Backward() }},
		{"gelu_ileri", func() { h.Gelu() }},
		{"gelu_ileri_geri", func() { h.Gelu().SumAll().Backward() }},
		{"capraz_entropi_ileri", func() { CrossEntropy(l, target) }},
		{"capraz_entropi_ileri_geri", func() { CrossEntropy(l, target).Backward() }},
	} {
		b.Run(d.ad, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				d.f()
			}
		})
	}
}

// Başarım ölçümleri:
//
//	go test ./internal/compute/tensor/ -run '^$' -bench . -benchtime 5x

func benchMatMul(b *testing.B, n int) {
	rng := rand.New(rand.NewSource(1))
	x := Randn(rng, n, n)
	y := Randn(rng, n, n)
	b.SetBytes(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y)
	}
	b.ReportMetric(2*float64(n)*float64(n)*float64(n)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOPS")
}

func BenchmarkMatMul512(b *testing.B)  { benchMatMul(b, 512) }
func BenchmarkMatMul1024(b *testing.B) { benchMatMul(b, 1024) }

// İleri + geri matris çarpımı (eğitimdeki tipik maliyet: 3 çarpım).
func BenchmarkMatMulForwardBackward1024(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	x := Randn(rng, 1024, 1024).SetRequiresGrad(true)
	y := Randn(rng, 1024, 1024).SetRequiresGrad(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y).SumAll().Backward()
	}
}

// Toplu çarpım: 8 bakış × (256×64)·(64×256), dikkat puanları gibi.
func BenchmarkMatMulBatched(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	q := Randn(rng, 8, 256, 64)
	k := Randn(rng, 8, 64, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(q, k)
	}
}

func BenchmarkSoftmax(b *testing.B) {
	for _, d := range [][2]int{{1024, 1024}, {256, 32000}} {
		b.Run(fmt.Sprintf("%dx%d", d[0], d[1]), func(b *testing.B) {
			x := Randn(rand.New(rand.NewSource(1)), d[0], d[1])
			b.SetBytes(int64(d[0] * d[1] * 4))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x.Softmax(-1)
			}
		})
	}
}

func BenchmarkCrossEntropyForwardBackward(b *testing.B) {
	x := Randn(rand.New(rand.NewSource(1)), 256, 32000).SetRequiresGrad(true)
	target := make([]int, 256)
	for i := range target {
		target[i] = i * 7 % 32000
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CrossEntropy(x, target).Backward()
	}
}

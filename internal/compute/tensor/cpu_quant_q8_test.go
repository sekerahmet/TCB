package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// TestQ4KQ8KernelMatchesGo: AVX2 çekirdeği (sdotq4kq8) Go eşiyle (dotQ4KQ8Go) bit bit aynıdır; ikisi de
// açılmış float çarpımına yakındır (nicemleme hatası kadar). H1 hız prototipi.
func TestQ4KQ8KernelMatchesGo(t *testing.T) {
	if !hasAVX2 || !hasF16C {
		t.Skip("AVX2 yok")
	}
	r := rand.New(rand.NewSource(31))
	for _, K := range []int{256, 512, 2560, 9728} {
		for trial := range 20 {
			w := smallScaleQuant(Q4K, uint32(K+trial), K/kBlock)
			x := randomData(r, K)
			if trial%5 == 0 {
				for i := range x {
					x[i] *= 1000
				}
			}
			q := newQ8KRow(K)
			quantizeQ8K(x, q)
			var tmp [16]byte
			got := sdotq4kq8(K/kBlock, &w[0], &q.q[0], &q.d[0], &q.bs[0], &tmp[0])
			want := dotQ4KQ8Go(K/kBlock, w, q)
			if math.Float32bits(got) != math.Float32bits(want) {
				t.Fatalf("K=%d deneme %d: çekirdek %v (%#x), Go %v (%#x)", K, trial, got, math.Float32bits(got), want, math.Float32bits(want))
			}
			ref := make([]float32, K)
			dequantBlocks(Q4K, ref, w)
			var f, n2 float64
			for i := range x {
				f += float64(x[i]) * float64(ref[i])
				n2 += math.Abs(float64(x[i]) * float64(ref[i]))
			}
			if d := math.Abs(float64(got) - f); d > 0.02*n2+1e-6 {
				t.Fatalf("K=%d deneme %d: nicemli %v, float %v, fark %v (ölçek %v)", K, trial, got, f, d, n2)
			}
		}
	}
}

// TestQ4KQ8FourRowsMatchOneRow: dört satırlı çekirdeğin her satırı tek satırlı çekirdekle bit bit aynıdır;
// quantGemmQ8 (dörtlü ve kalan satırlar, q8BlockN'lik bloklar ve eksik son blok) her satırda quantGemvQ8 ile
// aynıdır; Q6_K'da da.
func TestQ4KQ8FourRowsMatchOneRow(t *testing.T) {
	r := rand.New(rand.NewSource(32))
	for _, dt := range []DType{Q4K, Q6K} {
		for _, c := range [][3]int{{4, 3, 256}, {7, 17, 512}, {9, 40, 2560}, {13, 5, 9728}, {21, 33, 512}} {
			M, N, K := c[0], c[1], c[2]
			w := smallScaleQuant(dt, uint32(M+N+K), N*K/kBlock)
			x := randomData(r, M*K)
			got := make([]float32, M*N)
			quantGemmQ8(M, N, K, x, w, dt, got)
			for m := range M {
				want := make([]float32, N)
				quantGemvQ8(N, K, x[m*K:(m+1)*K], w, dt, want)
				sameFloatBits(t, dt.Name()+" satır", got[m*N:(m+1)*N], want)
			}
		}
	}
}

// TestQ6KQ8KernelMatchesGo: Q6_K'nın AVX2 çekirdeği Go eşiyle bit bit aynı, açılmış float çarpımına yakın.
func TestQ6KQ8KernelMatchesGo(t *testing.T) {
	if !hasAVX2 || !hasF16C {
		t.Skip("AVX2 yok")
	}
	r := rand.New(rand.NewSource(33))
	for _, K := range []int{256, 512, 2560, 9728} {
		for trial := range 20 {
			w := smallScaleQuant(Q6K, uint32(K+trial), K/kBlock)
			x := randomData(r, K)
			q := newQ8KRow(K)
			quantizeQ8K(x, q)
			var tmp [32]byte
			got := sdotq6kq8(K/kBlock, &w[0], &q.q[0], &q.d[0], &q.bs[0], &tmp[0])
			want := dotQ6KQ8Go(K/kBlock, w, q)
			if math.Float32bits(got) != math.Float32bits(want) {
				t.Fatalf("K=%d deneme %d: çekirdek %v, Go %v", K, trial, got, want)
			}
			ref := make([]float32, K)
			dequantBlocks(Q6K, ref, w)
			var f, n2 float64
			for i := range x {
				f += float64(x[i]) * float64(ref[i])
				n2 += math.Abs(float64(x[i]) * float64(ref[i]))
			}
			if d := math.Abs(float64(got) - f); d > 0.02*n2+1e-6 {
				t.Fatalf("K=%d deneme %d: nicemli %v, float %v, fark %v", K, trial, got, f, d)
			}
		}
	}
}

// TestQ8PathKeepsNonFinite: prototip açıkken de girdide NaN ya da sonsuz varsa sonuç float yolununkiyle bit bit
// aynıdır (nicemli yol kullanılmaz); sonlu girdide nicemli yol gerçekten kullanılır (sonuç farklıdır).
func TestQ8PathKeepsNonFinite(t *testing.T) {
	eski := actQuant
	defer func() { actQuant = eski }()
	r := rand.New(rand.NewSource(34))
	for _, dt := range []DType{Q4K, Q6K} {
		for _, M := range []int{1, 5} {
			N, K := 9, 512
			w := FromQuantized(dt, smallScaleQuant(dt, uint32(M+N), N*K/kBlock), N, K)
			for _, kotu := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
				data := randomData(r, M*K)
				data[K/2] = kotu
				x := New(data, M, K)
				actQuant = false
				want := MatMul(x, w.T()).Data()
				actQuant = true
				sameFloatBits(t, dt.Name()+" sonlu olmayan", MatMul(x, w.T()).Data(), want)
			}
			x := New(randomData(r, M*K), M, K)
			actQuant = false
			float := MatMul(x, w.T()).Data()
			actQuant = true
			nicem := MatMul(x, w.T()).Data()
			ayni := true
			for i := range float {
				if math.Float32bits(float[i]) != math.Float32bits(nicem[i]) {
					ayni = false
				}
			}
			if ayni {
				t.Fatalf("%s M=%d: nicemli yol kullanılmadı (sonuç float yolunun aynısı)", dt.Name(), M)
			}
		}
	}
}

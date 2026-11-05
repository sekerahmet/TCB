package tensor

import (
	"fmt"
	"math/rand"
	"runtime"
	"testing"
)

// Paralel yollar: işçi havuzuna dağıtılan büyük işler ve dQ'nun anahtar
// dilimlerinde ayrı biriktirildiği az bakışlı durum.
func TestAttentionParallel(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	for _, blk := range [][2]int{{0, 0}, {32, 48}} {
		SetAttentionBlocks(CPU, blk[0], blk[1])
		for _, d := range []attnState{
			{ad: "paralel nedensel", B: 2, H: 4, Hkv: 4, Tq: 300, Tk: 300, Dh: 64, Dv: 64, causalFlag: true},
			{ad: "paralel gqa tek toplu", B: 1, H: 8, Hkv: 2, Tq: 257, Tk: 257, Dh: 64, Dv: 64, causalFlag: true},
			{ad: "paralel pencere", B: 1, H: 2, Hkv: 1, Tq: 400, Tk: 400, Dh: 64, Dv: 32, causalFlag: true, window: 70},
			{ad: "paralel maske", B: 1, H: 2, Hkv: 2, Tq: 200, Tk: 200, Dh: 64, Dv: 64, maskVal: true},
		} {
			p := AttentionParams{Batch: d.B, H: d.H, Tq: d.Tq, Tk: d.Tk, Dh: d.Dh, Dv: d.Dv, Causal: d.causalFlag, Window: d.window}
			if attnWorkerCount(attnJob(&p)) < 2 && workers() > 1 {
				t.Fatalf("%s: paralel yol denenmiyor", d.ad)
			}
			d.ad = fmt.Sprintf("%s blok=%v", d.ad, blk)
			compareAttention(t, d, 5e-4)
		}
	}
}

// Sonuç işçi sayısından bağımsızdır: tek geçiş ve iki geçiş kipleri (ve tek
// iş parçacığı) bit düzeyinde aynı ileri çıktıyı ve gradyanları verir.
func TestAttentionIndependentOfWorkers(t *testing.T) {
	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)
	rng := rand.New(rand.NewSource(11))
	q := Randn(rng, 1, 4, 200, 64)
	k := Randn(rng, 1, 2, 200, 64)
	v := Randn(rng, 1, 2, 200, 64)
	g := Randn(rng, 1, 4, 200, 64)
	execute := func(n int) [][]float32 {
		runtime.GOMAXPROCS(n)
		qq, kk, vv := q.Detach().SetRequiresGrad(true), k.Detach().SetRequiresGrad(true), v.Detach().SetRequiresGrad(true)
		o := attentionHM(qq, kk, vv, AttentionOptions{Causal: true, Window: 90})
		o.BackwardWith(g)
		return [][]float32{o.Data(), qq.Grad().Data(), kk.Grad().Data(), vv.Grad().Data()}
	}
	a := execute(1)
	for _, n := range []int{2, 3, 8, 32} {
		b := execute(n)
		for i := range a {
			for j := range a[i] {
				if a[i][j] != b[i][j] {
					t.Fatalf("GOMAXPROCS=%d: çıktı %d eleman %d farklı: %v ≠ %v", n, i, j, b[i][j], a[i][j])
				}
			}
		}
	}
}

// attnDecodeCases üretim adımı ve kısa promptun şekilleridir: tek ve iki toplu eleman, blok sınırının
// iki yanı, GQA ve pencere.
var attnDecodeCases = []struct {
	B, H, Hkv, Tq, Tk, Dh, window int
}{
	{1, 16, 8, 1, 100, 128, 0}, {1, 16, 8, 1, 64, 128, 0}, {1, 16, 8, 1, 65, 128, 0}, {2, 4, 2, 1, 33, 32, 0},
	{1, 8, 1, 1, 130, 64, 0}, {1, 6, 2, 1, 70, 16, 5}, {1, 4, 4, 1, 40, 16, 0}, {1, 4, 2, 5, 40, 16, 0}, {2, 4, 2, 70, 70, 16, 9},
}

// TestAttentionWithoutGradMatchesCopied: türevsiz yol K ve V'yi yerinde okur, tek sorguda bakışlar birlikte
// gezilir; sonuç kopyalı yolla bit bit aynıdır, işçi sayısından bağımsızdır. K ve V önbelleğin dilimidir.
func TestAttentionWithoutGradMatchesCopied(t *testing.T) {
	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)
	for _, c := range attnDecodeCases {
		rng := rand.New(rand.NewSource(int64(c.Tk)))
		q := Randn(rng, c.B, c.Tq, c.H, c.Dh)
		buffer := Randn(rng, c.B, c.Tk+7, c.Hkv, c.Dh)
		other := Randn(rng, c.B, c.Tk+7, c.Hkv, c.Dh)
		k, v := buffer.Slice(1, 0, c.Tk), other.Slice(1, 0, c.Tk)
		opt := AttentionOptions{Causal: true, Window: c.window}
		runtime.GOMAXPROCS(1)
		want := Attention(q, k, v, opt).Data()
		for _, n := range []int{1, 2, 3, 8} {
			runtime.GOMAXPROCS(n)
			var got *Tensor
			NoGrad(func() { got = Attention(q, k, v, opt) })
			sameFloatBits(t, "türevsiz", got.Data(), want)
		}
	}
}

// TestAttentionSharedHeadsMatchSeparate: tek sorguda K ve V'yi paylaşan bakışların çıktısı, her bakışa kendi
// kopyası verilmiş dikkatle bit bit aynıdır.
func TestAttentionSharedHeadsMatchSeparate(t *testing.T) {
	const H, Hkv, Dh = 8, 2, 64
	for _, Tk := range []int{1, 15, 16, 64, 65, 100} {
		rng := rand.New(rand.NewSource(int64(Tk)))
		q := Randn(rng, 1, 1, H, Dh)
		k, v := Randn(rng, 1, Tk, Hkv, Dh), Randn(rng, 1, Tk, Hkv, Dh)
		spread := func(x *Tensor) *Tensor {
			d := x.Data()
			out := make([]float32, 0, Tk*H*Dh)
			for j := range Tk {
				for h := range H {
					kv := h / (H / Hkv)
					out = append(out, d[(j*Hkv+kv)*Dh:(j*Hkv+kv+1)*Dh]...)
				}
			}
			return New(out, 1, Tk, H, Dh)
		}
		opt := AttentionOptions{Causal: true}
		var shared, separate *Tensor
		NoGrad(func() {
			shared = Attention(q, k, v, opt)
			separate = Attention(q, spread(k), spread(v), opt)
		})
		sameFloatBits(t, "paylaşılan", shared.Data(), separate.Data())
	}
}

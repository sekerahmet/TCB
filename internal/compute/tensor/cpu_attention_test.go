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

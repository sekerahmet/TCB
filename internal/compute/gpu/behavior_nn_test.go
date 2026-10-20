package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Sinir ağı işlemleri: matris çarpımı (devrikler, toplu, iki döşeme ayarı), softmax,
// çapraz entropi, normlar, gömme ve dikkat; ileri ve gradyanlar CPU ile karşılaştırılır.

func TestMatMul(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	g := gpuBackend()
	defer g.gemmTile(nil)
	for _, ay := range []gemmConfig{defaultGemm, largeGemm, {32, 16, 8, 2, 4}} {
		g.gemmTile(&ay)
		for _, d := range [][4]int{{1, 1, 1, 1}, {1, 3, 5, 7}, {2, 17, 13, 9}, {1, 64, 64, 64}, {3, 65, 31, 129}, {1, 130, 1, 70}, {1, 5, 0, 4}} {
			b, M, K, N := d[0], d[1], d[2], d[3]
			av, bv := random(rng, b*M*K, 1), random(rng, b*K*N, 1)
			ad := fmt.Sprintf("döşeme %v, %d×(%d×%d)·(%d×%d)", ay, b, M, K, K, N)
			compare(t, ad, 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x, y := twin(av, true, b, M, K)[c], twin(bv, true, b, K, N)[c]
				return tensor.MatMul(x, y), []*tensor.Tensor{x, y}
			})
			compare(t, ad+" devrikli", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x, y := twin(av, true, b, K, M)[c], twin(bv, true, b, N, K)[c]
				return tensor.MatMul(x.Transpose(1, 2), y.Transpose(1, 2)), []*tensor.Tensor{x, y}
			})
		}
	}
	xv, wv := random(rng, 2*5*6, 1), random(rng, 6*4, 1)
	compare(t, "doğrusal katman (3B @ 2B)", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, w := twin(xv, true, 2, 5, 6)[c], twin(wv, true, 6, 4)[c]
		return tensor.MatMul(x, w), []*tensor.Tensor{x, w}
	})
}

func TestSoftmax(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	for _, sh := range [][]int{{1, 1}, {3, 7}, {2, 300}, {5, 4, 3}, {1, 1031}} {
		v := random(rng, numel(sh), 3)
		for ax := range sh {
			for _, lg := range []bool{false, true} {
				compare(t, fmt.Sprintf("softmax %v eksen %d log=%v", sh, ax, lg), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, sh...)[c]
					if lg {
						return x.LogSoftmax(ax), []*tensor.Tensor{x}
					}
					return x.Softmax(ax), []*tensor.Tensor{x}
				})
			}
		}
	}
}

func TestCrossEntropy(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	// 300 ve 2056: 4'lü vektör yolu (2056 > 4·256: satır birden çok turda gezilir); 1 ve 1031:
	// tekli yol.
	for _, d := range [][2]int{{1, 2}, {5, 7}, {9, 300}, {4, 1031}, {3, 2056}, {6, 1}} {
		R, C := d[0], d[1]
		v := random(rng, R*C, 3)
		target := make([]int, R)
		for i := range target {
			target[i] = rng.Intn(C)
		}
		if R > 2 {
			target[1] = -1 // yok sayılan satır
		}
		compare(t, fmt.Sprintf("çapraz entropi %d×%d", R, C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, R, C)[c]
			return tensor.CrossEntropy(x, target), []*tensor.Tensor{x}
		})
		// Aynı logitlerden iki kayıp: ikinci gradyan birincinin üzerine eklenir (birikim yolu).
		second := make([]int, R)
		for i := range second {
			second[i] = (target[i] + 1) % C
		}
		compare(t, fmt.Sprintf("iki çapraz entropi %d×%d", R, C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, R, C)[c]
			return tensor.Add(tensor.CrossEntropy(x, target), tensor.CrossEntropy(x, second)), []*tensor.Tensor{x}
		})
	}
}

func TestNorms(t *testing.T) {
	rng := rand.New(rand.NewSource(10))
	for _, d := range [][2]int{{1, 1}, {3, 8}, {5, 129}, {2, 600}} {
		R, D := d[0], d[1]
		xv, gv, bv := random(rng, R*D, 2), random(rng, D, 1), random(rng, D, 1)
		compare(t, fmt.Sprintf("rms normu %d×%d", R, D), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, gm := twin(xv, true, R, D)[c], twin(gv, true, D)[c]
			return tensor.RMSNorm(x, gm, 1e-5), []*tensor.Tensor{x, gm}
		})
		compare(t, fmt.Sprintf("katman normu %d×%d", R, D), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, gm, bt := twin(xv, true, R, D)[c], twin(gv, true, D)[c], twin(bv, true, D)[c]
			return tensor.LayerNorm(x, gm, bt, 1e-5), []*tensor.Tensor{x, gm, bt}
		})
		compare(t, fmt.Sprintf("ölçeksiz katman normu %d×%d", R, D), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(xv, true, R, D)[c]
			return tensor.LayerNorm(x, nil, nil, 1e-5), []*tensor.Tensor{x}
		})
	}
}

func TestEmbedding(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	V, D := 50, 13
	wv := random(rng, V*D, 1)
	idx := make([]int, 37)
	for i := range idx {
		idx[i] = rng.Intn(8) // yinelenen indeksler
	}
	compare(t, "gömme", 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		w := twin(wv, true, V, D)[c]
		return tensor.Embedding(w, idx, 37), []*tensor.Tensor{w}
	})
}

// TestAttention16Bit bfloat16 ve float16 q, k, v ile dikkatin ileri ve geri yönünü denetler
// (geri yön 16 bitlik çıktıyı float32'ye açarken bir kopya çekirdeği başlatır).
func TestAttention16Bit(t *testing.T) {
	rng := rand.New(rand.NewSource(18))
	qv, kv, vv := random(rng, 2*2*5*4, 1), random(rng, 2*2*5*4, 1), random(rng, 2*2*5*4, 1)
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		compare(t, "dikkat "+dt.String(), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q, k, v := twin(qv, true, 2, 2, 5, 4)[c], twin(kv, true, 2, 2, 5, 4)[c], twin(vv, true, 2, 2, 5, 4)[c]
			y := tensor.Attention(q.Cast(dt), k.Cast(dt), v.Cast(dt), tensor.AttentionOptions{Causal: true})
			if y.DType() != dt {
				t.Fatalf("dikkat çıktısı %s olmalıydı, %s", dt, y.DType())
			}
			return y.Cast(tensor.Float32), []*tensor.Tensor{q, k, v}
		})
	}
}

func TestAttention(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	cases := []struct {
		ad                        string
		B, H, Hkv, Tq, Tk, Dh, Dv int
		opt                       tensor.AttentionOptions
		mask2                     bool
	}{
		{"nedensel", 2, 2, 2, 7, 7, 4, 4, tensor.AttentionOptions{Causal: true}, false},
		{"tam", 1, 2, 2, 5, 9, 3, 6, tensor.AttentionOptions{}, false},
		{"GQA nedensel", 1, 4, 2, 6, 6, 4, 4, tensor.AttentionOptions{Causal: true}, false},
		{"Tq<Tk nedensel", 1, 1, 1, 3, 8, 4, 4, tensor.AttentionOptions{Causal: true}, false},
		{"pencere", 1, 2, 1, 9, 9, 4, 4, tensor.AttentionOptions{Causal: true, Window: 3}, false},
		{"iki yönlü pencere", 1, 1, 1, 9, 9, 4, 4, tensor.AttentionOptions{Window: 2}, false},
		{"maske", 2, 2, 2, 5, 5, 4, 4, tensor.AttentionOptions{Scale: 0.3}, true},
		{"uzun satır", 1, 1, 1, 3, 300, 4, 4, tensor.AttentionOptions{}, false},
		// Birleşik çekirdekte birden çok sorgu/anahtar döşemesi, 16'nın katı olmayan boylar.
		{"çok döşeme nedensel", 1, 2, 1, 70, 70, 20, 12, tensor.AttentionOptions{Causal: true}, false},
		{"çok döşeme pencere", 1, 1, 1, 75, 75, 8, 8, tensor.AttentionOptions{Causal: true, Window: 20}, false},
		{"çok döşeme iki yönlü", 1, 1, 1, 66, 66, 8, 8, tensor.AttentionOptions{Window: 9}, false},
		{"Tq<Tk pencere", 1, 1, 1, 21, 90, 8, 8, tensor.AttentionOptions{Causal: true, Window: 30}, false},
		{"büyük baş boyu", 1, 1, 1, 20, 20, 100, 72, tensor.AttentionOptions{Causal: true}, false},
		{"matris yolu", 1, 1, 1, 5, 6, 300, 8, tensor.AttentionOptions{Causal: true}, false},
	}
	for _, d := range cases {
		qv := random(rng, d.B*d.H*d.Tq*d.Dh, 1)
		kv := random(rng, d.B*d.Hkv*d.Tk*d.Dh, 1)
		vv := random(rng, d.B*d.Hkv*d.Tk*d.Dv, 1)
		var mv []float32
		if d.mask2 {
			mv = make([]float32, d.Tq*d.Tk)
			for i := range mv {
				if rng.Intn(3) > 0 {
					mv[i] = 1
				}
			}
			for j := range d.Tk {
				mv[j] = 0 // ilk satır hiçbir anahtara bakamaz: çıktı ve gradyan 0
			}
		}
		compare(t, "dikkat "+d.ad, 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q := twin(qv, true, d.B, d.H, d.Tq, d.Dh)[c]
			k := twin(kv, true, d.B, d.Hkv, d.Tk, d.Dh)[c]
			v := twin(vv, true, d.B, d.Hkv, d.Tk, d.Dv)[c]
			opt := d.opt
			if d.mask2 {
				opt.Mask = twin(mv, false, d.Tq, d.Tk)[c]
			}
			return tensor.Attention(q, k, v, opt), []*tensor.Tensor{q, k, v}
		})
	}
	// Aynı q, k, v'den iki dikkat: ikinci geri yayılım gradyanları eskisine ekler.
	qv, kv, vv := random(rng, 2*20*8, 1), random(rng, 2*20*8, 1), random(rng, 2*20*8, 1)
	compare(t, "dikkat birikim", 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		q, k, v := twin(qv, true, 1, 2, 20, 8)[c], twin(kv, true, 1, 2, 20, 8)[c], twin(vv, true, 1, 2, 20, 8)[c]
		y := tensor.Add(tensor.Attention(q, k, v, tensor.AttentionOptions{Causal: true}), tensor.Attention(q, k, v, tensor.AttentionOptions{}))
		return y, []*tensor.Tensor{q, k, v}
	})
}

// TestNaNRowsMatchCPU tamamı NaN olan satırların softmax, log_softmax ve dikkatte (bütün dikkat
// yollarında: birleşik, matris çarpımı, tensör çekirdekli bf16) CPU'daki gibi NaN verdiğini,
// tamamen maskeli satırların ise 0 / -sonsuz kaldığını denetler; geri yön de karşılaştırılır.
func TestNaNRowsMatchCPU(t *testing.T) {
	nan, inf := float32(math.NaN()), float32(math.Inf(-1))
	rows := []float32{nan, nan, nan, inf, inf, inf, 1, nan, 2, 0.5, -1, 2}
	for _, lg := range []bool{false, true} {
		compare(t, fmt.Sprintf("NaN satırı softmax log=%v", lg), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(rows, true, 4, 3)[c]
			if lg {
				return x.LogSoftmax(1), []*tensor.Tensor{x}
			}
			return x.Softmax(1), []*tensor.Tensor{x}
		})
	}
	g := gpuBackend()
	old := g.attnPath
	defer func() { g.attnPath = old }()
	rng := rand.New(rand.NewSource(24))
	for _, d := range []struct {
		path string
		dt   tensor.DType
		Dh   int
	}{{"birlesik", tensor.Float32, 8}, {"carpim", tensor.Float32, 8}, {"tensor", tensor.BFloat16, 16}} {
		g.attnPath = d.path
		T := 5
		qv, kv, vv := random(rng, T*d.Dh, 1), random(rng, T*d.Dh, 1), random(rng, T*d.Dh, 1)
		for j := 0; j < d.Dh; j++ {
			qv[2*d.Dh+j] = nan // 2. sorgu satırının bütün puanları NaN
		}
		tol := 5e-5
		if d.dt != tensor.Float32 {
			tol = 2e-2
		}
		compare(t, "NaN satırı dikkat "+d.path, tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q, k, v := twin(qv, true, 1, 1, T, d.Dh)[c], twin(kv, true, 1, 1, T, d.Dh)[c], twin(vv, true, 1, 1, T, d.Dh)[c]
			return tensor.Attention(q.Cast(d.dt), k.Cast(d.dt), v.Cast(d.dt), tensor.AttentionOptions{Causal: true}).Cast(tensor.Float32),
				[]*tensor.Tensor{q, k, v}
		})
	}
}

// TestLinearCrossEntropy parçalı doğrusal + çapraz entropinin GPU'da CPU ile aynı kaybı ve x, w
// gradyanlarını verdiğini sınar: birden çok dilim (satır ofsetli çarpım), yok sayılan hedefler,
// gelen gradyanla ölçekleme (skalerle yayınlamalı çarpım); float32 ve bf16.
func TestLinearCrossEntropy(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	R, D, C := tensor.LinearCrossEntropyChunk+5, 8, 6
	xv, wv := random(rng, R*D, 1), random(rng, D*C, 1)
	target := make([]int, R)
	for i := range target {
		target[i] = rng.Intn(C)
		if i%7 == 3 {
			target[i] = -1
		}
	}
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
		tol := 2e-5
		if dt != tensor.Float32 {
			tol = 2e-2
		}
		compare(t, fmt.Sprintf("doğrusal çapraz entropi %s", dt), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, w := twin(xv, true, R, D)[c], twin(wv, true, D, C)[c]
			return tensor.LinearCrossEntropy(x.Cast(dt), w.Cast(dt), target), []*tensor.Tensor{x, w}
		})
	}
}

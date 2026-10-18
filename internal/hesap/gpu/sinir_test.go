package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// Sinir ağı işlemleri: matris çarpımı (devrikler, toplu, iki döşeme ayarı), softmax,
// çapraz entropi, normlar, gömme ve dikkat; ileri ve gradyanlar CPU ile karşılaştırılır.

func TestMatrisCarpimi(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	g := gpuArkaUc()
	defer g.carpimDosemesi(nil)
	for _, ay := range []carpimAyari{varsayilanCarpim, buyukCarpim, {32, 16, 8, 2, 4}} {
		g.carpimDosemesi(&ay)
		for _, d := range [][4]int{{1, 1, 1, 1}, {1, 3, 5, 7}, {2, 17, 13, 9}, {1, 64, 64, 64}, {3, 65, 31, 129}, {1, 130, 1, 70}, {1, 5, 0, 4}} {
			b, M, K, N := d[0], d[1], d[2], d[3]
			av, bv := rastgele(rng, b*M*K, 1), rastgele(rng, b*K*N, 1)
			ad := fmt.Sprintf("döşeme %v, %d×(%d×%d)·(%d×%d)", ay, b, M, K, K, N)
			karsilastir(t, ad, 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x, y := ikiz(av, true, b, M, K)[c], ikiz(bv, true, b, K, N)[c]
				return tensor.MatMul(x, y), []*tensor.Tensor{x, y}
			})
			karsilastir(t, ad+" devrikli", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x, y := ikiz(av, true, b, K, M)[c], ikiz(bv, true, b, N, K)[c]
				return tensor.MatMul(x.Transpose(1, 2), y.Transpose(1, 2)), []*tensor.Tensor{x, y}
			})
		}
	}
	xv, wv := rastgele(rng, 2*5*6, 1), rastgele(rng, 6*4, 1)
	karsilastir(t, "doğrusal katman (3B @ 2B)", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, w := ikiz(xv, true, 2, 5, 6)[c], ikiz(wv, true, 6, 4)[c]
		return tensor.MatMul(x, w), []*tensor.Tensor{x, w}
	})
}

func TestSoftmax(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	for _, sh := range [][]int{{1, 1}, {3, 7}, {2, 300}, {5, 4, 3}, {1, 1031}} {
		v := rastgele(rng, numel(sh), 3)
		for ax := range sh {
			for _, lg := range []bool{false, true} {
				karsilastir(t, fmt.Sprintf("softmax %v eksen %d log=%v", sh, ax, lg), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := ikiz(v, true, sh...)[c]
					if lg {
						return x.LogSoftmax(ax), []*tensor.Tensor{x}
					}
					return x.Softmax(ax), []*tensor.Tensor{x}
				})
			}
		}
	}
}

func TestCaprazEntropi(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	for _, d := range [][2]int{{1, 2}, {5, 7}, {9, 300}, {4, 1031}} {
		R, C := d[0], d[1]
		v := rastgele(rng, R*C, 3)
		hedef := make([]int, R)
		for i := range hedef {
			hedef[i] = rng.Intn(C)
		}
		if R > 2 {
			hedef[1] = -1 // yok sayılan satır
		}
		karsilastir(t, fmt.Sprintf("çapraz entropi %d×%d", R, C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(v, true, R, C)[c]
			return tensor.CrossEntropy(x, hedef), []*tensor.Tensor{x}
		})
	}
}

func TestNormlar(t *testing.T) {
	rng := rand.New(rand.NewSource(10))
	for _, d := range [][2]int{{1, 1}, {3, 8}, {5, 129}, {2, 600}} {
		R, D := d[0], d[1]
		xv, gv, bv := rastgele(rng, R*D, 2), rastgele(rng, D, 1), rastgele(rng, D, 1)
		karsilastir(t, fmt.Sprintf("rms normu %d×%d", R, D), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, gm := ikiz(xv, true, R, D)[c], ikiz(gv, true, D)[c]
			return tensor.RMSNorm(x, gm, 1e-5), []*tensor.Tensor{x, gm}
		})
		karsilastir(t, fmt.Sprintf("katman normu %d×%d", R, D), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, gm, bt := ikiz(xv, true, R, D)[c], ikiz(gv, true, D)[c], ikiz(bv, true, D)[c]
			return tensor.LayerNorm(x, gm, bt, 1e-5), []*tensor.Tensor{x, gm, bt}
		})
		karsilastir(t, fmt.Sprintf("ölçeksiz katman normu %d×%d", R, D), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(xv, true, R, D)[c]
			return tensor.LayerNorm(x, nil, nil, 1e-5), []*tensor.Tensor{x}
		})
	}
}

func TestGomme(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	V, D := 50, 13
	wv := rastgele(rng, V*D, 1)
	idx := make([]int, 37)
	for i := range idx {
		idx[i] = rng.Intn(8) // yinelenen indeksler
	}
	karsilastir(t, "gömme", 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		w := ikiz(wv, true, V, D)[c]
		return tensor.Embedding(w, idx, 37), []*tensor.Tensor{w}
	})
}

func TestDikkat(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	durumlar := []struct {
		ad                        string
		B, H, Hkv, Tq, Tk, Dh, Dv int
		opt                       tensor.AttentionOptions
		maske                     bool
	}{
		{"nedensel", 2, 2, 2, 7, 7, 4, 4, tensor.AttentionOptions{Causal: true}, false},
		{"tam", 1, 2, 2, 5, 9, 3, 6, tensor.AttentionOptions{}, false},
		{"GQA nedensel", 1, 4, 2, 6, 6, 4, 4, tensor.AttentionOptions{Causal: true}, false},
		{"Tq<Tk nedensel", 1, 1, 1, 3, 8, 4, 4, tensor.AttentionOptions{Causal: true}, false},
		{"pencere", 1, 2, 1, 9, 9, 4, 4, tensor.AttentionOptions{Causal: true, Window: 3}, false},
		{"iki yönlü pencere", 1, 1, 1, 9, 9, 4, 4, tensor.AttentionOptions{Window: 2}, false},
		{"maske", 2, 2, 2, 5, 5, 4, 4, tensor.AttentionOptions{Scale: 0.3}, true},
		{"uzun satır", 1, 1, 1, 3, 300, 4, 4, tensor.AttentionOptions{}, false},
	}
	for _, d := range durumlar {
		qv := rastgele(rng, d.B*d.H*d.Tq*d.Dh, 1)
		kv := rastgele(rng, d.B*d.Hkv*d.Tk*d.Dh, 1)
		vv := rastgele(rng, d.B*d.Hkv*d.Tk*d.Dv, 1)
		var mv []float32
		if d.maske {
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
		karsilastir(t, "dikkat "+d.ad, 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q := ikiz(qv, true, d.B, d.H, d.Tq, d.Dh)[c]
			k := ikiz(kv, true, d.B, d.Hkv, d.Tk, d.Dh)[c]
			v := ikiz(vv, true, d.B, d.Hkv, d.Tk, d.Dv)[c]
			opt := d.opt
			if d.maske {
				opt.Mask = ikiz(mv, false, d.Tq, d.Tk)[c]
			}
			return tensor.Attention(q, k, v, opt), []*tensor.Tensor{q, k, v}
		})
	}
}

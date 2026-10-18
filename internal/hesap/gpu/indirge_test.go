package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// İndirgemeler: eksen toplamı ve ortalaması, hepsinin toplamı, en büyük değer ve
// indeksi; boş eksen, tek eleman, uzun ve kısa eksenler (iki çekirdek biçimi de).

func TestToplamlar(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	// {3000, 5}, {9000}, {2100, 300}: uzun indirgenen eksen, az çıktı (iki aşamalı indirgeme).
	sekiller := [][]int{{1}, {7}, {3, 5}, {4, 300}, {300, 4}, {2, 3, 70}, {5, 0, 3}, {2, 1031}, {1031, 3}, {600, 260},
		{3000, 5}, {9000}, {2100, 300}}
	for _, sh := range sekiller {
		v := rastgele(rng, numel(sh), 1)
		tol := 1e-5
		if numel(sh) > 5000 {
			tol = 1e-4 // binlerce float32 terimin toplamı (CPU float64 biriktirir)
		}
		for ax := range sh {
			for _, ort := range []bool{false, true} {
				if ort && sh[ax] == 0 {
					continue // boş eksenin ortalaması tanımsız (açık hata)
				}
				karsilastir(t, fmt.Sprintf("%v eksen %d ortalama=%v", sh, ax, ort), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := ikiz(v, true, sh...)[c]
					if ort {
						return x.Mean(ax, true), []*tensor.Tensor{x}
					}
					return x.Sum(ax, false), []*tensor.Tensor{x}
				})
			}
		}
		karsilastir(t, fmt.Sprintf("%v hepsi", sh), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(v, true, sh...)[c]
			return x.SumAll(), []*tensor.Tensor{x}
		})
		karsilastir(t, fmt.Sprintf("%v adımlı hepsi", sh), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(v, true, sh...)[c]
			if len(sh) < 2 {
				return x.MeanAll(), []*tensor.Tensor{x}
			}
			return x.Transpose(0, 1).SumAll(), []*tensor.Tensor{x}
		})
	}
}

// TestSapmaGradyani yayınlanan toplamanın (sapma) gradyanını uzun satır ekseninde denetler
// (iki aşamalı sütun indirgemesi).
func TestSapmaGradyani(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	xv, bv := rastgele(rng, 2500*40, 1), rastgele(rng, 40, 1)
	karsilastir(t, "sapma gradyanı", 1e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, b := ikiz(xv, true, 2500, 40)[c], ikiz(bv, true, 40)[c]
		return tensor.Mul(tensor.Add(x, b), b), []*tensor.Tensor{x, b}
	})
}

// TestBelirlenimci aynı girdinin her çalıştırmada bit düzeyinde aynı sonucu verdiğini
// denetler (indirgemeler, matris çarpımı, gradyanlar; atomik toplama yok).
func TestBelirlenimci(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	xv, wv := rastgele(rng, 300*70, 1), rastgele(rng, 70*90, 1)
	idx := make([]int, 300)
	for i := range idx {
		idx[i] = rng.Intn(5)
	}
	var onceki [][]float32
	for tekrar := 0; tekrar < 3; tekrar++ {
		x := tensor.NewOn(gpuCihaz, xv, 300, 70).SetRequiresGrad(true)
		w := tensor.NewOn(gpuCihaz, wv, 70, 90).SetRequiresGrad(true)
		y := tensor.MatMul(x, w)
		k := tensor.Add(y.Softmax(1).Sum(0, false).SumAll(), tensor.Embedding(w, idx).SumAll())
		k.Backward()
		simdi := [][]float32{k.Data(), x.Grad().Data(), w.Grad().Data()}
		if onceki != nil {
			for i := range simdi {
				farkDenetle(t, fmt.Sprintf("tekrar %d, çıktı %d", tekrar, i), simdi[i], onceki[i], 0)
			}
		}
		onceki = simdi
	}
}

func TestEnBuyuk(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	nan := float32(math.NaN())
	for _, sh := range [][]int{{1}, {9}, {4, 6}, {3, 4, 5}, {2, 300}} {
		v := rastgele(rng, numel(sh), 1)
		// Eşit en büyükler: ilk indeks kazanmalı.
		v[0] = 5
		if len(v) > 2 {
			v[2] = 5
		}
		for ax := range sh {
			karsilastir(t, fmt.Sprintf("en büyük %v eksen %d", sh, ax), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := ikiz(v, true, sh...)[c]
				return x.Max(ax, false), []*tensor.Tensor{x}
			})
			gi := tensor.NewOn(gpuCihaz, v, sh...).ArgMax(ax)
			ci := tensor.NewOn(tensor.CPU, v, sh...).ArgMax(ax)
			if fmt.Sprint(gi) != fmt.Sprint(ci) {
				t.Fatalf("en büyük indeks %v eksen %d: GPU %v, CPU %v", sh, ax, gi, ci)
			}
		}
	}
	// NaN en büyük sayılır, ilk NaN kazanır.
	v := []float32{1, nan, 3, nan, 2, 7}
	gi := tensor.NewOn(gpuCihaz, v, 2, 3).ArgMax(1)
	ci := tensor.NewOn(tensor.CPU, v, 2, 3).ArgMax(1)
	if fmt.Sprint(gi) != fmt.Sprint(ci) {
		t.Fatalf("NaN'lı en büyük indeks: GPU %v, CPU %v", gi, ci)
	}
}

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
	sekiller := [][]int{{1}, {7}, {3, 5}, {4, 300}, {300, 4}, {2, 3, 70}, {5, 0, 3}, {2, 1031}, {1031, 3}, {600, 260}}
	for _, sh := range sekiller {
		v := rastgele(rng, numel(sh), 1)
		for ax := range sh {
			for _, ort := range []bool{false, true} {
				if ort && sh[ax] == 0 {
					continue // boş eksenin ortalaması tanımsız (açık hata)
				}
				karsilastir(t, fmt.Sprintf("%v eksen %d ortalama=%v", sh, ax, ort), 1e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := ikiz(v, true, sh...)[c]
					if ort {
						return x.Mean(ax, true), []*tensor.Tensor{x}
					}
					return x.Sum(ax, false), []*tensor.Tensor{x}
				})
			}
		}
		karsilastir(t, fmt.Sprintf("%v hepsi", sh), 1e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(v, true, sh...)[c]
			return x.SumAll(), []*tensor.Tensor{x}
		})
		karsilastir(t, fmt.Sprintf("%v adımlı hepsi", sh), 1e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(v, true, sh...)[c]
			if len(sh) < 2 {
				return x.MeanAll(), []*tensor.Tensor{x}
			}
			return x.Transpose(0, 1).SumAll(), []*tensor.Tensor{x}
		})
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

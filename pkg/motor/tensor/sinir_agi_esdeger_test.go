package tensor

import (
	"math"
	"testing"

	ct "tr_dili/pkg/tensor"
)

// Bu test, otomatik türevle eğitilen sinir_ağı'nın eski elle yazılmış geri yayılımla
// (float64 matrisler) aynı öğrenmeyi yaptığını gösterir: aynı tohumdan başlayan iki model
// aynı devir sayısından sonra float32 duyarlığında aynı ağırlıklara ulaşır.

// eskiAg eski NeuralNetVal'in float64 uygulamasının kısaltılmış kopyasıdır.
type eskiAg struct {
	akt  string
	w, b [][][]float64
}

func (e *eskiAg) etkin(x float64, cikis bool) float64 {
	if cikis {
		if e.akt == "sigmoid" {
			return 1 / (1 + math.Exp(-x))
		}
		return x
	}
	switch e.akt {
	case "relu":
		if x > 0 {
			return x
		}
		return 0.01 * x
	case "sigmoid":
		return 1 / (1 + math.Exp(-x))
	}
	return math.Tanh(x)
}

func (e *eskiAg) turev(a, z float64, cikis bool) float64 {
	if cikis {
		if e.akt == "sigmoid" {
			return a * (1 - a)
		}
		return 1
	}
	switch e.akt {
	case "relu":
		if z > 0 {
			return 1
		}
		return 0.01
	case "sigmoid":
		return a * (1 - a)
	}
	return 1 - a*a
}

func carp(a, b [][]float64) [][]float64 {
	out := make([][]float64, len(a))
	for i := range a {
		out[i] = make([]float64, len(b[0]))
		for k := range b {
			for j := range b[0] {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

func devir(a [][]float64) [][]float64 {
	out := make([][]float64, len(a[0]))
	for j := range out {
		out[j] = make([]float64, len(a))
		for i := range a {
			out[j][i] = a[i][j]
		}
	}
	return out
}

func (e *eskiAg) adim(X, Y [][]float64, lr float64) {
	L := len(e.w)
	acts, zs := [][][]float64{X}, [][][]float64{}
	cur := X
	for l := 0; l < L; l++ {
		z := carp(cur, e.w[l])
		a := make([][]float64, len(z))
		for i := range z {
			a[i] = make([]float64, len(z[i]))
			for j := range z[i] {
				z[i][j] += e.b[l][0][j]
				a[i][j] = e.etkin(z[i][j], l == L-1)
			}
		}
		zs, acts, cur = append(zs, z), append(acts, a), a
	}
	pred := acts[L]
	delta := make([][]float64, len(pred))
	for i := range pred {
		delta[i] = make([]float64, len(pred[i]))
		for j := range pred[i] {
			delta[i][j] = 2 * (pred[i][j] - Y[i][j]) / float64(len(X)) * e.turev(pred[i][j], zs[L-1][i][j], true)
		}
	}
	gw, gb := make([][][]float64, L), make([][][]float64, L)
	for l := L - 1; l >= 0; l-- {
		gw[l] = carp(devir(acts[l]), delta)
		gb[l] = [][]float64{make([]float64, len(delta[0]))}
		for i := range delta {
			for j := range delta[i] {
				gb[l][0][j] += delta[i][j]
			}
		}
		if l > 0 {
			nd := carp(delta, devir(e.w[l]))
			for i := range nd {
				for j := range nd[i] {
					nd[i][j] *= e.turev(acts[l][i][j], zs[l-1][i][j], false)
				}
			}
			delta = nd
		}
	}
	for l := 0; l < L; l++ {
		for i := range e.w[l] {
			for j := range e.w[l][i] {
				e.w[l][i][j] -= lr * gw[l][i][j]
			}
		}
		for j := range e.b[l][0] {
			e.b[l][0][j] -= lr * gb[l][0][j]
		}
	}
}

func TestSinirAgiEskiGeriYayilimlaEsdeger(t *testing.T) {
	X := [][]float64{{0, 0}, {0, 1}, {1, 0}, {1, 1}}
	Y := [][]float64{{0, 1}, {1, 0}, {1, 0}, {0, 1}}
	for _, akt := range []string{"tanh", "relu", "sigmoid"} {
		yeni := yeniSinirAgi([]int{2, 5, 4, 2}, akt, 7)
		eski := &eskiAg{akt: akt}
		for l := range yeni.agirliklar {
			eski.w = append(eski.w, matrisVerisi(yeni.agirliklar[l]))
			eski.b = append(eski.b, matrisVerisi(yeni.sapmalar[l]))
		}
		const devirSayisi, lr = 40, 0.2
		for i := 0; i < devirSayisi; i++ {
			eski.adim(X, Y, lr)
		}
		x := ct.New([]float32{0, 0, 0, 1, 1, 0, 1, 1}, 4, 2)
		y := ct.New([]float32{0, 1, 1, 0, 1, 0, 0, 1}, 4, 2)
		if _, err := yeni.egit(x, y, devirSayisi, lr); err != nil {
			t.Fatal(err)
		}
		for l := range yeni.agirliklar {
			for k, pair := range [][2][][]float64{{matrisVerisi(yeni.agirliklar[l]), eski.w[l]}, {matrisVerisi(yeni.sapmalar[l]), eski.b[l]}} {
				for i := range pair[0] {
					for j := range pair[0][i] {
						if d := math.Abs(pair[0][i][j] - pair[1][i][j]); d > 1e-4 {
							t.Fatalf("%s: %d. katman %s[%d][%d]: yeni %v, eski %v", akt, l, []string{"ağırlık", "sapma"}[k], i, j, pair[0][i][j], pair[1][i][j])
						}
					}
				}
			}
		}
	}
}

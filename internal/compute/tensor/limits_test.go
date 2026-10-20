package tensor

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bulgu 13 ve 14: tanımsız sonuçlar sessizce 0 dönmez: boş tensörün/boş eksenin ortalaması
// ve bütün hedefleri yok sayılan çapraz entropi açık hata verir; 0 boyutlu girdide
// LayerNorm/RMSNorm biçimli bir hata verir (ham indeks paniği değil).
func TestUndefinedResultsError(t *testing.T) {
	panicCode(t, "MeanAll(boş)", messages.H3064, func() { Zeros(0).MeanAll() })
	panicCode(t, "Mean(boş eksen)", messages.H3063, func() { Zeros(3, 0).Mean(1, false) })
	// Boş olmayan eksenin boş tensördeki ortalaması tanımlıdır (sonuç boş).
	if n := Zeros(3, 0).Mean(0, false).Numel(); n != 0 {
		t.Fatalf("[3,0] eksen 0 ortalaması %d elemanlı", n)
	}
	if v := Zeros(0).SumAll().Item(); v != 0 {
		t.Fatalf("boş toplam %v", v)
	}
	logit := New([]float32{1, 2, 3, 4}, 2, 2)
	panicCode(t, "CE yok sayılan", messages.H3047, func() { CrossEntropy(logit, []int{-1, -1}) })
	panicCode(t, "CE C=0", messages.H3047, func() { CrossEntropy(Zeros(2, 0), nil) })
	if v := CrossEntropy(logit, []int{-1, 1}).Item(); v != v || v <= 0 {
		t.Fatalf("tek geçerli hedefle kayıp %v", v)
	}
	panicCode(t, "LayerNorm 0 boyut", messages.H3048, func() { LayerNorm(Scalar(1), nil, nil, 1e-5) })
	panicCode(t, "RMSNorm 0 boyut", messages.H3048, func() { RMSNorm(Scalar(1), nil, 1e-5) })
}

// ============================================================================
// Gradyan kapısı: gömme, çapraz entropi, katman ve RMS normu
// ============================================================================

func init() {
	embIdx := []int{1, 3, 1, 0, 3, 3}
	ceTargets := []int{2, -1, 0, 3, -1, 1}
	layerNorm := func(hasG bool) refFn {
		return func(in [][]float64) []float64 {
			x, res := in[0], make([]float64, len(in[0]))
			for r := 0; r < 3; r++ {
				row := x[r*5 : r*5+5]
				mu := sum64(row) / 5
				var v float64
				for _, e := range row {
					v += (e - mu) * (e - mu)
				}
				rs := 1 / math.Sqrt(v/5+1e-5)
				for d, e := range row {
					y := (e - mu) * rs
					if hasG {
						y = y*in[1][d] + in[2][d]
					}
					res[r*5+d] = y
				}
			}
			return res
		}
	}
	rmsNorm := func(hasG bool) refFn {
		return func(in [][]float64) []float64 {
			x, res := in[0], make([]float64, len(in[0]))
			for r := 0; r < 3; r++ {
				row := x[r*5 : r*5+5]
				var ss float64
				for _, e := range row {
					ss += e * e
				}
				rs := 1 / math.Sqrt(ss/5+1e-6)
				for d, e := range row {
					res[r*5+d] = e * rs
					if hasG {
						res[r*5+d] *= in[1][d]
					}
				}
			}
			return res
		}
	}
	registerGrad("limits",
		gradSpec{name: "embedding", ops: []string{"embedding"},
			build:  func(xs []*Tensor) *Tensor { return Embedding(xs[0], embIdx, 2, 3) },
			inputs: genInputs(gen(-1, 1, 0, 4, 3)),
			ref: func(in [][]float64) []float64 {
				var res []float64
				for _, ix := range embIdx {
					res = append(res, in[0][ix*3:ix*3+3]...)
				}
				return res
			}},
		gradSpec{name: "cross_entropy yok sayma", ops: []string{"cross_entropy"},
			build:  func(xs []*Tensor) *Tensor { return CrossEntropy(xs[0], ceTargets) },
			inputs: genInputs(gen(-2, 2, 0, 2, 3, 4)),
			ref: func(in [][]float64) []float64 {
				var loss float64
				valid := 0
				for r, tg := range ceTargets {
					if tg < 0 {
						continue
					}
					row := in[0][r*4 : r*4+4]
					loss += logSumExp64(row) - row[tg]
					valid++
				}
				return []float64{loss / float64(valid)}
			}},
		gradSpec{name: "layer_norm γβ", ops: []string{"layer_norm"},
			build:  func(xs []*Tensor) *Tensor { return LayerNorm(xs[0], xs[1], xs[2], 1e-5) },
			inputs: genInputs(gen(-2, 2, 0, 3, 5), gen(-1, 1, 0, 5), gen(-1, 1, 0, 5)), ref: layerNorm(true)},
		gradSpec{name: "layer_norm yalın", ops: []string{"layer_norm"},
			build:  func(xs []*Tensor) *Tensor { return LayerNorm(xs[0], nil, nil, 1e-5) },
			inputs: genInputs(gen(-2, 2, 0, 3, 5)), ref: layerNorm(false)},
		gradSpec{name: "rms_norm γ", ops: []string{"rms_norm"},
			build:  func(xs []*Tensor) *Tensor { return RMSNorm(xs[0], xs[1], 1e-6) },
			inputs: genInputs(gen(-2, 2, 0.2, 3, 5), gen(-1, 1, 0, 5)), ref: rmsNorm(true)},
		gradSpec{name: "rms_norm yalın", ops: []string{"rms_norm"},
			build:  func(xs []*Tensor) *Tensor { return RMSNorm(xs[0], nil, 1e-6) },
			inputs: genInputs(gen(-2, 2, 0.2, 3, 5)), ref: rmsNorm(false)})
}

func TestGradLimits(t *testing.T) { runGradGroup(t, "limits") }

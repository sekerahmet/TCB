package tensor

import (
	"math"
	"testing"
)

// Birleştirme girdilerin bitlerini korur: işaretli sıfır (-0) +0'a dönmez; 16 bitlik çıktıda
// da (float32 tamponda toplanıp bir kez yuvarlanır).
func TestConcatKeepsSignedZero(t *testing.T) {
	neg := float32(math.Copysign(0, -1))
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		a, b := New([]float32{neg, 1}, 2).Cast(dt), New([]float32{neg, 0}, 2).Cast(dt)
		d := Concat([]*Tensor{a, b}, 0).Data()
		for i, want := range []bool{true, false, true, false} {
			if math.Signbit(float64(d[i])) != want {
				t.Fatalf("%s: %v elemanlarının işaretleri yanlış (%d. eleman)", dt, d, i)
			}
		}
	}
}

// Fill(-0) işaret bitini korur (float32 ve 16 bitlik depolamada aynı).
func TestFillNegativeZero(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		s := defaultBackend().Alloc(3, dt)
		defaultBackend().Fill(s, float32(math.Copysign(0, -1)))
		for _, v := range fromStorage(defaultBackend(), s, []int{3}).Data() {
			if !math.Signbit(float64(v)) {
				t.Fatalf("%s: Fill(-0) işareti kaybetti", dt)
			}
		}
	}
}

// ============================================================================
// Gradyan kapısı: birleştirme
// ============================================================================

func init() {
	registerGrad("concat",
		gradSpec{name: "concat eksen 1", ops: []string{"concat"},
			build:  func(xs []*Tensor) *Tensor { return Concat(xs, 1) },
			inputs: genInputs(gen(-1, 1, 0, 2, 3), gen(-1, 1, 0, 2, 2), gen(-1, 1, 0, 2, 1)),
			ref: func(in [][]float64) []float64 {
				var res []float64
				for r := 0; r < 2; r++ {
					res = append(res, in[0][r*3:r*3+3]...)
					res = append(res, in[1][r*2:r*2+2]...)
					res = append(res, in[2][r:r+1]...)
				}
				return res
			}},
		gradSpec{name: "concat eksen 0", ops: []string{"concat"},
			build:  func(xs []*Tensor) *Tensor { return Concat(xs, 0) },
			inputs: genInputs(gen(-1, 1, 0, 1, 3), gen(-1, 1, 0, 2, 3)),
			ref: func(in [][]float64) []float64 {
				return append(append([]float64(nil), in[0]...), in[1]...)
			}})
}

func TestGradConcat(t *testing.T) { runGradGroup(t, "concat") }

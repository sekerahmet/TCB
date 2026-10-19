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

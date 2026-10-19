package ml

import (
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Yazdırma ve indirgeme hız ölçümleri: go test -bench . ./internal/stdlib/ml/

func BenchmarkPrintLarge(b *testing.B) {
	d := New(tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = d.Inspect()
	}
}

func BenchmarkMaxAll(b *testing.B) {
	t := tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = t.Reshape(-1).Max(0, false)
	}
}

func BenchmarkMinAll(b *testing.B) {
	t := tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = minAll(t)
	}
}

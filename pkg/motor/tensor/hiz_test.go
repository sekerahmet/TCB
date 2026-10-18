package tensor

import (
	"math/rand"
	"testing"

	ct "tr_dili/pkg/tensor"
)

// Yazdırma ve indirgeme hız ölçümleri: go test -bench . ./pkg/motor/tensor/

func BenchmarkYazdirBuyuk(b *testing.B) {
	d := Yeni(ct.Randn(rand.New(rand.NewSource(1)), 1000, 1000))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = d.Inspect()
	}
}

func BenchmarkMaksTum(b *testing.B) {
	t := ct.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = t.Reshape(-1).Max(0, false)
	}
}

func BenchmarkMinTum(b *testing.B) {
	t := ct.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = enKucukTum(t)
	}
}

package tensor

import (
	"fmt"
	"math/rand"
	"testing"

	"tr_dili/pkg/vm"
)

// Başarım ölçümleri. Mevcut dil içi matris çarpımı (pkg/vm MatrixVal.MatMul,
// [][]float64) ile aynı boyutlarda karşılaştırma içerir:
//
//	go test ./pkg/tensor/ -run '^$' -bench . -benchtime 5x

func benchMatMul(b *testing.B, n int) {
	rng := rand.New(rand.NewSource(1))
	x := Randn(rng, n, n)
	y := Randn(rng, n, n)
	b.SetBytes(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y)
	}
	b.ReportMetric(2*float64(n)*float64(n)*float64(n)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOPS")
}

func BenchmarkMatMul512(b *testing.B)  { benchMatMul(b, 512) }
func BenchmarkMatMul1024(b *testing.B) { benchMatMul(b, 1024) }

func benchVMMatMul(b *testing.B, n int) {
	rng := rand.New(rand.NewSource(1))
	x, y := vm.NewMatrix(n, n), vm.NewMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			x.Data[i][j] = rng.NormFloat64()
			y.Data[i][j] = rng.NormFloat64()
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := x.MatMul(y); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(2*float64(n)*float64(n)*float64(n)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOPS")
}

func BenchmarkVMMatMul512(b *testing.B)  { benchVMMatMul(b, 512) }
func BenchmarkVMMatMul1024(b *testing.B) { benchVMMatMul(b, 1024) }

// İleri + geri matris çarpımı (eğitimdeki tipik maliyet: 3 çarpım).
func BenchmarkMatMulIleriGeri1024(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	x := Randn(rng, 1024, 1024).SetRequiresGrad(true)
	y := Randn(rng, 1024, 1024).SetRequiresGrad(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(x, y).SumAll().Backward()
	}
}

// Toplu çarpım: 8 baş × (256×64)·(64×256), dikkat puanları gibi.
func BenchmarkMatMulToplu(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	q := Randn(rng, 8, 256, 64)
	k := Randn(rng, 8, 64, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatMul(q, k)
	}
}

func BenchmarkSoftmax(b *testing.B) {
	for _, d := range [][2]int{{1024, 1024}, {256, 32000}} {
		b.Run(fmt.Sprintf("%dx%d", d[0], d[1]), func(b *testing.B) {
			x := Randn(rand.New(rand.NewSource(1)), d[0], d[1])
			b.SetBytes(int64(d[0] * d[1] * 4))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x.Softmax(-1)
			}
		})
	}
}

func BenchmarkCrossEntropyIleriGeri(b *testing.B) {
	x := Randn(rand.New(rand.NewSource(1)), 256, 32000).SetRequiresGrad(true)
	hedef := make([]int, 256)
	for i := range hedef {
		hedef[i] = i * 7 % 32000
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CrossEntropy(x, hedef).Backward()
	}
}

// Eşdeğerlik: aynı girdilerde iki çarpım aynı sonucu vermeli (float32 toleransı).
func TestVMMatMulIleEsdeger(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	n := 70
	x, y := vm.NewMatrix(n, n), vm.NewMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			x.Data[i][j] = rng.NormFloat64()
			y.Data[i][j] = rng.NormFloat64()
		}
	}
	r, err := x.MatMul(y)
	if err != nil {
		t.Fatal(err)
	}
	flat := func(m *vm.MatrixVal) []float64 {
		out := make([]float64, 0, n*n)
		for _, row := range m.Data {
			out = append(out, row...)
		}
		return out
	}
	got := MatMul(FromFloat64(flat(x), n, n), FromFloat64(flat(y), n, n)).Data()
	want := flat(r.(*vm.MatrixVal))
	w32 := make([]float32, len(want))
	for i, v := range want {
		w32[i] = float32(v)
	}
	yakin(t, "vm eşdeğerlik", got, w32, 1e-4)
}

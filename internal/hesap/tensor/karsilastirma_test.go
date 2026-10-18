package tensor

import (
	"fmt"
	"math/rand"
	"testing"
)

// Başarım ölçümleri:
//
//	go test ./internal/hesap/tensor/ -run '^$' -bench . -benchtime 5x

// Eşdeğerlik: çarpım, float64 ile yapılan düz üçlü döngüyle aynı sonucu vermeli
// (float32 toleransı).
func TestMatMulFloat64IleEsdeger(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	n := 70
	x, y := make([]float64, n*n), make([]float64, n*n)
	for i := range x {
		x[i], y[i] = rng.NormFloat64(), rng.NormFloat64()
	}
	want := make([]float32, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			s := 0.0
			for k := 0; k < n; k++ {
				s += x[i*n+k] * y[k*n+j]
			}
			want[i*n+j] = float32(s)
		}
	}
	got := MatMul(FromFloat64(x, n, n), FromFloat64(y, n, n)).Data()
	yakin(t, "float64 eşdeğerlik", got, want, 1e-4)
}

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

package tensor

import (
	"math/rand"
	"testing"
)

// linearCECase LinearCrossEntropy'yi CrossEntropy(MatMul(x, w)) ile karşılaştırır: kayıp ve x
// ile w'nin gradyanları (gelen gradyan scale).
func linearCECase(t *testing.T, name string, R, D, C int, targets []int, scale float32, dt DType, tol float64) {
	t.Helper()
	rng := rand.New(rand.NewSource(int64(R*31 + C)))
	x0, w0 := Randn(rng, R, D), Randn(rng, D, C)
	run := func(fused bool) (loss, gx, gw []float32) {
		x := x0.Clone().SetRequiresGrad(true)
		w := w0.Clone().SetRequiresGrad(true)
		xc, wc := x.Cast(dt), w.Cast(dt)
		var l *Tensor
		if fused {
			l = LinearCrossEntropy(xc, wc, targets)
		} else {
			l = CrossEntropy(MatMulOut(xc, wc, dt), targets)
		}
		l.MulScalar(scale).Backward()
		return l.Data(), x.Grad().Data(), w.Grad().Data()
	}
	lf, xf, wf := run(true)
	lr, xr, wr := run(false)
	near(t, name+": kayıp", lf, lr, tol)
	near(t, name+": x gradyanı", xf, xr, tol)
	near(t, name+": w gradyanı", wf, wr, tol)
}

func TestLinearCrossEntropyMatchesCrossEntropyOfMatMul(t *testing.T) {
	tg := func(R, C int, ignoreFrom, ignoreTo int) []int {
		out := make([]int, R)
		for i := range out {
			out[i] = (i*7 + 3) % C
			if i >= ignoreFrom && i < ignoreTo {
				out[i] = -1
			}
			if i%5 == 4 {
				out[i] = -1
			}
		}
		return out
	}
	linearCECase(t, "tek dilim", 37, 8, 11, tg(37, 11, 0, 0), 1, Float32, 1e-5)
	// Üç dilim; ikincisinin bütün hedefleri yok sayılır (dilim atlanır, ortalama doğru kalır).
	n := 2*LinearCrossEntropyChunk + 3
	linearCECase(t, "çok dilim", n, 4, 3, tg(n, 3, LinearCrossEntropyChunk, 2*LinearCrossEntropyChunk), 0.5, Float32, 1e-5)
	linearCECase(t, "bf16", 64, 16, 32, tg(64, 32, 0, 0), 1, BFloat16, 2e-2)
}

// Türevsiz kipte ya da gradyan istemeyen girdilerle sonuç çizgeye bağlanmaz ve gradyan
// hesaplanmaz; ağırlığın şekli uyumsuzsa açık hata verir.
func TestLinearCrossEntropyNoGradAndShapes(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	x, w := Randn(rng, 5, 4), Randn(rng, 4, 6).SetRequiresGrad(true)
	var l *Tensor
	NoGrad(func() { l = LinearCrossEntropy(x, w, []int{0, 1, 2, 3, 4}) })
	if l.RequiresGrad() {
		t.Fatal("türevsiz kipte kayıp çizgeye bağlanmamalı")
	}
	want := CrossEntropy(MatMul(x, w.Detach()), []int{0, 1, 2, 3, 4}).Data()
	near(t, "türevsiz kayıp", l.Data(), want, 1e-6)
	defer func() {
		if recover() == nil {
			t.Fatal("uyumsuz ağırlık şekli hata vermedi")
		}
	}()
	LinearCrossEntropy(x, Randn(rng, 5, 6), []int{0, 1, 2, 3, 4})
}

// lceTargets gradyan kapısı kaydının hedefleridir (biri yok sayılır).
var lceTargets = []int{1, -1, 4}

func init() {
	registerGrad("linear_ce",
		gradSpec{name: "linear_cross_entropy", ops: []string{"linear_cross_entropy"},
			build:  func(xs []*Tensor) *Tensor { return LinearCrossEntropy(xs[0], xs[1], lceTargets) },
			inputs: genInputs(gen(-2, 2, 0, 3, 4), gen(-1, 1, 0, 4, 5)),
			ref: func(in [][]float64) []float64 {
				x, w := in[0], in[1]
				var loss float64
				valid := 0
				for r, tg := range lceTargets {
					if tg < 0 {
						continue
					}
					row := make([]float64, 5)
					for c := range row {
						for k := 0; k < 4; k++ {
							row[c] += x[r*4+k] * w[k*5+c]
						}
					}
					loss += logSumExp64(row) - row[tg]
					valid++
				}
				return []float64{loss / float64(valid)}
			}})
}

func TestGradLinearCE(t *testing.T) { runGradGroup(t, "linear_ce") }

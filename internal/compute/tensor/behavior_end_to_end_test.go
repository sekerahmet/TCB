package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Uçtan uca öğrenme deneyleri.

// İki katmanlı bir ağ (2 → 16 tanh → 2 logit) AdamW ile XOR'u öğrenmeli.
func TestXORAdamW(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	X := New([]float32{0, 0, 0, 1, 1, 0, 1, 1}, 4, 2)
	target := []int{0, 1, 1, 0}
	W1 := Randn(rng, 2, 16).MulScalar(0.5).SetRequiresGrad(true)
	b1 := Zeros(16).SetRequiresGrad(true)
	W2 := Randn(rng, 16, 2).MulScalar(0.5).SetRequiresGrad(true)
	b2 := Zeros(2).SetRequiresGrad(true)
	params := []*Tensor{W1, b1, W2, b2}
	opt := NewAdamW(params, 0.05, 0.01)
	forward := func() *Tensor {
		h := Add(MatMul(X, W1), b1).Tanh()
		return Add(MatMul(h, W2), b2)
	}
	var firstLoss, lossVal float32
	for step := 0; step < 300; step++ {
		opt.ZeroGrad()
		l := CrossEntropy(forward(), target)
		if step == 0 {
			firstLoss = l.Item()
		}
		lossVal = l.Item()
		l.Backward()
		opt.Step()
	}
	if lossVal > 0.05 || lossVal >= firstLoss {
		t.Fatalf("XOR öğrenilemedi: ilk kayıp %v, son kayıp %v", firstLoss, lossVal)
	}
	var estimate []int
	NoGrad(func() { estimate = forward().ArgMax(-1) })
	for i := range target {
		if estimate[i] != target[i] {
			t.Fatalf("XOR tahmini %v, beklenen %v", estimate, target)
		}
	}
	if opt.StepCount() != 300 {
		t.Fatal("adım sayacı yanlış")
	}
}

// Küçük doğrusal regresyon: y = X·w* + b*, AdamW ve momentumlu SGD ile.
func TestLinearRegression(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	X := Randn(rng, 64, 3)
	wActual := New([]float32{2, -3, 0.5}, 3, 1)
	y := Add(MatMul(X, wActual), Scalar(1.25))
	for _, ad := range []string{"adamw", "sgd"} {
		w := Zeros(3, 1).SetRequiresGrad(true)
		b := Zeros(1).SetRequiresGrad(true)
		var opt Optimizer
		if ad == "adamw" {
			opt = NewAdamW([]*Tensor{w, b}, 0.05, 0)
		} else {
			opt = NewSGD([]*Tensor{w, b}, 0.05, 0.9)
		}
		for step := 0; step < 500; step++ {
			opt.ZeroGrad()
			lossVal := Sub(Add(MatMul(X, w), b), y).Square().MeanAll()
			lossVal.Backward()
			opt.Step()
		}
		near(t, ad+" ağırlıklar", w.Data(), []float32{2, -3, 0.5}, 1e-2)
		near(t, ad+" sapma", b.Data(), []float32{1.25}, 1e-2)
	}
}

func TestAdamWWeightDecay(t *testing.T) {
	// Gradyan sıfırken yalnızca ayrık ağırlık çürümesi etki etmeli:
	// p ← p·(1 - lr·wd).
	p := New([]float32{2, -4}, 2).SetRequiresGrad(true)
	p.n.accGrad()
	opt := NewAdamW([]*Tensor{p}, 0.1, 0.5)
	opt.Step()
	near(t, "çürüme", p.Data(), []float32{2 * 0.95, -4 * 0.95}, 1e-6)
}

func TestClipGradNorm(t *testing.T) {
	a := New([]float32{0, 0}, 2).SetRequiresGrad(true)
	Mul(a, New([]float32{3, 4}, 2)).SumAll().Backward()
	n := ClipGradNorm([]*Tensor{a}, 1)
	if math.Abs(n-5) > 1e-6 {
		t.Fatalf("norm %v", n)
	}
	near(t, "kırpılmış", a.Grad().Data(), []float32{0.6, 0.8}, 1e-6)
	expectPanic(t, "gradyansız parametre", func() { NewAdamW([]*Tensor{Ones(2)}, 0.1, 0) })
	expectPanic(t, "görünüm parametre", func() {
		NewSGD([]*Tensor{Ones(2, 2).SetRequiresGrad(true).Detach().SetRequiresGrad(true).Slice(0, 1, 2)}, 0.1, 0)
	})
}

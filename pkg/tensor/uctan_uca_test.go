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
	hedef := []int{0, 1, 1, 0}
	W1 := Randn(rng, 2, 16).MulScalar(0.5).SetRequiresGrad(true)
	b1 := Zeros(16).SetRequiresGrad(true)
	W2 := Randn(rng, 16, 2).MulScalar(0.5).SetRequiresGrad(true)
	b2 := Zeros(2).SetRequiresGrad(true)
	params := []*Tensor{W1, b1, W2, b2}
	opt := NewAdamW(params, 0.05, 0.01)
	ileri := func() *Tensor {
		h := Add(MatMul(X, W1), b1).Tanh()
		return Add(MatMul(h, W2), b2)
	}
	var ilkKayip, kayip float32
	for adim := 0; adim < 300; adim++ {
		opt.ZeroGrad()
		l := CrossEntropy(ileri(), hedef)
		if adim == 0 {
			ilkKayip = l.Item()
		}
		kayip = l.Item()
		l.Backward()
		opt.Step()
	}
	if kayip > 0.05 || kayip >= ilkKayip {
		t.Fatalf("XOR öğrenilemedi: ilk kayıp %v, son kayıp %v", ilkKayip, kayip)
	}
	var tahmin []int
	NoGrad(func() { tahmin = ileri().ArgMax(-1) })
	for i := range hedef {
		if tahmin[i] != hedef[i] {
			t.Fatalf("XOR tahmini %v, beklenen %v", tahmin, hedef)
		}
	}
	if opt.StepCount() != 300 {
		t.Fatal("adım sayacı yanlış")
	}
}

// Küçük doğrusal regresyon: y = X·w* + b*, AdamW ve momentumlu SGD ile.
func TestDogrusalRegresyon(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	X := Randn(rng, 64, 3)
	wGercek := New([]float32{2, -3, 0.5}, 3, 1)
	y := Add(MatMul(X, wGercek), Scalar(1.25))
	for _, ad := range []string{"adamw", "sgd"} {
		w := Zeros(3, 1).SetRequiresGrad(true)
		b := Zeros(1).SetRequiresGrad(true)
		var opt Optimizer
		if ad == "adamw" {
			opt = NewAdamW([]*Tensor{w, b}, 0.05, 0)
		} else {
			opt = NewSGD([]*Tensor{w, b}, 0.05, 0.9)
		}
		for adim := 0; adim < 500; adim++ {
			opt.ZeroGrad()
			kayip := Sub(Add(MatMul(X, w), b), y).Square().MeanAll()
			kayip.Backward()
			opt.Step()
		}
		yakin(t, ad+" ağırlıklar", w.Data(), []float32{2, -3, 0.5}, 1e-2)
		yakin(t, ad+" sapma", b.Data(), []float32{1.25}, 1e-2)
	}
}

func TestAdamWAgirlikCurumesi(t *testing.T) {
	// Gradyan sıfırken yalnızca ayrık ağırlık çürümesi etki etmeli:
	// p ← p·(1 - lr·wd).
	p := New([]float32{2, -4}, 2).SetRequiresGrad(true)
	p.accGrad()
	opt := NewAdamW([]*Tensor{p}, 0.1, 0.5)
	opt.Step()
	yakin(t, "çürüme", p.Data(), []float32{2 * 0.95, -4 * 0.95}, 1e-6)
}

func TestClipGradNorm(t *testing.T) {
	a := New([]float32{0, 0}, 2).SetRequiresGrad(true)
	Mul(a, New([]float32{3, 4}, 2)).SumAll().Backward()
	n := ClipGradNorm([]*Tensor{a}, 1)
	if math.Abs(n-5) > 1e-6 {
		t.Fatalf("norm %v", n)
	}
	yakin(t, "kırpılmış", a.Grad().Data(), []float32{0.6, 0.8}, 1e-6)
	panikBekle(t, "gradyansız parametre", func() { NewAdamW([]*Tensor{Ones(2)}, 0.1, 0) })
	panikBekle(t, "görünüm parametre", func() {
		NewSGD([]*Tensor{Ones(2, 2).SetRequiresGrad(true).Detach().SetRequiresGrad(true).Slice(0, 1, 2)}, 0.1, 0)
	})
}

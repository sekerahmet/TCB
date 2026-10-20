package tensor

import (
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// squareCustom x² işleminin dış düğümle yazılmış biçimi: ileri türevsiz, geri 2·x·g.
func squareCustom(x *Tensor) *Tensor {
	var y *Tensor
	NoGrad(func() { y = x.Mul(x) })
	return CustomOp("kare", []*Tensor{x}, []*Tensor{y}, []*Tensor{x}, func(_ any, g []*Tensor) []*Tensor {
		return []*Tensor{g[0].Mul(x).MulScalar(2)}
	})[0]
}

func TestCustomOpSquareMatchesFused(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	x1 := Randn(rng, 3, 4).SetRequiresGrad(true)
	x2 := x1.Detach().Clone().SetRequiresGrad(true)
	squareCustom(x1).Mul(x1).SumAll().Backward() // x³'ün türevi: 3x²
	x2.Mul(x2).Mul(x2).SumAll().Backward()
	near(t, "kare gradyanı", x1.Grad().Data(), x2.Grad().Data(), 1e-5)
	checkGrad(t, "kare (sayısal)", func(xs []*Tensor) *Tensor { return squareCustom(xs[0]) }, Randn(rng, 2, 3))
}

func TestCustomOpSingleNode(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	y := squareCustom(x)
	if y.IsLeaf() || !y.RequiresGrad() {
		t.Fatal("dış işlemin sonucu çizgeye bağlanmalı")
	}
	if n := len(topoOrder(y)); n != 2 { // x ve y
		t.Fatalf("çizgede %d düğüm, 2 bekleniyordu", n)
	}
	// ileri hesap girdiyi olduğu gibi döndürse bile girdi yaprak kalır.
	z := CustomOp("özdeş", []*Tensor{x}, []*Tensor{x}, nil, func(_ any, g []*Tensor) []*Tensor { return g })[0]
	if !x.IsLeaf() || z == x || z.IsLeaf() {
		t.Fatal("çıktı girdiyle aynı tensör olmamalı; girdi yaprak kalmalı")
	}
	z.SumAll().Backward()
	near(t, "özdeş gradyanı", x.Grad().Data(), []float32{1, 1}, 0)
}

func TestCustomOpUntrackedInput(t *testing.T) {
	a := New([]float32{1, 2, 3}, 3).SetRequiresGrad(true)
	b := New([]float32{4, 5, 6}, 3)
	var y *Tensor
	NoGrad(func() { y = a.Mul(b) })
	called := false
	out := CustomOp("çarp", []*Tensor{a, b, nil}, []*Tensor{y}, []*Tensor{a, b}, func(_ any, g []*Tensor) []*Tensor {
		called = true
		if GradEnabled() {
			t.Error("geri çağrı türevsiz kipte çalışmalı")
		}
		// b ve tensör olmayan üçüncü girdi için verilen gradyanlar yok sayılır.
		return []*Tensor{g[0].Mul(b), g[0].Mul(a), Ones(7)}
	})[0]
	out.SumAll().Backward()
	if !called {
		t.Fatal("geri çağrı çalışmadı")
	}
	near(t, "a gradyanı", a.Grad().Data(), []float32{4, 5, 6}, 0)
	if b.Grad() != nil {
		t.Fatal("türev izlemeyen girdiye gradyan yazılmamalı")
	}

	// Hiçbir girdi gradyan istemiyorsa ya da türevsiz kipte çıktı olduğu gibi döner.
	if r := CustomOp("x", []*Tensor{b}, []*Tensor{y}, nil, func(any, []*Tensor) []*Tensor { return nil })[0]; r != y {
		t.Fatal("gradyan istemeyen girdide çıktı değişmemeli")
	}
	NoGrad(func() {
		if r := CustomOp("x", []*Tensor{a}, []*Tensor{y}, nil, func(any, []*Tensor) []*Tensor { return nil })[0]; r != y {
			t.Fatal("türevsiz kipte çıktı değişmemeli")
		}
	})
}

// sumDiff (a+b, a-b) iki çıktılı bir işlem; geri hesap kaç kez çağrıldığını sayar.
func sumDiff(a, b *Tensor, counter *int) (*Tensor, *Tensor) {
	var s, f *Tensor
	NoGrad(func() { s, f = a.Add(b), a.Sub(b) })
	outs := CustomOp("topla_fark", []*Tensor{a, b}, []*Tensor{s, f}, nil, func(_ any, g []*Tensor) []*Tensor {
		*counter++
		return []*Tensor{g[0].Add(g[1]), g[0].Sub(g[1])}
	})
	return outs[0], outs[1]
}

func TestCustomOpManyOutputs(t *testing.T) {
	a := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	b := New([]float32{3, 5}, 2).SetRequiresGrad(true)
	counter := 0
	s, f := sumDiff(a, b, &counter)
	// İki çıktı da kayba bağlı: L = Σ 2s + Σ 3f → dL/da = 5, dL/db = -1.
	s.MulScalar(2).SumAll().Add(f.MulScalar(3).SumAll()).Backward()
	if counter != 1 {
		t.Fatalf("geri hesap %d kez çağrıldı, bir kez bekleniyordu", counter)
	}
	near(t, "a", a.Grad().Data(), []float32{5, 5}, 0)
	near(t, "b", b.Grad().Data(), []float32{-1, -1}, 0)

	// Yalnızca ikinci çıktı kayba bağlı: birinci çıktının gradyanı sıfır tensördür.
	a.ZeroGrad()
	b.ZeroGrad()
	counter = 0
	_, f = sumDiff(a, b, &counter)
	f.Mul(f).SumAll().Backward() // dL/df = 2f = [-4, -6]
	near(t, "a (tek çıktı)", a.Grad().Data(), []float32{-4, -6}, 0)
	near(t, "b (tek çıktı)", b.Grad().Data(), []float32{4, 6}, 0)

	// Aynı çizgeden ikinci geri yayılım (diğer çıktı üzerinden) açık hatadır.
	s, f = sumDiff(a, b, &counter)
	f.SumAll().Backward()
	expectPanic(t, "çok çıktıda ikinci geri yayılım", func() { s.SumAll().Backward() })
}

func TestCustomOpSavedChangedIsError(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	c := New([]float32{3, 4}, 2)
	var y *Tensor
	NoGrad(func() { y = x.Mul(c) })
	out := CustomOp("ölçekle", []*Tensor{x}, []*Tensor{y}, []*Tensor{c}, func(_ any, g []*Tensor) []*Tensor {
		return []*Tensor{g[0].Mul(c)}
	})[0]
	c.Set(9, 0)
	err := panicError(func() { out.SumAll().Backward() })
	if m, ok := messages.As(err); !ok || m.Code != messages.H3012 || m.Args[0] != "ölçekle" {
		t.Fatalf("sürüm hatası beklenirdi, alınan: %v", err)
	}
}

func TestCustomOpWrongGrads(t *testing.T) {
	x := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	constant := func(grads ...*Tensor) *Tensor {
		var y *Tensor
		NoGrad(func() { y = x.MulScalar(2) })
		return CustomOp("yanlış", []*Tensor{x}, []*Tensor{y}, nil, func(any, []*Tensor) []*Tensor { return grads })[0]
	}
	panicCode(t, "şekil hatası", messages.H3032, func() { constant(Ones(3)).SumAll().Backward() })
	panicCode(t, "uzunluk hatası", messages.H3031, func() { constant(Ones(2, 3), nil).SumAll().Backward() })
}

func TestCustomOpContextPassed(t *testing.T) {
	x := New([]float32{1}, 1).SetRequiresGrad(true)
	var incoming any
	var y *Tensor
	NoGrad(func() { y = x.Clone() })
	out := CustomOp("bağlam", []*Tensor{x}, []*Tensor{y}, nil, func(ctx any, g []*Tensor) []*Tensor {
		incoming = ctx
		return g
	})[0]
	out.SumAll().BackwardCtx(nil, "görev")
	if incoming != "görev" {
		t.Fatalf("bağlam iletilmedi: %v", incoming)
	}
}

// Karışık türler: bf16 çıktı bir ara sonuçtur, gradyanı kendi türündedir (bf16); bf16
// yaprak girdinin gradyanı GradDType türündedir.
func TestCustomOpBF16(t *testing.T) {
	x := NewOf(BFloat16, []float32{1, 2}, 2).SetRequiresGrad(true)
	var y *Tensor
	NoGrad(func() { y = x.Mul(x) })
	out := CustomOp("kare16", []*Tensor{x}, []*Tensor{y}, []*Tensor{x}, func(_ any, g []*Tensor) []*Tensor {
		if g[0].DType() != BFloat16 {
			t.Errorf("çıktı gradyanı %s, bfloat16 bekleniyordu", g[0].DType())
		}
		return []*Tensor{g[0].Mul(x).MulScalar(2)}
	})[0]
	if out.DType() != BFloat16 {
		t.Fatalf("çıktı türü %s", out.DType())
	}
	out.Cast(Float32).SumAll().Backward()
	near(t, "bf16 gradyan", x.Grad().Data(), []float32{2, 4}, 0)
	if x.Grad().DType() != GradDType() {
		t.Fatalf("yaprak gradyanı %s, %s bekleniyordu", x.Grad().DType(), GradDType())
	}
}

// Bir dış düğümün geri hesabı sürerken aynı çizgenin düğümlerinden ikinci bir geri yayılım
// açık hata verir; ilk geri yayılım bozulmadan biter.
func TestCustomOpSecondBackwardDuringBackward(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	var k2 *Tensor
	var secondErr error
	o := CustomOp("özdeş", []*Tensor{x}, []*Tensor{x.Detach()}, nil, func(_ any, g []*Tensor) []*Tensor {
		secondErr = panicError(func() { k2.Backward() })
		return g
	})[0]
	k1 := o.MulScalar(3).SumAll()
	k2 = o.MulScalar(2).SumAll()
	k1.Backward()
	if !messages.Has(secondErr, messages.H3015) {
		t.Fatalf("ikinci geri yayılım hata vermedi: %v", secondErr)
	}
	near(t, "ilk geri yayılım", x.Grad().Data(), []float32{3, 3}, 0)
	// Hata veren geri yayılımın ayırdığı düğümlerin işareti kaldırıldı; o artık serbest
	// olduğu için k2 açıkça "zaten yapıldı" hatası verir.
	panicCode(t, "ikinci geri yayılım", messages.H3017, func() { k2.Backward() })
}

// Gradyan kapısı: kullanıcı tanımlı (dış düğümlü) işlem.
func init() {
	registerGrad("custom_op", gradSpec{name: "kare (dış işlem)", ops: []string{"custom"},
		build:  func(xs []*Tensor) *Tensor { return squareCustom(xs[0]) },
		inputs: genInputs(gen(-1, 1, 0, 2, 3)), ref: map64(func(x float64) float64 { return x * x })})
}

func TestGradCustomOp(t *testing.T) { runGradGroup(t, "custom_op") }

package tensor

import (
	"fmt"
	"math"
	"testing"
)

// Bulgu 12: GELU'nun çok büyük |x|'te ve x**0'ın x=0'da gradyanı NaN değildir.
func TestGeluAndZeroPowerGrad(t *testing.T) {
	x := New([]float32{1e20, -1e20, 3e38, -3e38, 0, 2}, 6).SetRequiresGrad(true)
	x.Gelu().SumAll().Backward()
	near(t, "GELU' uçlarda", x.Grad().Data()[:4], []float32{1, 0, 1, 0}, 0)
	z := New([]float32{0, 2, -1}, 3).SetRequiresGrad(true)
	z.Pow(0).SumAll().Backward()
	near(t, "d/dx x^0", z.Grad().Data(), []float32{0, 0, 0}, 0)
	near(t, "x^0", z.Detach().Pow(0).Data(), []float32{1, 1, 1}, 0)
}

// Bulgu 5: skalere bölme tensörün türünü korur ve gerçek bölmedir (float32'de bölünüp bir
// kez yuvarlanır).
func TestDivScalarKeepsDType(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3)
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		y := x.Cast(dt).DivScalar(3)
		if y.DType() != dt {
			t.Fatalf("%s / 3 türü %s", dt, y.DType())
		}
		near(t, dt.String()+" / 3", y.Data(), Div(x, Scalar(3)).Cast(dt).Data(), 0)
	}
	// 1/s ile çarpmadan farklı olarak tam bölme: 3/3 = 1.
	near(t, "3/3", New([]float32{3}, 1).DivScalar(3).Data(), []float32{1}, 0)
	checkGrad(t, "DivScalar gradyanı", func(xs []*Tensor) *Tensor { return xs[0].DivScalar(4) }, New([]float32{1, -2, 3}, 3))
}

// İkili işlemlerin aynı şekilli girdilerde doğrudan yazılan (sıfırlamasız) ve devredilen
// gradyanları: aynı tensör iki işlenen olduğunda, birikimde ve karışık türde de doğru.
func TestBinaryGradSameShape(t *testing.T) {
	x := New([]float32{1, -2, 3}, 3).SetRequiresGrad(true)
	w := New([]float32{2, 4, -1}, 3).SetRequiresGrad(true)
	cases := []struct {
		name   string
		f      func() *Tensor
		gx, gw []float32
	}{
		{"x+x", func() *Tensor { return x.Add(x) }, []float32{2, 2, 2}, nil},
		{"x-x", func() *Tensor { return x.Sub(x) }, []float32{0, 0, 0}, nil},
		{"x*x", func() *Tensor { return x.Mul(x) }, []float32{2, -4, 6}, nil},
		{"x-w", func() *Tensor { return x.Sub(w) }, []float32{1, 1, 1}, []float32{-1, -1, -1}},
		{"x*w", func() *Tensor { return x.Mul(w) }, []float32{2, 4, -1}, []float32{1, -2, 3}},
		{"x/w", func() *Tensor { return x.Div(w) }, []float32{0.5, 0.25, -1}, []float32{-0.25, 0.125, -3}},
		{"(x+w)*x", func() *Tensor { return x.Add(w).Mul(x) }, []float32{4, 0, 5}, []float32{1, -2, 3}},
	}
	for _, c := range cases {
		x.grad, w.grad = nil, nil
		c.f().SumAll().Backward()
		near(t, c.name+" dx", x.Grad().Data(), c.gx, 1e-6)
		if c.gw != nil {
			near(t, c.name+" dw", w.Grad().Data(), c.gw, 1e-6)
		}
	}
	// Yaprakta birikim: ikinci geri yayılım üzerine ekler.
	x.grad = nil
	x.Mul(w).SumAll().Backward()
	x.Mul(w).SumAll().Backward()
	near(t, "birikim", x.Grad().Data(), []float32{4, 8, -2}, 0)

	// Karışık tür: bf16 + float32 sonucu float32; bf16 ara sonuca gradyan kendi türünde gelir.
	h := x.Cast(BFloat16).Mul(x.Cast(BFloat16))
	x.grad = nil
	h.Add(w).SumAll().Backward()
	near(t, "karışık tür", x.Grad().Data(), []float32{2, -4, 6}, 0)

	// Çarpımın sıfır işareti korunur: -0 · 1 = -0.
	z := New([]float32{1}, 1).SetRequiresGrad(true)
	z.Mul(New([]float32{float32(math.Copysign(0, -1))}, 1)).SumAll().Backward()
	if g := z.Grad().Data()[0]; !math.Signbit(float64(g)) {
		t.Fatalf("-0 çarpanın gradyanı %v, -0 bekleniyordu", g)
	}
}

// CPU'da 16 bitlik tekli gradyan yalnızca işlemin okuduğu girdiyi (x, y ya da hiçbirini)
// açar; sonuç float32 hesabıyla (y'nin bf16 yuvarlaması payıyla) aynıdır.
func TestUnaryGrad16BitReads(t *testing.T) {
	vals := []float32{0.25, 0.5, 0.75, 1.25}
	ops := []struct {
		name string
		f    func(*Tensor) *Tensor
	}{
		{"exp", (*Tensor).Exp}, {"sqrt", (*Tensor).Sqrt}, {"tanh", (*Tensor).Tanh},
		{"sigmoid", (*Tensor).Sigmoid}, {"tan", (*Tensor).Tan}, {"expm1", (*Tensor).Expm1},
		{"log", (*Tensor).Log}, {"sin", (*Tensor).Sin}, {"silu", (*Tensor).Silu}, {"abs", (*Tensor).Abs},
		{"3/x", func(x *Tensor) *Tensor { return x.RDivScalar(3) }},
		{"x*3", func(x *Tensor) *Tensor { return x.MulScalar(3) }},
		{"x^1", func(x *Tensor) *Tensor { return x.Pow(1) }},
		{"x^3", func(x *Tensor) *Tensor { return x.Pow(3) }},
	}
	for _, op := range ops {
		x16 := NewOf(BFloat16, vals, 4).SetRequiresGrad(true)
		op.f(x16).Cast(Float32).SumAll().Backward()
		x32 := New(NewOf(BFloat16, vals, 4).Data(), 4).SetRequiresGrad(true)
		op.f(x32).SumAll().Backward()
		near(t, op.name+" bf16 gradyanı", x16.Grad().Data(), x32.Grad().Data(), 2e-2)
	}
}

// DivScalar çıktıyı saklamaz (pay gradyanı çıktıyı okumaz): çıktı geri yayılımdan önce
// yerinde değişse de gradyan doğrudur ve sürüm denetimi hata vermez.
func TestDivScalarDoesNotSaveOutput(t *testing.T) {
	x := New([]float32{2, 4}, 2).SetRequiresGrad(true)
	y := x.DivScalar(2)
	z := y.MulScalar(3) // girdisini saklamaz
	y.Set(9, 0)
	z.SumAll().Backward()
	near(t, "DivScalar gradyanı", x.Grad().Data(), []float32{1.5, 1.5}, 0)
}

// ============================================================================
// Gradyan kapısı: ikili ve temel tekli işlemler (float64 başvuru + bf16)
// ============================================================================

func gelu64(x float64) float64 {
	return 0.5 * x * (1 + math.Tanh(0.7978845608028654*(x+0.044715*x*x*x)))
}

func sigmoid64(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

func relu64(x float64) float64 { return math.Max(0, x) }

func binSpec(name string, f func(*Tensor, *Tensor) *Tensor, ref func(a, b float64) float64, a, b []int, bLo, bHi float32) gradSpec {
	return gradSpec{name: name + " " + fmt.Sprint(a, b), ops: []string{name},
		build:  func(xs []*Tensor) *Tensor { return f(xs[0], xs[1]) },
		inputs: genInputs(gen(-1, 1, 0.05, a...), gen(bLo, bHi, 0.05, b...)),
		ref:    bcast64(a, b, ref)}
}

func init() {
	add := func(a, b float64) float64 { return a + b }
	sub := func(a, b float64) float64 { return a - b }
	mul := func(a, b float64) float64 { return a * b }
	div := func(a, b float64) float64 { return a / b }
	cmp := func(f func(a, b float64) bool) func(a, b float64) float64 {
		return func(a, b float64) float64 {
			if f(a, b) {
				return 1
			}
			return 0
		}
	}
	specs := []gradSpec{
		binSpec("add", Add, add, []int{2, 3}, []int{2, 3}, -1, 1),
		binSpec("add", Add, add, []int{2, 3, 4}, []int{4}, -1, 1),
		binSpec("sub", Sub, sub, []int{3, 1}, []int{1, 4}, -1, 1),
		binSpec("sub", Sub, sub, []int{2, 1, 3}, []int{4, 1}, -1, 1),
		binSpec("mul", Mul, mul, []int{2, 3}, []int{2, 3}, -1, 1),
		binSpec("mul", Mul, mul, []int{2, 3, 4}, []int{3, 1}, -1, 1),
		binSpec("div", Div, div, []int{2, 3}, []int{2, 3}, 0.5, 2),
		binSpec("div", Div, div, []int{2, 3}, []int{2, 1}, 0.5, 2),
		{name: "div_scalar", ops: []string{"div"}, build: func(xs []*Tensor) *Tensor { return xs[0].DivScalar(4) },
			inputs: genInputs(gen(-1, 1, 0, 5)), ref: map64(func(x float64) float64 { return x / 4 })},
		{name: "sub_scalar", ops: []string{"add_scalar"}, build: func(xs []*Tensor) *Tensor { return xs[0].SubScalar(1.5) },
			inputs: genInputs(gen(-1, 1, 0, 5)), ref: map64(func(x float64) float64 { return x - 1.5 })},
		{name: "neg", ops: []string{"mul_scalar"}, build: func(xs []*Tensor) *Tensor { return xs[0].Neg() },
			inputs: genInputs(gen(-1, 1, 0, 5)), ref: map64(func(x float64) float64 { return -x })},
		unarySpec("exp", (*Tensor).Exp, math.Exp, -1, 1, 0),
		unarySpec("log", (*Tensor).Log, math.Log, 0.5, 2, 0),
		unarySpec("sqrt", (*Tensor).Sqrt, math.Sqrt, 0.5, 2, 0),
		unarySpec("tanh", (*Tensor).Tanh, math.Tanh, -2, 2, 0),
		unarySpec("relu", (*Tensor).Relu, relu64, -1, 1, 0.05),
		unarySpec("gelu", (*Tensor).Gelu, gelu64, -3, 3, 0),
		unarySpec("sigmoid", (*Tensor).Sigmoid, sigmoid64, -3, 3, 0),
		{name: "pow", ops: []string{"pow"}, build: func(xs []*Tensor) *Tensor { return xs[0].Pow(3) },
			inputs: genInputs(gen(-1.5, 1.5, 0, 3, 4)), ref: map64(func(x float64) float64 { return x * x * x })},
		{name: "pow negatif üs", ops: []string{"pow"}, build: func(xs []*Tensor) *Tensor { return xs[0].Pow(-1.5) },
			inputs: genInputs(gen(0.5, 2, 0, 3, 4)), ref: map64(func(x float64) float64 { return math.Pow(x, -1.5) })},
		{name: "add_scalar", ops: []string{"add_scalar"}, build: func(xs []*Tensor) *Tensor { return xs[0].AddScalar(1.5) },
			inputs: genInputs(gen(-1, 1, 0, 5)), ref: map64(func(x float64) float64 { return x + 1.5 })},
		{name: "mul_scalar", ops: []string{"mul_scalar"}, build: func(xs []*Tensor) *Tensor { return xs[0].MulScalar(-2.5) },
			inputs: genInputs(gen(-1, 1, 0, 5)), ref: map64(func(x float64) float64 { return -2.5 * x })},
		{name: "rsub_scalar", ops: []string{"rsub_scalar"}, build: func(xs []*Tensor) *Tensor { return xs[0].RSubScalar(3) },
			inputs: genInputs(gen(-1, 1, 0, 5)), ref: map64(func(x float64) float64 { return 3 - x })},
		{name: "rdiv_scalar", ops: []string{"rdiv_scalar"}, build: func(xs []*Tensor) *Tensor { return xs[0].RDivScalar(2) },
			inputs: genInputs(gen(0.5, 2, 0, 5)), ref: map64(func(x float64) float64 { return 2 / x })},
	}
	for _, c := range []struct {
		name string
		op   BinOp
		f    func(a, b float64) bool
	}{{"lt", OpLt, func(a, b float64) bool { return a < b }}, {"gt", OpGt, func(a, b float64) bool { return a > b }},
		{"le", OpLe, func(a, b float64) bool { return a <= b }}, {"ge", OpGe, func(a, b float64) bool { return a >= b }}} {
		c := c
		specs = append(specs, gradSpec{name: c.name, ops: []string{c.name}, mode: gradNone,
			build:  func(xs []*Tensor) *Tensor { return Compare(xs[0], xs[1], c.op) },
			inputs: genInputs(gen(-1, 1, 0.05, 2, 3), gen(-1, 1, 0.05, 3)),
			ref:    bcast64([]int{2, 3}, []int{3}, cmp(c.f))})
	}
	registerGrad("elementwise", specs...)
}

func TestGradElementwise(t *testing.T) { runGradGroup(t, "elementwise") }

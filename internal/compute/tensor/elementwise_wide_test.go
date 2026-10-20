package tensor

import (
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Genişletilmiş eleman bazlı işlemler: float64 başvuruya göre doğruluk (kenar değerler
// dahil), sayısal gradyan denetimi, yayınlama, 16 bitlik türler ve türevsiz işlemlerin
// sıfır gradyanı.

var (
	posInf32 = float32(math.Inf(1))
	negInf32 = float32(math.Inf(-1))
	nan32    = float32(math.NaN())
)

// customInputs her tekli işlemin denendiği kenar değerler: sıfırlar, ±1, alt-normaller,
// küçük ve büyük sayılar, sonsuzlar, NaN.
var customInputs = []float32{0, float32(math.Copysign(0, -1)), 1, -1, 0.5, -0.5, 1e-40, -1e-40, 1e-30, -1e-30,
	1e-8, -1e-8, 1e-4, -1e-4, 0.3, -0.3, 0.999, -0.999, 2, -2, 10, -10, 50, -50, 88, -88, 89, -89, 100, -100,
	1e10, -1e10, 1e30, -1e30, posInf32, negInf32, nan32}

type unaryRef struct {
	ad     string
	f      func(*Tensor) *Tensor
	ref    func(float64) float64
	ulp    float64 // izin verilen en büyük fark (float32 ulp'u)
	dtypeV func(x float64) float64
}

func silu64(x float64) float64 {
	if math.IsInf(x, -1) {
		return 0 // sınır değeri
	}
	return x / (1 + math.Exp(-x))
}

func sign64(x float64) float64 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return x
}

var unaryRefs = []unaryRef{
	{"silu", (*Tensor).Silu, silu64, 3, func(x float64) float64 { s := 1 / (1 + math.Exp(-x)); return s * (1 + x*(1-s)) }},
	{"tan", (*Tensor).Tan, math.Tan, 1, func(x float64) float64 { t := math.Tan(x); return 1 + t*t }},
	{"asin", (*Tensor).Asin, math.Asin, 1, func(x float64) float64 { return 1 / math.Sqrt(1-x*x) }},
	{"acos", (*Tensor).Acos, math.Acos, 1, func(x float64) float64 { return -1 / math.Sqrt(1-x*x) }},
	{"atan", (*Tensor).Atan, math.Atan, 1, func(x float64) float64 { return 1 / (1 + x*x) }},
	{"sinh", (*Tensor).Sinh, math.Sinh, 1, math.Cosh},
	{"cosh", (*Tensor).Cosh, math.Cosh, 1, math.Sinh},
	{"log1p", (*Tensor).Log1p, math.Log1p, 1, func(x float64) float64 { return 1 / (1 + x) }},
	{"log2", (*Tensor).Log2, math.Log2, 1, func(x float64) float64 { return 1 / (x * math.Ln2) }},
	{"log10", (*Tensor).Log10, math.Log10, 1, func(x float64) float64 { return 1 / (x * math.Ln10) }},
	{"expm1", (*Tensor).Expm1, math.Expm1, 1, math.Exp},
	{"floor", (*Tensor).Floor, math.Floor, 0, nil},
	{"ceil", (*Tensor).Ceil, math.Ceil, 0, nil},
	{"sign", (*Tensor).Sign, sign64, 0, nil},
	{"abs", (*Tensor).Abs, math.Abs, 0, sign64},
}

// ulpDistance a ile b (float32) arasındaki fark, b'nin ulp'u biriminde; ikisi de NaN ise 0,
// biri NaN ya da sonsuzsa ve öbürü aynı değilse sonsuz.
func ulpDistance(a, b float32) float64 {
	if a != a || b != b {
		if a != a && b != b {
			return 0
		}
		return math.Inf(1)
	}
	if a == b {
		return 0
	}
	if math.IsInf(float64(a), 0) || math.IsInf(float64(b), 0) {
		return math.Inf(1)
	}
	u := float64(math.Nextafter32(abs32(b), posInf32) - abs32(b))
	return math.Abs(float64(a)-float64(b)) / u
}

func TestUnaryWideRef(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	input := append([]float32(nil), customInputs...)
	for i := 0; i < 2000; i++ {
		input = append(input, float32(rng.NormFloat64()*3), float32(rng.Float64()*2-1))
	}
	x := New(input, len(input))
	for _, b := range unaryRefs {
		got := b.f(x).Data()
		for i, v := range input {
			want := float32(b.ref(float64(v)))
			if math.Abs(float64(want)) < 1e-36 && math.Abs(float64(got[i])-float64(want)) < 1e-38 {
				continue // alt taşma bölgesi (ör. silu(-100)): mutlak hata yeterince küçük
			}
			if d := ulpDistance(got[i], want); d > b.ulp {
				t.Fatalf("%s(%v) = %v, başvuru %v (%.1f ulp; izin %.0f)", b.ad, v, got[i], want, d, b.ulp)
			}
		}
	}
}

// TestUnaryWideEdges belgelenen kenar değerler: tanım dışı NaN, sonsuzlar, işaretli sıfır,
// log1p ve expm1'in küçük x kararlılığı.
func TestUnaryWideEdges(t *testing.T) {
	cases := []struct {
		ad   string
		f    func(*Tensor) *Tensor
		x, y float32
	}{
		{"asin(2)", (*Tensor).Asin, 2, nan32},
		{"acos(-1.5)", (*Tensor).Acos, -1.5, nan32},
		{"asin(1)", (*Tensor).Asin, 1, float32(math.Pi / 2)},
		{"acos(-1)", (*Tensor).Acos, -1, float32(math.Pi)},
		{"atan(+sonsuz)", (*Tensor).Atan, posInf32, float32(math.Pi / 2)},
		{"log1p(-1)", (*Tensor).Log1p, -1, negInf32},
		{"log1p(-2)", (*Tensor).Log1p, -2, nan32},
		{"log1p(1e-10)", (*Tensor).Log1p, 1e-10, 1e-10},
		{"log2(8)", (*Tensor).Log2, 8, 3},
		{"log2(0)", (*Tensor).Log2, 0, negInf32},
		{"log10(1000)", (*Tensor).Log10, 1000, 3},
		{"log10(-1)", (*Tensor).Log10, -1, nan32},
		{"expm1(1e-10)", (*Tensor).Expm1, 1e-10, 1e-10},
		{"expm1(-sonsuz)", (*Tensor).Expm1, negInf32, -1},
		{"sinh(89)", (*Tensor).Sinh, 89, float32(math.Sinh(89))},
		{"cosh(-sonsuz)", (*Tensor).Cosh, negInf32, posInf32},
		{"silu(-sonsuz)", (*Tensor).Silu, negInf32, 0},
		{"silu(+sonsuz)", (*Tensor).Silu, posInf32, posInf32},
		{"floor(-2.5)", (*Tensor).Floor, -2.5, -3},
		{"ceil(-2.5)", (*Tensor).Ceil, -2.5, -2},
		{"sign(NaN)", (*Tensor).Sign, nan32, nan32},
		{"sign(-0)", (*Tensor).Sign, float32(math.Copysign(0, -1)), 0},
		{"abs(-sonsuz)", (*Tensor).Abs, negInf32, posInf32},
		{"abs(NaN)", (*Tensor).Abs, nan32, nan32},
		{"abs(-1e-40)", (*Tensor).Abs, -1e-40, 1e-40},
	}
	for _, d := range cases {
		got := d.f(New([]float32{d.x}, 1)).Data()[0]
		if ulpDistance(got, d.y) > 1 {
			t.Errorf("%s = %v, beklenen %v", d.ad, got, d.y)
		}
	}
	// log1p ve expm1 küçük x'te 1 + x yuvarlamasından etkilenmez: göreli hata küçük kalır.
	for _, v := range []float32{1e-7, -1e-7, 3e-9, 1e-20} {
		if g := New([]float32{v}, 1).Log1p().Data()[0]; ulpDistance(g, float32(math.Log1p(float64(v)))) > 1 {
			t.Errorf("log1p(%v) = %v", v, g)
		}
		if g := New([]float32{v}, 1).Expm1().Data()[0]; ulpDistance(g, float32(math.Expm1(float64(v)))) > 1 {
			t.Errorf("expm1(%v) = %v", v, g)
		}
	}
}

func TestUnaryWideGrad(t *testing.T) {
	for _, b := range unaryRefs {
		if b.dtypeV == nil {
			continue
		}
		lo, hi := float32(-2), float32(2)
		switch b.ad {
		case "asin", "acos":
			lo, hi = -0.9, 0.9
		case "log1p":
			lo, hi = -0.5, 2
		case "log2", "log10":
			lo, hi = 0.5, 3
		case "tan":
			lo, hi = -1.2, 1.2
		}
		x := random(rand.New(rand.NewSource(5)), lo, hi, 0, 3, 4)
		checkGrad(t, b.ad, func(xs []*Tensor) *Tensor { return b.f(xs[0]) }, x)
		// Analitik türev float64 başvuruyla da uyuşmalı.
		xg := New(x.Data(), 3, 4).SetRequiresGrad(true)
		b.f(xg).SumAll().Backward()
		for i, v := range x.Data() {
			want := b.dtypeV(float64(v))
			if g := float64(xg.Grad().Data()[i]); math.Abs(g-want) > 2e-6*(1+math.Abs(want)) {
				t.Fatalf("%s'(%v) = %v, başvuru %v", b.ad, v, g, want)
			}
		}
	}
}

// TestUnaryNoGrad taban, tavan ve işaretin gradyanı her yerde 0'dır (işaret 0'da da).
func TestUnaryNoGrad(t *testing.T) {
	for _, f := range []func(*Tensor) *Tensor{(*Tensor).Floor, (*Tensor).Ceil, (*Tensor).Sign} {
		x := New([]float32{-1.5, 0, 2.5, 3}, 4).SetRequiresGrad(true)
		f(x).Mul(New([]float32{1, 2, 3, 4}, 4)).SumAll().Backward()
		nanSame(t, "türevsiz tekli gradyanı", x.Grad().Data(), []float32{0, 0, 0, 0})
	}
}

// TestAbs mutlak değer: −0'ın sonucu +0'dır (işaret biti silinir); türev işaret(x), 0'da (±0)
// tam 0, NaN'da NaN; 16 bitlik türlerde sonuç girdinin türündedir; işçi sayısından bağımsızdır.
func TestAbs(t *testing.T) {
	negZero := float32(math.Copysign(0, -1))
	if y := New([]float32{negZero}, 1).Abs().Data()[0]; math.Float32bits(y) != 0 {
		t.Fatalf("abs(-0) bitleri %#x, 0 bekleniyordu", math.Float32bits(y))
	}
	x := New([]float32{-1.5, 0, negZero, 2.5, nan32, negInf32}, 6).SetRequiresGrad(true)
	x.Abs().Mul(New([]float32{1, 2, 3, 4, 5, 6}, 6)).SumAll().Backward()
	nanSame(t, "abs gradyanı", x.Grad().Data(), []float32{-1, 0, 0, 4, nan32, -6})
	for _, dtyp := range []DType{BFloat16, Float16} {
		y := New([]float32{-1.5, 2, -0.25}, 3).Cast(dtyp).Abs()
		if y.DType() != dtyp {
			t.Fatalf("abs %v sonucu %v türünde", dtyp, y.DType())
		}
		nanSame(t, "abs 16 bit", y.Cast(Float32).Data(), []float32{1.5, 2, 0.25})
	}
	// Büyük girdi (paralel yol) tek elemanlı hesapla aynıdır.
	rng := rand.New(rand.NewSource(8))
	b := random(rng, -3, 3, 0, 300, 700)
	got := b.Abs().Data()
	for i, v := range b.Data() {
		if got[i] != abs32(v) {
			t.Fatalf("abs paralel yol %d: %v, %v bekleniyordu", i, got[i], abs32(v))
		}
	}
}

func TestClamp(t *testing.T) {
	x := New([]float32{-3, -1, 0, 1, 2, 5, nan32}, 7).SetRequiresGrad(true)
	y := x.Clamp(-1, 2)
	nanSame(t, "clamp", y.Data(), []float32{-1, -1, 0, 1, 2, 2, nan32})
	y.SumAll().Backward()
	// Sınırlar dahil içeride 1, dışarıda ve NaN'da 0.
	nanSame(t, "clamp gradyanı", x.Grad().Data(), []float32{0, 1, 1, 1, 1, 0, 0})
	nanSame(t, "clamp alt", New([]float32{-2, 3}, 2).ClampMin(0).Data(), []float32{0, 3})
	nanSame(t, "clamp üst", New([]float32{-2, 3}, 2).ClampMax(0).Data(), []float32{-2, 0})
	panicCode(t, "ters sınırlar", messages.H3040, func() { x.Clamp(2, 1) })
	checkGrad(t, "clamp", func(xs []*Tensor) *Tensor { return xs[0].Clamp(-0.5, 0.5) },
		random(rand.New(rand.NewSource(2)), -1, 1, 0, 4, 5))
}

func TestAtan2(t *testing.T) {
	zeroVal, negZero := float32(0), float32(math.Copysign(0, -1))
	ys := []float32{1, 1, -1, -1, 0, 0, negZero, negZero, zeroVal, posInf32, posInf32, 3, nan32, 2}
	xs := []float32{1, -1, 1, -1, 1, -1, 1, -1, zeroVal, posInf32, -5, negInf32, 1, negZero}
	got := Atan2(New(ys, len(ys)), New(xs, len(xs))).Data()
	for i := range ys {
		want := float32(math.Atan2(float64(ys[i]), float64(xs[i])))
		if ulpDistance(got[i], want) > 1 || (want == want && math.Signbit(float64(got[i])) != math.Signbit(float64(want))) {
			t.Fatalf("atan2(%v, %v) = %v, beklenen %v", ys[i], xs[i], got[i], want)
		}
	}
	// Yayınlama ve gradyan.
	rng := rand.New(rand.NewSource(4))
	checkGrad(t, "atan2 yayın", func(x []*Tensor) *Tensor { return Atan2(x[0], x[1]) },
		random(rng, -2, 2, 0.2, 3, 1), random(rng, -2, 2, 0.2, 1, 4))
	// (0, 0)'da gradyan 0.
	y := New([]float32{0, 1}, 2).SetRequiresGrad(true)
	x := New([]float32{0, 1}, 2).SetRequiresGrad(true)
	Atan2(y, x).SumAll().Backward()
	nanSame(t, "atan2 (0,0) gradyanı y", y.Grad().Data(), []float32{0, 0.5})
	nanSame(t, "atan2 (0,0) gradyanı x", x.Grad().Data(), []float32{0, -0.5})
}

func TestCompareAndWhere(t *testing.T) {
	a := New([]float32{1, 2, 3, nan32}, 4)
	b := New([]float32{2}, 1)
	nanSame(t, "<", Compare(a, b, OpLt).Data(), []float32{1, 0, 0, 0})
	nanSame(t, ">=", Compare(a, b, OpGe).Data(), []float32{0, 1, 1, 0})
	nanSame(t, "sayı solda", a.CompareScalar(2, OpLt, true).Data(), []float32{0, 0, 1, 0})
	nanSame(t, "yayın", Compare(New([]float32{1, 2}, 2, 1), New([]float32{1, 2, 3}, 3), OpLe).Data(), []float32{1, 1, 1, 0, 1, 1})

	c := New([]float32{1, 0, nan32}, 3, 1)
	x := New([]float32{10, 20}, 1, 2).SetRequiresGrad(true)
	y := New([]float32{-1, -2, -3}, 3, 1).SetRequiresGrad(true)
	w := Where(c, x, y)
	shapeEqual(t, "where şekli", w.Shape(), []int{3, 2})
	nanSame(t, "where", w.Data(), []float32{10, 20, -2, -2, 10, 20})
	// Seçilmeyen yere giden gradyan NaN olsa da öbür dala sızmaz.
	g := New([]float32{1, 2, nan32, nan32, 5, 6}, 3, 2)
	w.BackwardWith(g)
	nanSame(t, "where gradyanı a", x.Grad().Data(), []float32{6, 8})
	nanSame(t, "where gradyanı b", y.Grad().Data(), []float32{0, nan32, 0})
	rng := rand.New(rand.NewSource(8))
	cond := Compare(Randn(rng, 2, 3), Scalar(0), OpGt)
	checkGrad(t, "where", func(z []*Tensor) *Tensor { return Where(cond, z[0], z[1]) },
		random(rng, -1, 1, 0, 2, 3), random(rng, -1, 1, 0, 3))
}

// TestWideBits16 16 bitlik girdiler float32'ye açılarak hesaplanır; sonuç girdinin
// türündedir ve float32 sonucun o türe yuvarlanmışıdır.
func TestWideBits16(t *testing.T) {
	data := []float32{-2, -0.75, -0.1, 0, 0.3, 0.9, 1.7, 3}
	for _, dt := range []DType{BFloat16, Float16} {
		x := NewOf(dt, data, len(data))
		x32 := x.Cast(Float32)
		for _, b := range unaryRefs {
			got := b.f(x)
			if got.DType() != dt {
				t.Fatalf("%s %s: sonuç türü %s", b.ad, dt, got.DType())
			}
			want := b.f(x32).Cast(dt).Data()
			nanSame(t, b.ad+" "+dt.String(), got.Data(), want)
		}
		nanSame(t, "clamp "+dt.String(), x.Clamp(-1, 1).Data(), x32.Clamp(-1, 1).Cast(dt).Data())
		nanSame(t, "atan2 "+dt.String(), Atan2(x, x.Neg()).Data(), Atan2(x32, x32.Neg()).Cast(dt).Data())
	}
}

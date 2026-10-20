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

// geluExact64 tam GELU'nun float64 başvurusu: x·½·erfc(−x/√2); −∞'da 0.
func geluExact64(x float64) float64 {
	if math.IsInf(x, -1) {
		return 0
	}
	return x * 0.5 * math.Erfc(-x/math.Sqrt2)
}

// geluExactSlope64 türev başvurusu: Φ(x) + x·φ(x); sonsuzlarda x·φ(x) → 0 alınır.
func geluExactSlope64(x float64) float64 {
	phi := 0.5 * math.Erfc(-x/math.Sqrt2)
	if math.IsInf(x, 0) {
		return phi
	}
	return phi + x*math.Exp(-x*x/2)/math.Sqrt(2*math.Pi)
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
	{"silu", (*Tensor).Silu, silu64, 3, func(x float64) float64 {
		if math.IsInf(x, 0) { // sınırda türev 1 (+∞) ve 0 (−∞); formül ∞·0 verirdi
			return math.Max(0, math.Copysign(1, x))
		}
		s := 1 / (1 + math.Exp(-x))
		return s * (1 + x*(1-s))
	}},
	{"gelu_exact", (*Tensor).GeluExact, geluExact64, 1, geluExactSlope64},
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
		{"gelu_exact(-sonsuz)", (*Tensor).GeluExact, negInf32, 0},
		{"gelu_exact(+sonsuz)", (*Tensor).GeluExact, posInf32, posInf32},
		{"gelu_exact(NaN)", (*Tensor).GeluExact, nan32, nan32},
		{"gelu_exact(-100)", (*Tensor).GeluExact, -100, 0},
		{"gelu_exact(0)", (*Tensor).GeluExact, 0, 0},
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

// ============================================================================
// Gradyan kapısı: genişletilmiş tekli işlemler, clamp, atan2, where
// ============================================================================

func init() {
	zeroSpec := func(name string, f func(*Tensor) *Tensor, ref func(float64) float64) gradSpec {
		s := unarySpec(name, f, ref, -3, 3, 0.05)
		s.mode = gradZero
		return s
	}
	w := []gradSpec{
		unarySpec("silu", (*Tensor).Silu, silu64, -3, 3, 0),
		unarySpec("gelu_exact", (*Tensor).GeluExact, geluExact64, -3, 3, 0),
		unarySpec("tan", (*Tensor).Tan, math.Tan, -1.2, 1.2, 0),
		unarySpec("asin", (*Tensor).Asin, math.Asin, -0.9, 0.9, 0),
		unarySpec("acos", (*Tensor).Acos, math.Acos, -0.9, 0.9, 0),
		unarySpec("atan", (*Tensor).Atan, math.Atan, -3, 3, 0),
		unarySpec("sinh", (*Tensor).Sinh, math.Sinh, -2, 2, 0),
		unarySpec("cosh", (*Tensor).Cosh, math.Cosh, -2, 2, 0),
		unarySpec("log1p", (*Tensor).Log1p, math.Log1p, -0.5, 2, 0),
		unarySpec("log2", (*Tensor).Log2, math.Log2, 0.5, 3, 0),
		unarySpec("log10", (*Tensor).Log10, math.Log10, 0.5, 3, 0),
		unarySpec("expm1", (*Tensor).Expm1, math.Expm1, -2, 2, 0),
		unarySpec("abs", (*Tensor).Abs, math.Abs, -2, 2, 0.05),
		zeroSpec("floor", (*Tensor).Floor, math.Floor),
		zeroSpec("ceil", (*Tensor).Ceil, math.Ceil),
		zeroSpec("sign", (*Tensor).Sign, sign64),
		{name: "clamp", ops: []string{"clamp"}, build: func(xs []*Tensor) *Tensor { return xs[0].Clamp(-0.5, 0.5) },
			inputs: genInputs(gen(-1, 1, 0.03, 4, 5)), ref: map64(func(x float64) float64 { return math.Max(-0.5, math.Min(0.5, x)) })},
		{name: "atan2 yayın", ops: []string{"atan2"}, build: func(xs []*Tensor) *Tensor { return Atan2(xs[0], xs[1]) },
			inputs: genInputs(gen(-2, 2, 0.2, 3, 1), gen(-2, 2, 0.2, 1, 4)), ref: bcast64([]int{3, 1}, []int{1, 4}, math.Atan2)},
		{name: "where yayın", ops: []string{"where"},
			build:  func(xs []*Tensor) *Tensor { return Where(New([]float32{1, 0, 1, 1, 0, 0}, 2, 3), xs[0], xs[1]) },
			inputs: genInputs(gen(-1, 1, 0.05, 2, 3), gen(-1, 1, 0.05, 3)),
			ref: func(in [][]float64) []float64 {
				cond := []bool{true, false, true, true, false, false}
				res := make([]float64, 6)
				for i := range res {
					if cond[i] {
						res[i] = in[0][i]
					} else {
						res[i] = in[1][i%3]
					}
				}
				return res
			}},
	}
	registerGrad("elementwise_wide", w...)
}

func TestGradElementwiseWide(t *testing.T) { runGradGroup(t, "elementwise_wide") }

// knownEdgeGradIssues sınır değerlerinde gradyanı beklenenden saptığı bilinen işlemlerdir
// (gerekçeli; yalnızca küçülür). Kayıttaki girdilerde sapma sürmelidir: düzeltilirse test
// kaydın silinmesini ister. Düzeltme bu işin kapsamı dışında, bulgu olarak bildirilir.
var knownEdgeGradIssues = []struct {
	op     string
	x      func(float32) bool
	reason string
}{}

// TestUnaryGradEdges türevin sıfırlar, alt-normaller, büyük ve sonsuz girdilerdeki değeri float64
// başvuruyla uyuşur (silu'nun alt-normal bölgesi hariç mutlak hata 1e-38).
func TestUnaryGradEdges(t *testing.T) {
	type edge struct {
		ad   string
		f    func(*Tensor) *Tensor
		d    func(float64) float64
		atol float64 // ulp tutmasa da kabul edilen mutlak hata
	}
	var cases []edge
	for _, b := range unaryRefs {
		if b.dtypeV != nil {
			cases = append(cases, edge{b.ad, b.f, b.dtypeV, 0})
		}
	}
	cases = append(cases,
		edge{"exp", (*Tensor).Exp, math.Exp, 0},
		// Tanım dışında (x<0) log'un gradyanı g/x'tir (NaN değil): ileri değer NaN'dır, gradyan
		// formülü girdiyi okur.
		edge{"log", (*Tensor).Log, func(x float64) float64 { return 1 / x }, 0},
		edge{"sqrt", (*Tensor).Sqrt, func(x float64) float64 {
			if x < 0 {
				return math.NaN()
			}
			return 0.5 / math.Sqrt(x)
		}, 0},
		// tanh türevi 1−y² ile bulunur (y float32): |x|>9'da y=1−2⁻²⁴ olur ve türev 1.2e-7 çıkar
		// (doğrusu ≤ 8e-9); mutlak hata 2⁻²³ ile sınırlıdır, atol bunu kabul eder.
		edge{"tanh", (*Tensor).Tanh, func(x float64) float64 { y := math.Tanh(x); return 1 - y*y }, 2.5e-7},
		// sigmoid türevi s(1−s) ile bulunur (s float32): kuyrukta 1−s yuvarlaması göreli hatayı
		// 5e-4'e çıkarır, mutlak hata ≤ 1e-7 kalır.
		edge{"sigmoid", (*Tensor).Sigmoid, func(x float64) float64 { s := 1 / (1 + math.Exp(-x)); return s * (1 - s) }, 1e-7},
		edge{"sin", (*Tensor).Sin, math.Cos, 0},
		edge{"cos", (*Tensor).Cos, func(x float64) float64 { return -math.Sin(x) }, 0})
	for _, c := range cases {
		x := New(append([]float32(nil), customInputs...), len(customInputs)).SetRequiresGrad(true)
		c.f(x).SumAll().Backward()
		for i, v := range customInputs {
			got, want := x.Grad().Data()[i], float32(c.d(float64(v)))
			if math.Abs(float64(want)) < 1e-36 && math.Abs(float64(got)-float64(want)) < 1e-38 {
				continue // alt taşma bölgesi
			}
			bad := ulpDistance(got, want) > 8 && math.Abs(float64(got)-float64(want)) > c.atol
			if known := knownEdgeIssue(c.ad, v); known != "" {
				if !bad {
					t.Errorf("%s'(%v) artık doğru: knownEdgeGradIssues kaydını silin", c.ad, v)
				}
			} else if bad {
				t.Errorf("%s'(%v) = %v, başvuru %v", c.ad, v, got, want)
			}
		}
	}
}

func knownEdgeIssue(op string, x float32) string {
	for _, k := range knownEdgeGradIssues {
		if k.op == op && k.x(x) {
			return k.reason
		}
	}
	return ""
}

// TestGeluExactErfFormula tam GELU, tanım olan 0.5·x·(1 + erf(x/√2)) ile (iptalin zararsız
// olduğu x > −3'te) uyuşur; tanh yaklaşımından en çok ~5e-4 farklıdır, ama farklıdır.
func TestGeluExactErfFormula(t *testing.T) {
	var xs []float32
	for v := float32(-3); v <= 8; v += 0.0625 {
		xs = append(xs, v)
	}
	exact := New(xs, len(xs)).GeluExact().Data()
	approx := New(xs, len(xs)).Gelu().Data()
	worst := 0.0
	for i, v := range xs {
		x := float64(v)
		want := float32(0.5 * x * (1 + math.Erf(x/math.Sqrt2)))
		if want != 0 && ulpDistance(exact[i], want) > 2 {
			t.Fatalf("gelu_exact(%v) = %v, 0.5·x·(1+erf) = %v", v, exact[i], want)
		}
		worst = math.Max(worst, math.Abs(float64(exact[i]-approx[i])))
	}
	if worst < 1e-4 || worst > 6e-4 {
		t.Errorf("tam ve tanh yaklaşımlı GELU arasındaki en büyük fark %g; ~4.7e-4 beklenir", worst)
	}
}

// TestGeluExactGradEdges türev uçlarda: −∞'da 0, +∞'da 1, NaN'da NaN; x·φ(x) terimi ∞·0
// NaN üretmez.
func TestGeluExactGradEdges(t *testing.T) {
	x := New([]float32{negInf32, posInf32, nan32, -100, 100, 0}, 6).SetRequiresGrad(true)
	x.GeluExact().SumAll().Backward()
	want := []float32{0, 1, nan32, 0, 1, 0.5}
	for i, g := range x.Grad().Data() {
		if ulpDistance(g, want[i]) > 1 {
			t.Errorf("gelu_exact'(%v) = %v, beklenen %v", x.Data()[i], g, want[i])
		}
	}
}

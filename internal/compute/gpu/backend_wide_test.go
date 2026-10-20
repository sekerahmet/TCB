package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Genişletilmiş matematik: tekli işlemler (ileri, türev, özel değerler), atan2,
// sınırlama, karşılaştırma, koşullu seçim, birikimli toplam ve sıralama GPU'da (ya da
// benzetimde) CPU ile karşılaştırılır; 16 bitlik türler ve belirlenimcilik dahil.

type wideUnary struct {
	ad    string
	f     func(*tensor.Tensor) *tensor.Tensor
	space func(rng *rand.Rand) float32 // tanım aralığından girdi
}

func normal(scaleF float64) func(*rand.Rand) float32 {
	return func(r *rand.Rand) float32 { return float32(r.NormFloat64() * scaleF) }
}

func span(lo, hi float64) func(*rand.Rand) float32 {
	return func(r *rand.Rand) float32 { return float32(lo + (hi-lo)*r.Float64()) }
}

var wideUnaries = []wideUnary{
	{"silu", (*tensor.Tensor).Silu, normal(4)},
	{"gelu_tam", (*tensor.Tensor).GeluExact, normal(4)},
	{"tan", (*tensor.Tensor).Tan, normal(3)},
	{"asin", (*tensor.Tensor).Asin, span(-0.99, 0.99)},
	{"acos", (*tensor.Tensor).Acos, span(-0.99, 0.99)},
	{"atan", (*tensor.Tensor).Atan, normal(5)},
	{"sinh", (*tensor.Tensor).Sinh, normal(4)},
	{"cosh", (*tensor.Tensor).Cosh, normal(4)},
	{"log1p", (*tensor.Tensor).Log1p, span(-0.95, 20)},
	{"log2", (*tensor.Tensor).Log2, span(0.01, 100)},
	{"log10", (*tensor.Tensor).Log10, span(0.01, 1000)},
	{"expm1", (*tensor.Tensor).Expm1, normal(4)},
	{"taban", (*tensor.Tensor).Floor, normal(5)},
	{"tavan", (*tensor.Tensor).Ceil, normal(5)},
	{"işaret", (*tensor.Tensor).Sign, normal(1)},
	{"mutlak", (*tensor.Tensor).Abs, normal(3)},
}

// TestAbsGradAtZero mutlak değerin türevinin 0'da (±0) tam 0, NaN'da NaN olduğunu ve
// GPU'nun bitleri CPU'nunkiyle aynı verdiğini (−0 → 0, NaN korunur) denetler; 16 bitlik
// girdiler dahil.
func TestAbsGradAtZero(t *testing.T) {
	negZero := float32(math.Copysign(0, -1))
	data := []float32{0, negZero, -2, 3, float32(math.NaN()), float32(math.Inf(-1)), -1e-40}
	for _, dtype := range []tensor.DType{tensor.Float32, tensor.BFloat16, tensor.Float16} {
		compare(t, fmt.Sprintf("mutlak %v", dtype), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := inLeafDType(twin(data, true, len(data))[c], dtype)
			return x.Abs().Mul(tensor.NewOn(x.Device(), []float32{1, 2, 3, 4, 5, 6, 7}, 7).Cast(dtype)), []*tensor.Tensor{x}
		})
	}
	forward := twin(data, false, len(data))
	a, b := forward[0].Abs().Data(), forward[1].Abs().Data()
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) || math.Signbit(float64(b[i])) {
			t.Fatalf("mutlak(%v): GPU %v (%#x), CPU %v (%#x)", data[i], b[i], math.Float32bits(b[i]), a[i], math.Float32bits(a[i]))
		}
	}
	x := tensor.NewOn(gpuDevice, data, len(data)).SetRequiresGrad(true)
	x.Abs().SumAll().Backward()
	g := x.Grad().Data()
	for i, b := range []float32{0, 0, -1, 1} {
		if g[i] != b || (b == 0 && math.Signbit(float64(g[i]))) {
			t.Fatalf("mutlak türevi %v'de %v, %v bekleniyordu (%v)", data[i], g[i], b, g)
		}
	}
	if g[4] == g[4] {
		t.Fatalf("NaN'da türev NaN olmalı: %v", g)
	}
}

func TestUnaryWide(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for _, is := range wideUnaries {
		for _, n := range []int{0, 1, 257, 1031} {
			data := make([]float32, n)
			for i := range data {
				data[i] = is.space(rng)
			}
			compare(t, fmt.Sprintf("%s n=%d", is.ad, n), 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(data, true, n)[c]
				return is.f(x), []*tensor.Tensor{x}
			})
		}
	}
}

// TestUnaryWideSpecialValues sıfırlar, ±1, alt-normaller, büyük ve küçük sayılar,
// sonsuzlar, NaN ve tanım dışı değerler.
func TestUnaryWideSpecialValues(t *testing.T) {
	inf := float32(math.Inf(1))
	data := []float32{0, float32(math.Copysign(0, -1)), 1, -1, 0.5, -0.5, 0.75, -0.75, 1e-40, -1e-40, 1e-30, -1e-30,
		1e-8, -1e-8, 1e-4, -1e-4, 0.3, -0.3, 0.35, -0.35, 2, -2, 1.5707964, -1.5707964, 10, -10, 50, -50, 81, -81,
		88, -88, 89, -89, 100, -100, 1e10, -1e10, 1e30, -1e30, inf, -inf, float32(math.NaN()), 1.5, 2, -2.5, 3}
	for _, is := range wideUnaries {
		v := data
		if is.ad == "tan" {
			// Tanjantın aralık indirgemesi (sinüs ve kosinüs gibi) |x| ≤ 1e5'te doğrudur.
			v = nil
			for _, x := range data {
				if !(math.Abs(float64(x)) > 1e5) || math.IsInf(float64(x), 0) {
					v = append(v, x)
				}
			}
		}
		compare(t, is.ad+" özel", 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			return is.f(twin(v, false, len(v))[c]), nil
		})
	}
}

func TestAtan2ClampCompare(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	inf, nan := float32(math.Inf(1)), float32(math.NaN())
	negZero := float32(math.Copysign(0, -1))
	ys := []float32{1, 1, -1, -1, 0, 0, negZero, negZero, 0, inf, inf, -inf, 3, nan, 2, 1e-30, 1e30}
	xs := []float32{1, -1, 1, -1, 1, -1, 1, -1, negZero, inf, -5, -inf, -inf, 1, 0, 1e-30, -1e30}
	compare(t, "atan2 özel", 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		return tensor.Atan2(twin(ys, false, len(ys))[c], twin(xs, false, len(xs))[c]), nil
	})
	for _, sh := range [][2][]int{{{37}, {37}}, {{5, 1}, {1, 7}}, {{3, 1, 4}, {2, 1}}, {{1}, {6, 3}}} {
		av, bv := random(rng, numel(sh[0]), 2), random(rng, numel(sh[1]), 2)
		compare(t, fmt.Sprintf("atan2 %v %v", sh[0], sh[1]), 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			a, b := twin(av, true, sh[0]...)[c], twin(bv, true, sh[1]...)[c]
			return tensor.Atan2(a, b), []*tensor.Tensor{a, b}
		})
		for _, op := range []tensor.BinOp{tensor.OpLt, tensor.OpGt, tensor.OpLe, tensor.OpGe} {
			compare(t, fmt.Sprintf("karşılaştırma %d %v", op, sh), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				return tensor.Compare(twin(av, false, sh[0]...)[c], twin(bv, false, sh[1]...)[c], op), nil
			})
		}
	}
	v := append(random(rng, 300, 2), nan, inf, -inf, -1, 1)
	compare(t, "sınırla", 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := twin(v, true, len(v))[c]
		return x.Clamp(-1, 1), []*tensor.Tensor{x}
	})
	compare(t, "sınırla alt", 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := twin(v, true, len(v))[c]
		return x.ClampMin(0.5), []*tensor.Tensor{x}
	})
}

func TestWhere(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for _, sh := range [][3][]int{{{41}, {41}, {41}}, {{4, 1}, {1, 5}, {4, 5}}, {{2, 3, 1}, {3, 4}, {1}}, {{6}, {1}, {6}}} {
		cv := random(rng, numel(sh[0]), 1)
		for i := range cv {
			if cv[i] < 0 {
				cv[i] = 0
			}
		}
		if len(cv) > 2 {
			cv[1] = float32(math.NaN()) // NaN koşul doğru sayılır
		}
		av, bv := random(rng, numel(sh[1]), 1), random(rng, numel(sh[2]), 1)
		compare(t, fmt.Sprintf("koşullu %v", sh), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			k := twin(cv, false, sh[0]...)[c]
			a, b := twin(av, true, sh[1]...)[c], twin(bv, true, sh[2]...)[c]
			return tensor.Where(k, a, b), []*tensor.Tensor{a, b}
		})
	}
}

func TestCumSum(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	for _, sh := range [][]int{{1}, {37}, {3, 5}, {2, 3, 4}, {2, 3000}, {300, 2}, {4, 0}} {
		v := random(rng, numel(sh), 1)
		for axis := range sh {
			compare(t, fmt.Sprintf("birikimli %v eksen %d", sh, axis), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(v, true, sh...)[c]
				return x.CumSum(axis), []*tensor.Tensor{x}
			})
		}
	}
	// Bitişik olmayan görünüm.
	v := random(rng, 12, 1)
	compare(t, "birikimli devrik", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := twin(v, true, 3, 4)[c]
		return x.T().CumSum(1), []*tensor.Tensor{x}
	})
}

// sortData bol eşitlikli, NaN ve işaretli sıfırlı veri.
func sortData(rng *rand.Rand, n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(rng.Intn(7)) - 3
	}
	if n > 3 {
		v[0], v[1], v[2] = float32(math.NaN()), float32(math.Copysign(0, -1)), float32(math.Inf(-1))
	}
	return v
}

func sortCompare(t *testing.T, shapes [][]int) {
	t.Helper()
	rng := rand.New(rand.NewSource(15))
	for _, sh := range shapes {
		v := sortData(rng, numel(sh))
		for axis := range sh {
			ad := fmt.Sprintf("%v eksen %d", sh, axis)
			c, g := twin(v, false, sh...)[0], twin(v, false, sh...)[1]
			checkDiff(t, "sıralama indeksi "+ad, g.ArgSort(axis).Data(), c.ArgSort(axis).Data(), 0)
			for _, k := range []int{1, (sh[axis] + 1) / 2, sh[axis]} {
				compare(t, fmt.Sprintf("en büyük %d %s", k, ad), 0, func(d int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, sh...)[d]
					vals, _ := x.TopK(k, axis)
					return vals, []*tensor.Tensor{x}
				})
				_, ci := c.TopK(k, axis)
				_, gi := g.TopK(k, axis)
				checkDiff(t, fmt.Sprintf("en büyük %d indeks %s", k, ad), gi.Data(), ci.Data(), 0)
			}
		}
	}
}

func TestSort(t *testing.T) {
	sortCompare(t, [][]int{{1}, {2}, {5}, {37}, {4, 9}, {3, 2, 5}, {2, 600}})
}

// TestSortGlobalMemory uzun satırların genel bellek yolu (paylaşımlı bellek sınırı
// sınama için küçültülür).
func TestSortGlobalMemory(t *testing.T) {
	old := sharedSortLimit
	sharedSortLimit = 16
	defer func() { sharedSortLimit = old }()
	sortCompare(t, [][]int{{17}, {3, 40}, {50, 2}})
}

// TestWide16BitDeterministic 16 bitlik depolamada float32'ye açılıp hesaplanır; aynı
// girdi aynı bitleri verir.
func TestWide16BitDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(16))
	v := random(rng, 513, 1)
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, is := range wideUnaries {
			compare(t, is.ad+" "+dt.String(), 1e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(v, false, 513)[c].Cast(dt)
				if is.ad == "asin" || is.ad == "acos" {
					x = x.Clamp(-0.99, 0.99)
				}
				if is.ad == "log2" || is.ad == "log10" || is.ad == "log1p" {
					x = x.Clamp(0.01, 10)
				}
				return is.f(x).Cast(tensor.Float32), nil
			})
		}
		compare(t, "birikimli "+dt.String(), 1e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			return twin(v, false, 513)[c].Cast(dt).CumSum(0).Cast(tensor.Float32), nil
		})
	}
	x := tensor.NewOn(gpuDevice, random(rng, 4*3000, 1), 4, 3000)
	checkDiff(t, "birikimli belirlenimci", x.CumSum(1).Data(), x.CumSum(1).Data(), 0)
	_, i1 := x.TopK(10, 1)
	_, i2 := x.TopK(10, 1)
	checkDiff(t, "en büyük belirlenimci", i1.Data(), i2.Data(), 0)
}

// TestGeluExactAccuracy tam GELU'nun PTX erf/erfc yaklaşımı float64 başvuruya karşı:
// ileri değerin göreli hatası ve türevin mutlak hatası, kuyruklar (x → −∞, Φ'nin göreli
// doğruluğu) dahil. Ölçülen en kötü değerler (benzetim): göreli ileri 2.7e-7, mutlak türev 9e-8.
func TestGeluExactAccuracy(t *testing.T) {
	var xs []float32
	for v := -14.0; v <= 14.0; v += 1.0 / 128 {
		xs = append(xs, float32(v))
	}
	x := tensor.NewOn(gpuDevice, xs, len(xs)).SetRequiresGrad(true)
	y := x.GeluExact()
	y.SumAll().Backward()
	got, grad := y.Data(), x.Grad().Data()
	var worstRel, worstSlope float64
	for i, v := range xs {
		xv := float64(v)
		phi := 0.5 * math.Erfc(-xv/math.Sqrt2)
		want := xv * phi
		slope := phi + xv*math.Exp(-xv*xv/2)/math.Sqrt(2*math.Pi)
		if want != 0 && math.Abs(want) > 1e-30 {
			worstRel = math.Max(worstRel, math.Abs(float64(got[i])-want)/math.Abs(want))
		}
		worstSlope = math.Max(worstSlope, math.Abs(float64(grad[i])-slope))
	}
	t.Logf("gelu_tam: en büyük göreli ileri hata %.3g, en büyük mutlak türev hatası %.3g", worstRel, worstSlope)
	if worstRel > 5e-7 || worstSlope > 2e-7 {
		t.Errorf("gelu_tam doğruluğu: göreli ileri %.3g (sınır 5e-7), mutlak türev %.3g (sınır 2e-7)", worstRel, worstSlope)
	}
}

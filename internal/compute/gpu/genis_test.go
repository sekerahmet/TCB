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

type genisTekli struct {
	ad   string
	f    func(*tensor.Tensor) *tensor.Tensor
	alan func(rng *rand.Rand) float32 // tanım aralığından girdi
}

func normal(olcek float64) func(*rand.Rand) float32 {
	return func(r *rand.Rand) float32 { return float32(r.NormFloat64() * olcek) }
}

func aralik(lo, hi float64) func(*rand.Rand) float32 {
	return func(r *rand.Rand) float32 { return float32(lo + (hi-lo)*r.Float64()) }
}

var genisTekliler = []genisTekli{
	{"silu", (*tensor.Tensor).Silu, normal(4)},
	{"tan", (*tensor.Tensor).Tan, normal(3)},
	{"asin", (*tensor.Tensor).Asin, aralik(-0.99, 0.99)},
	{"acos", (*tensor.Tensor).Acos, aralik(-0.99, 0.99)},
	{"atan", (*tensor.Tensor).Atan, normal(5)},
	{"sinh", (*tensor.Tensor).Sinh, normal(4)},
	{"cosh", (*tensor.Tensor).Cosh, normal(4)},
	{"log1p", (*tensor.Tensor).Log1p, aralik(-0.95, 20)},
	{"log2", (*tensor.Tensor).Log2, aralik(0.01, 100)},
	{"log10", (*tensor.Tensor).Log10, aralik(0.01, 1000)},
	{"expm1", (*tensor.Tensor).Expm1, normal(4)},
	{"taban", (*tensor.Tensor).Floor, normal(5)},
	{"tavan", (*tensor.Tensor).Ceil, normal(5)},
	{"işaret", (*tensor.Tensor).Sign, normal(1)},
	{"mutlak", (*tensor.Tensor).Abs, normal(3)},
}

// TestMutlakSifirdaTurev mutlak değerin türevinin 0'da (±0) tam 0, NaN'da NaN olduğunu ve
// GPU'nun bitleri CPU'nunkiyle aynı verdiğini (−0 → 0, NaN korunur) denetler; 16 bitlik
// girdiler dahil.
func TestMutlakSifirdaTurev(t *testing.T) {
	eksiSifir := float32(math.Copysign(0, -1))
	veri := []float32{0, eksiSifir, -2, 3, float32(math.NaN()), float32(math.Inf(-1)), -1e-40}
	for _, tur := range []tensor.DType{tensor.Float32, tensor.BFloat16, tensor.Float16} {
		karsilastir(t, fmt.Sprintf("mutlak %v", tur), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := yaprakTurde(ikiz(veri, true, len(veri))[c], tur)
			return x.Abs().Mul(tensor.NewOn(x.Device(), []float32{1, 2, 3, 4, 5, 6, 7}, 7).Cast(tur)), []*tensor.Tensor{x}
		})
	}
	ileri := ikiz(veri, false, len(veri))
	a, b := ileri[0].Abs().Data(), ileri[1].Abs().Data()
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) || math.Signbit(float64(b[i])) {
			t.Fatalf("mutlak(%v): GPU %v (%#x), CPU %v (%#x)", veri[i], b[i], math.Float32bits(b[i]), a[i], math.Float32bits(a[i]))
		}
	}
	x := tensor.NewOn(gpuCihaz, veri, len(veri)).SetRequiresGrad(true)
	x.Abs().SumAll().Backward()
	g := x.Grad().Data()
	for i, b := range []float32{0, 0, -1, 1} {
		if g[i] != b || (b == 0 && math.Signbit(float64(g[i]))) {
			t.Fatalf("mutlak türevi %v'de %v, %v bekleniyordu (%v)", veri[i], g[i], b, g)
		}
	}
	if g[4] == g[4] {
		t.Fatalf("NaN'da türev NaN olmalı: %v", g)
	}
}

func TestTekliGenis(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for _, is := range genisTekliler {
		for _, n := range []int{0, 1, 257, 1031} {
			veri := make([]float32, n)
			for i := range veri {
				veri[i] = is.alan(rng)
			}
			karsilastir(t, fmt.Sprintf("%s n=%d", is.ad, n), 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := ikiz(veri, true, n)[c]
				return is.f(x), []*tensor.Tensor{x}
			})
		}
	}
}

// TestTekliGenisOzelDegerler sıfırlar, ±1, alt-normaller, büyük ve küçük sayılar,
// sonsuzlar, NaN ve tanım dışı değerler.
func TestTekliGenisOzelDegerler(t *testing.T) {
	inf := float32(math.Inf(1))
	veri := []float32{0, float32(math.Copysign(0, -1)), 1, -1, 0.5, -0.5, 0.75, -0.75, 1e-40, -1e-40, 1e-30, -1e-30,
		1e-8, -1e-8, 1e-4, -1e-4, 0.3, -0.3, 0.35, -0.35, 2, -2, 1.5707964, -1.5707964, 10, -10, 50, -50, 81, -81,
		88, -88, 89, -89, 100, -100, 1e10, -1e10, 1e30, -1e30, inf, -inf, float32(math.NaN()), 1.5, 2, -2.5, 3}
	for _, is := range genisTekliler {
		v := veri
		if is.ad == "tan" {
			// Tanjantın aralık indirgemesi (sinüs ve kosinüs gibi) |x| ≤ 1e5'te doğrudur.
			v = nil
			for _, x := range veri {
				if !(math.Abs(float64(x)) > 1e5) || math.IsInf(float64(x), 0) {
					v = append(v, x)
				}
			}
		}
		karsilastir(t, is.ad+" özel", 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			return is.f(ikiz(v, false, len(v))[c]), nil
		})
	}
}

func TestAtan2SinirlaKarsilastirma(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	inf, nan := float32(math.Inf(1)), float32(math.NaN())
	eksiSifir := float32(math.Copysign(0, -1))
	ys := []float32{1, 1, -1, -1, 0, 0, eksiSifir, eksiSifir, 0, inf, inf, -inf, 3, nan, 2, 1e-30, 1e30}
	xs := []float32{1, -1, 1, -1, 1, -1, 1, -1, eksiSifir, inf, -5, -inf, -inf, 1, 0, 1e-30, -1e30}
	karsilastir(t, "atan2 özel", 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		return tensor.Atan2(ikiz(ys, false, len(ys))[c], ikiz(xs, false, len(xs))[c]), nil
	})
	for _, sh := range [][2][]int{{{37}, {37}}, {{5, 1}, {1, 7}}, {{3, 1, 4}, {2, 1}}, {{1}, {6, 3}}} {
		av, bv := rastgele(rng, numel(sh[0]), 2), rastgele(rng, numel(sh[1]), 2)
		karsilastir(t, fmt.Sprintf("atan2 %v %v", sh[0], sh[1]), 3e-6, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			a, b := ikiz(av, true, sh[0]...)[c], ikiz(bv, true, sh[1]...)[c]
			return tensor.Atan2(a, b), []*tensor.Tensor{a, b}
		})
		for _, op := range []tensor.BinOp{tensor.OpLt, tensor.OpGt, tensor.OpLe, tensor.OpGe} {
			karsilastir(t, fmt.Sprintf("karşılaştırma %d %v", op, sh), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				return tensor.Compare(ikiz(av, false, sh[0]...)[c], ikiz(bv, false, sh[1]...)[c], op), nil
			})
		}
	}
	v := append(rastgele(rng, 300, 2), nan, inf, -inf, -1, 1)
	karsilastir(t, "sınırla", 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := ikiz(v, true, len(v))[c]
		return x.Clamp(-1, 1), []*tensor.Tensor{x}
	})
	karsilastir(t, "sınırla alt", 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := ikiz(v, true, len(v))[c]
		return x.ClampMin(0.5), []*tensor.Tensor{x}
	})
}

func TestKosulluSecim(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for _, sh := range [][3][]int{{{41}, {41}, {41}}, {{4, 1}, {1, 5}, {4, 5}}, {{2, 3, 1}, {3, 4}, {1}}, {{6}, {1}, {6}}} {
		cv := rastgele(rng, numel(sh[0]), 1)
		for i := range cv {
			if cv[i] < 0 {
				cv[i] = 0
			}
		}
		if len(cv) > 2 {
			cv[1] = float32(math.NaN()) // NaN koşul doğru sayılır
		}
		av, bv := rastgele(rng, numel(sh[1]), 1), rastgele(rng, numel(sh[2]), 1)
		karsilastir(t, fmt.Sprintf("koşullu %v", sh), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			k := ikiz(cv, false, sh[0]...)[c]
			a, b := ikiz(av, true, sh[1]...)[c], ikiz(bv, true, sh[2]...)[c]
			return tensor.Where(k, a, b), []*tensor.Tensor{a, b}
		})
	}
}

func TestBirikimliToplam(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	for _, sh := range [][]int{{1}, {37}, {3, 5}, {2, 3, 4}, {2, 3000}, {300, 2}, {4, 0}} {
		v := rastgele(rng, numel(sh), 1)
		for axis := range sh {
			karsilastir(t, fmt.Sprintf("birikimli %v eksen %d", sh, axis), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := ikiz(v, true, sh...)[c]
				return x.CumSum(axis), []*tensor.Tensor{x}
			})
		}
	}
	// Bitişik olmayan görünüm.
	v := rastgele(rng, 12, 1)
	karsilastir(t, "birikimli devrik", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := ikiz(v, true, 3, 4)[c]
		return x.T().CumSum(1), []*tensor.Tensor{x}
	})
}

// siralamaVerisi bol eşitlikli, NaN ve işaretli sıfırlı veri.
func siralamaVerisi(rng *rand.Rand, n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(rng.Intn(7)) - 3
	}
	if n > 3 {
		v[0], v[1], v[2] = float32(math.NaN()), float32(math.Copysign(0, -1)), float32(math.Inf(-1))
	}
	return v
}

func siralamaKarsilastir(t *testing.T, sekiller [][]int) {
	t.Helper()
	rng := rand.New(rand.NewSource(15))
	for _, sh := range sekiller {
		v := siralamaVerisi(rng, numel(sh))
		for axis := range sh {
			ad := fmt.Sprintf("%v eksen %d", sh, axis)
			c, g := ikiz(v, false, sh...)[0], ikiz(v, false, sh...)[1]
			farkDenetle(t, "sıralama indeksi "+ad, g.ArgSort(axis).Data(), c.ArgSort(axis).Data(), 0)
			for _, k := range []int{1, (sh[axis] + 1) / 2, sh[axis]} {
				karsilastir(t, fmt.Sprintf("en büyük %d %s", k, ad), 0, func(d int) (*tensor.Tensor, []*tensor.Tensor) {
					x := ikiz(v, true, sh...)[d]
					vals, _ := x.TopK(k, axis)
					return vals, []*tensor.Tensor{x}
				})
				_, ci := c.TopK(k, axis)
				_, gi := g.TopK(k, axis)
				farkDenetle(t, fmt.Sprintf("en büyük %d indeks %s", k, ad), gi.Data(), ci.Data(), 0)
			}
		}
	}
}

func TestSiralama(t *testing.T) {
	siralamaKarsilastir(t, [][]int{{1}, {2}, {5}, {37}, {4, 9}, {3, 2, 5}, {2, 600}})
}

// TestSiralamaGenelBellek uzun satırların genel bellek yolu (paylaşımlı bellek sınırı
// sınama için küçültülür).
func TestSiralamaGenelBellek(t *testing.T) {
	eski := ortakSiralamaSiniri
	ortakSiralamaSiniri = 16
	defer func() { ortakSiralamaSiniri = eski }()
	siralamaKarsilastir(t, [][]int{{17}, {3, 40}, {50, 2}})
}

// TestGenis16BitVeBelirlenimci 16 bitlik depolamada float32'ye açılıp hesaplanır; aynı
// girdi aynı bitleri verir.
func TestGenis16BitVeBelirlenimci(t *testing.T) {
	rng := rand.New(rand.NewSource(16))
	v := rastgele(rng, 513, 1)
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, is := range genisTekliler {
			karsilastir(t, is.ad+" "+dt.String(), 1e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := ikiz(v, false, 513)[c].Cast(dt)
				if is.ad == "asin" || is.ad == "acos" {
					x = x.Clamp(-0.99, 0.99)
				}
				if is.ad == "log2" || is.ad == "log10" || is.ad == "log1p" {
					x = x.Clamp(0.01, 10)
				}
				return is.f(x).Cast(tensor.Float32), nil
			})
		}
		karsilastir(t, "birikimli "+dt.String(), 1e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			return ikiz(v, false, 513)[c].Cast(dt).CumSum(0).Cast(tensor.Float32), nil
		})
	}
	x := tensor.NewOn(gpuCihaz, rastgele(rng, 4*3000, 1), 4, 3000)
	farkDenetle(t, "birikimli belirlenimci", x.CumSum(1).Data(), x.CumSum(1).Data(), 0)
	_, i1 := x.TopK(10, 1)
	_, i2 := x.TopK(10, 1)
	farkDenetle(t, "en büyük belirlenimci", i1.Data(), i2.Data(), 0)
}

package gpu

import (
	"fmt"
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Özel değerler (dil tanımı 15.50): ±0, ±∞ ve NaN girdide her tensör işlemi CPU'da ve GPU'da
// aynı sonucu verir (sıfırın işareti dahil; türevlerde sıfırın işareti belirtilmez) ve
// tablodaki kurallara uyar.

var (
	posInf  = float32(math.Inf(1))
	negInf  = float32(math.Inf(-1))
	qNaN    = float32(math.NaN())
	negZero = float32(math.Copysign(0, -1))
	devices = [2]tensor.Device{tensor.CPU, gpuDevice}
)

// specialInputs tekli işlemlerin girdileri: sıfırlar, sonsuzlar, NaN, sıradan sayılar, alt
// normaller ve büyük sayılar. float32'nin ucundaki sayılarda (3e38) ara sonuç taşabilir
// (x·ln10, x², toplam); oradaki farklar sayısal doğruluk işidir (yol haritası Evre 4, madde 8).
var specialInputs = []float32{0, negZero, posInf, negInf, qNaN, 1, -1, 0.5, -2.5, 1e-40, -1e-40, 1e30, -1e30}

// trigInputs trigonometrik işlemler için: aralık indirgemesi GPU'da |x| ≤ 1e5'te doğrudur
// (büyük açılar sayısal doğruluk işidir, özel değer değil).
var trigInputs = []float32{0, negZero, posInf, negInf, qNaN, 1, -1, 0.5, -2.5, 1e-40, -1e-40, 1e4, -1e4}

// specialRows iki elemanlı satırlar: indirgemelerin, softmax'ın ve normların özel durumları.
var specialRows = [][]float32{{negZero, negZero}, {0, negZero}, {negZero, 0}, {0, 0}, {posInf, 1}, {posInf, negInf},
	{qNaN, 1}, {1, qNaN}, {negInf, negInf}, {posInf, posInf}, {negInf, 1}, {1e-40, 1e-40}, {qNaN, posInf},
	{posInf, qNaN}, {qNaN, negInf}, {negInf, qNaN}, {1, -1}}

// sameSpecial a ile b aynı mı: ikisi de NaN, ya da aynı sınıf (±∞, ±0; signedZero değilse
// sıfırın işareti önemsiz) ve sonluysa göreli tol içinde.
func sameSpecial(a, b float32, signedZero bool, tol float64) bool {
	x, y := float64(a), float64(b)
	switch {
	case math.IsNaN(x) || math.IsNaN(y):
		return math.IsNaN(x) && math.IsNaN(y)
	case math.IsInf(x, 0) || math.IsInf(y, 0):
		return x == y
	case x == 0 || y == 0:
		return x == y && (!signedZero || math.Signbit(x) == math.Signbit(y))
	}
	return math.Abs(x-y) <= tol*math.Max(math.Abs(x), math.Abs(y))
}

func show(v float32) string {
	if v == 0 && math.Signbit(float64(v)) {
		return "-0"
	}
	return fmt.Sprint(v)
}

// specialCase bir işlemin cihazdaki sonucunu (ileri ya da türev) üretir.
type specialCase struct {
	ad   string
	in   []float32 // hata iletisi için (eleman bazlı işlemlerde i. girdi)
	grad bool      // türev: sıfırın işareti önemsiz
	run  func(c int) []float32
}

func onDevice(c int, v []float32, grad bool, shape ...int) *tensor.Tensor {
	return tensor.NewOn(devices[c], v, shape...).SetRequiresGrad(grad)
}

// seedOf y şeklinde tohum: birler (ones) ya da 1, 2, 3, 1, 2, 3, ...
func seedOf(y *tensor.Tensor, ones bool) *tensor.Tensor {
	w := make([]float32, y.Numel())
	for i := range w {
		w[i] = 1
		if !ones {
			w[i] = float32(1 + i%3)
		}
	}
	return tensor.NewOn(y.Device(), w, y.Shape()...)
}

// gradOf f'nin x'e göre türevi (tohum seedOf); türevsiz işlemde nil.
func gradOf(c int, v []float32, shape []int, ones bool, f func(*tensor.Tensor) *tensor.Tensor) []float32 {
	x := onDevice(c, v, true, shape...)
	y := f(x)
	y.BackwardWith(seedOf(y, ones))
	if x.Grad() == nil {
		return nil
	}
	return x.Grad().Data()
}

type namedUnary struct {
	ad string
	f  func(*tensor.Tensor) *tensor.Tensor
}

var specialUnaries = []namedUnary{
	{"üstel", (*tensor.Tensor).Exp}, {"log", (*tensor.Tensor).Log}, {"karekök", (*tensor.Tensor).Sqrt},
	{"tanh", (*tensor.Tensor).Tanh}, {"relu", (*tensor.Tensor).Relu}, {"gelu", (*tensor.Tensor).Gelu},
	{"gelu_tam", (*tensor.Tensor).GeluExact}, {"sigmoid", (*tensor.Tensor).Sigmoid}, {"silu", (*tensor.Tensor).Silu},
	{"arksinüs", (*tensor.Tensor).Asin}, {"arkkosinüs", (*tensor.Tensor).Acos}, {"arktanjant", (*tensor.Tensor).Atan},
	{"sinh", (*tensor.Tensor).Sinh}, {"cosh", (*tensor.Tensor).Cosh}, {"log1p", (*tensor.Tensor).Log1p},
	{"log2", (*tensor.Tensor).Log2}, {"log10", (*tensor.Tensor).Log10}, {"expm1", (*tensor.Tensor).Expm1},
	{"taban", (*tensor.Tensor).Floor}, {"tavan", (*tensor.Tensor).Ceil}, {"işaret", (*tensor.Tensor).Sign},
	{"mutlak", (*tensor.Tensor).Abs}, {"eksi", (*tensor.Tensor).Neg},
}

var specialTrig = []namedUnary{{"sinüs", (*tensor.Tensor).Sin}, {"kosinüs", (*tensor.Tensor).Cos}, {"tanjant", (*tensor.Tensor).Tan}}

// specialCases tablonun bütün satırlarını (ileri ve türev) kurar.
func specialCases() []specialCase {
	var cs []specialCase
	elem := func(ad string, in []float32, f func(*tensor.Tensor) *tensor.Tensor) {
		cs = append(cs, specialCase{ad: ad, in: in, run: func(c int) []float32 { return f(onDevice(c, in, false, len(in))).Data() }})
		cs = append(cs, specialCase{ad: ad + " türevi", in: in, grad: true, run: func(c int) []float32 {
			return gradOf(c, in, []int{len(in)}, true, f)
		}})
	}
	for _, u := range specialUnaries {
		elem(u.ad, specialInputs, u.f)
	}
	for _, u := range specialTrig {
		elem(u.ad, trigInputs, u.f)
	}
	for _, p := range []float32{0, negZero, 1, -1, posInf, qNaN} {
		elem(fmt.Sprintf("x+%v", show(p)), specialInputs, func(x *tensor.Tensor) *tensor.Tensor { return x.AddScalar(p) })
		elem(fmt.Sprintf("x·%v", show(p)), specialInputs, func(x *tensor.Tensor) *tensor.Tensor { return x.MulScalar(p) })
		elem(fmt.Sprintf("%v−x", show(p)), specialInputs, func(x *tensor.Tensor) *tensor.Tensor { return x.RSubScalar(p) })
		elem(fmt.Sprintf("%v/x", show(p)), specialInputs, func(x *tensor.Tensor) *tensor.Tensor { return x.RDivScalar(p) })
	}
	for _, p := range []float32{0, 1, 2, 3, 0.5, -1, -2, 2.5, posInf, negInf, qNaN} {
		elem(fmt.Sprintf("x^%v", p), specialInputs, func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(p) })
	}
	for _, lh := range [][2]float32{{0, 1}, {negZero, 1}, {-1, 0}, {-1, negZero}, {0, 0}, {negInf, 0}, {0, posInf}} {
		elem(fmt.Sprintf("sınırla[%v, %v]", show(lh[0]), show(lh[1])), specialInputs,
			func(x *tensor.Tensor) *tensor.Tensor { return x.Clamp(lh[0], lh[1]) })
	}
	// İkili: bütün çiftler.
	var A, B []float32
	for _, a := range specialInputs {
		for _, b := range specialInputs {
			A, B = append(A, a), append(B, b)
		}
	}
	for _, b := range []struct {
		ad string
		f  func(x, y *tensor.Tensor) *tensor.Tensor
	}{
		{"+", tensor.Add}, {"−", tensor.Sub}, {"·", tensor.Mul}, {"/", tensor.Div}, {"atan2", tensor.Atan2},
		{"<", func(x, y *tensor.Tensor) *tensor.Tensor { return tensor.Compare(x, y, tensor.OpLt) }},
		{">", func(x, y *tensor.Tensor) *tensor.Tensor { return tensor.Compare(x, y, tensor.OpGt) }},
		{"<=", func(x, y *tensor.Tensor) *tensor.Tensor { return tensor.Compare(x, y, tensor.OpLe) }},
		{">=", func(x, y *tensor.Tensor) *tensor.Tensor { return tensor.Compare(x, y, tensor.OpGe) }},
		{"koşullu", func(x, y *tensor.Tensor) *tensor.Tensor { return tensor.Where(x.Detach(), y, y.MulScalar(2)) }},
	} {
		cs = append(cs, specialCase{ad: "a " + b.ad + " b", run: func(c int) []float32 {
			return b.f(onDevice(c, A, false, len(A)), onDevice(c, B, false, len(B))).Data()
		}})
		for side := 0; side < 2; side++ {
			cs = append(cs, specialCase{ad: fmt.Sprintf("a %s b türevi %d", b.ad, side), grad: true, run: func(c int) []float32 {
				x, y := onDevice(c, A, true, len(A)), onDevice(c, B, true, len(B))
				z := b.f(x, y)
				if !z.RequiresGrad() {
					return nil
				}
				z.BackwardWith(seedOf(z, true))
				if g := []*tensor.Tensor{x, y}[side].Grad(); g != nil {
					return g.Data()
				}
				return nil
			}})
		}
	}
	// Satır ve sütun indirgemeleri, softmax, sıralama, normlar.
	var R []float32
	for _, r := range specialRows {
		R = append(R, r...)
	}
	rs := []int{len(specialRows), 2}
	RT := make([]float32, len(R)) // devrik: sütun indirgemesi yolu
	for i := range specialRows {
		RT[i], RT[len(specialRows)+i] = R[2*i], R[2*i+1]
	}
	rows := func(ad string, v []float32, shape []int, f func(*tensor.Tensor) *tensor.Tensor) {
		cs = append(cs, specialCase{ad: ad, run: func(c int) []float32 { return f(onDevice(c, v, false, shape...)).Data() }})
		cs = append(cs, specialCase{ad: ad + " türevi", grad: true, run: func(c int) []float32 {
			return gradOf(c, v, shape, false, f)
		}})
	}
	for _, r := range []namedUnary{
		{"toplam", func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(1, false) }},
		{"ortalama", func(x *tensor.Tensor) *tensor.Tensor { return x.Mean(1, false) }},
		{"en_büyük", func(x *tensor.Tensor) *tensor.Tensor { return x.Max(1, false) }},
		{"softmax", func(x *tensor.Tensor) *tensor.Tensor { return x.Softmax(1) }},
		{"log_softmax", func(x *tensor.Tensor) *tensor.Tensor { return x.LogSoftmax(1) }},
		{"birikimli_toplam", func(x *tensor.Tensor) *tensor.Tensor { return x.CumSum(1) }},
		{"en_büyük_k", func(x *tensor.Tensor) *tensor.Tensor { v, _ := x.TopK(1, 1); return v }},
		{"rms_norm", func(x *tensor.Tensor) *tensor.Tensor { return tensor.RMSNorm(x, nil, 1e-6) }},
		{"katman_norm", func(x *tensor.Tensor) *tensor.Tensor { return tensor.LayerNorm(x, nil, nil, 1e-5) }},
	} {
		rows(r.ad+" (satır)", R, rs, r.f)
	}
	cs = append(cs, specialCase{ad: "en_büyük indeksi", run: func(c int) []float32 {
		return intsAsFloats(onDevice(c, R, false, rs...).ArgMax(1))
	}}, specialCase{ad: "sıralama_indeksi", run: func(c int) []float32 {
		return onDevice(c, R, false, rs...).ArgSort(1).Data()
	}}, specialCase{ad: "en_büyük_k indeksi", run: func(c int) []float32 {
		_, i := onDevice(c, R, false, rs...).TopK(2, 1)
		return i.Data()
	}})
	for _, r := range []namedUnary{
		{"toplam", func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(0, false) }},
		{"ortalama", func(x *tensor.Tensor) *tensor.Tensor { return x.Mean(0, false) }},
		{"en_büyük", func(x *tensor.Tensor) *tensor.Tensor { return x.Max(0, false) }},
	} {
		rows(r.ad+" (sütun)", RT, []int{2, len(specialRows)}, r.f)
	}
	// Tek elemanlı satırlar, hepsinin toplamı ve uzun (bloklu ve iki aşamalı) yollar.
	rows("toplam (tek eleman)", specialInputs, []int{len(specialInputs), 1}, func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(1, false) })
	rows("softmax (tek eleman)", specialInputs, []int{len(specialInputs), 1}, func(x *tensor.Tensor) *tensor.Tensor { return x.Softmax(1) })
	for _, n := range []int{1, 3, 3000, 70000} {
		v := make([]float32, n)
		for i := range v {
			v[i] = negZero
		}
		rows(fmt.Sprintf("hepsinin toplamı (%d × -0)", n), v, []int{n}, (*tensor.Tensor).SumAll)
		rows(fmt.Sprintf("hepsinin ortalaması (%d × -0)", n), v, []int{n}, (*tensor.Tensor).MeanAll)
		rows(fmt.Sprintf("satır toplamı (2 × %d × -0)", n), append(v, v...), []int{2, n}, func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(1, false) })
		rows(fmt.Sprintf("sütun toplamı (%d × 2 × -0)", n), append(v, v...), []int{n, 2}, func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(0, false) })
	}
	// Matris çarpımı ve çapraz entropi.
	ma := []float32{negZero, negZero, 0, posInf, negZero, 1, qNaN, 0, posInf, negInf}
	for _, mb := range [][]float32{{1, 0, 1, 0}, {1, 1, negZero, 1}} {
		cs = append(cs, specialCase{ad: fmt.Sprintf("@ %v", mb), run: func(c int) []float32 {
			return tensor.MatMul(onDevice(c, ma, false, 5, 2), onDevice(c, mb, false, 2, 2)).Data()
		}})
	}
	for _, row := range [][]float32{{posInf, 0, 0}, {negInf, 0, 0}, {qNaN, 0, 0}, {0, negInf, negInf}, {negInf, negInf, negInf}} {
		for _, tgt := range []int{0, 1} {
			ce := func(x *tensor.Tensor) *tensor.Tensor { return tensor.CrossEntropy(x, []int{tgt}) }
			ad := fmt.Sprintf("çapraz_entropi %v hedef %d", row, tgt)
			cs = append(cs, specialCase{ad: ad, run: func(c int) []float32 { return ce(onDevice(c, row, false, 1, 3)).Data() }},
				specialCase{ad: ad + " türevi", grad: true, run: func(c int) []float32 { return gradOf(c, row, []int{1, 3}, true, ce) }})
		}
	}
	return cs
}

func intsAsFloats(v []int) []float32 {
	f := make([]float32, len(v))
	for i, x := range v {
		f[i] = float32(x)
	}
	return f
}

// Kural: ±0, ±∞ ve NaN girdide her işlemin CPU ve GPU sonucu aynıdır: NaN'lar NaN, sonsuzlar
// aynı işaretli sonsuz, sıfırlar aynı işaretli sıfır (türevlerde işaret önemsiz), sonlular
// yaklaşık hesabın yuvarlaması içinde.
func TestSpecialValuesSameOnBothDevices(t *testing.T) {
	for _, s := range specialCases() {
		cpu, gpu := s.run(0), s.run(1)
		if len(cpu) != len(gpu) {
			t.Errorf("%s: uzunluk CPU %d, GPU %d", s.ad, len(cpu), len(gpu))
			continue
		}
		for i := range cpu {
			if !sameSpecial(cpu[i], gpu[i], !s.grad, 2e-5) {
				in := ""
				if i < len(s.in) {
					in = " (girdi " + show(s.in[i]) + ")"
				}
				t.Errorf("%s, %d. eleman%s: CPU %s, GPU %s", s.ad, i, in, show(cpu[i]), show(gpu[i]))
			}
		}
	}
}

// specialRule tablonun bir satırı: işlem girdilere uygulanınca beklenen değerler (sıfırın
// işareti dahil) iki cihazda da çıkar.
type specialRule struct {
	ad       string
	in, want []float32
	f        func(*tensor.Tensor) *tensor.Tensor
	shape    []int // boşsa [len(in)]
	grad     bool  // f'nin türevi (tohum 1) beklenir
	anyZero  bool  // sıfırın işareti önemsiz
}

// Kural: dil tanımı 15.50'nin tablosu. Her satır tablodaki bir kuralın örnekleridir; değerler
// C99 Ek F'den (üs, tek işlevler, doyma) ya da tablodaki bilinçli seçimden gelir.
func TestSpecialValuesTable(t *testing.T) {
	n0, inf, nan := negZero, posInf, qNaN
	rules := []specialRule{
		// Tek işlevler sıfırın işaretini korur: f(±0) = ±0.
		{ad: "sinüs", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Sin},
		{ad: "tanjant", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Tan},
		{ad: "tanh", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Tanh},
		{ad: "arksinüs", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Asin},
		{ad: "arktanjant", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Atan},
		{ad: "sinh", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Sinh},
		{ad: "expm1", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Expm1},
		{ad: "log1p", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Log1p},
		{ad: "karekök", in: []float32{0, n0, -1}, want: []float32{0, n0, nan}, f: (*tensor.Tensor).Sqrt},
		{ad: "silu", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Silu},
		{ad: "gelu", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).Gelu},
		{ad: "gelu_tam", in: []float32{0, n0}, want: []float32{0, n0}, f: (*tensor.Tensor).GeluExact},
		{ad: "işaret", in: []float32{0, n0, nan, -3, inf}, want: []float32{0, n0, nan, -1, 1}, f: (*tensor.Tensor).Sign},
		{ad: "relu", in: []float32{0, n0, nan, -inf}, want: []float32{0, 0, nan, 0}, f: (*tensor.Tensor).Relu},
		{ad: "mutlak", in: []float32{n0, -inf, nan}, want: []float32{0, inf, nan}, f: (*tensor.Tensor).Abs},
		// Doyma ve tanım dışı.
		{ad: "tanh", in: []float32{inf, -inf, 10, -10}, want: []float32{1, -1, 1, -1}, f: (*tensor.Tensor).Tanh},
		{ad: "sigmoid", in: []float32{inf, -inf}, want: []float32{1, 0}, f: (*tensor.Tensor).Sigmoid},
		{ad: "üstel", in: []float32{inf, -inf, n0}, want: []float32{inf, 0, 1}, f: (*tensor.Tensor).Exp},
		{ad: "log", in: []float32{0, n0, -1, inf}, want: []float32{-inf, -inf, nan, inf}, f: (*tensor.Tensor).Log},
		// Limitler: silu ve gelu −∞'da 0, +∞'da +∞; türevleri 0 ve 1.
		{ad: "silu", in: []float32{inf, -inf}, want: []float32{inf, 0}, f: (*tensor.Tensor).Silu, anyZero: true},
		{ad: "gelu", in: []float32{inf, -inf}, want: []float32{inf, 0}, f: (*tensor.Tensor).Gelu, anyZero: true},
		{ad: "gelu_tam", in: []float32{inf, -inf}, want: []float32{inf, 0}, f: (*tensor.Tensor).GeluExact, anyZero: true},
		{ad: "silu", in: []float32{inf, -inf}, want: []float32{1, 0}, f: (*tensor.Tensor).Silu, grad: true},
		{ad: "gelu", in: []float32{inf, -inf}, want: []float32{1, 0}, f: (*tensor.Tensor).Gelu, grad: true},
		{ad: "gelu_tam", in: []float32{inf, -inf}, want: []float32{1, 0}, f: (*tensor.Tensor).GeluExact, grad: true},
		// Türevler: relu'nun türevi 0'da ve NaN'da 0, mutlak'ınki ±0'da 0 ve NaN'da NaN, tanh'ınki
		// ±∞'da 0.
		{ad: "relu", in: []float32{0, n0, nan, 2}, want: []float32{0, 0, 0, 1}, f: (*tensor.Tensor).Relu, grad: true},
		{ad: "mutlak", in: []float32{0, n0, nan, -2}, want: []float32{0, 0, nan, -1}, f: (*tensor.Tensor).Abs, grad: true},
		{ad: "tanh", in: []float32{inf, -inf}, want: []float32{0, 0}, f: (*tensor.Tensor).Tanh, grad: true},
		// Toplam: yalnızca −0'lardan oluşan toplam +0'dır; +∞ ile −∞ NaN.
		{ad: "toplam", in: []float32{n0, n0, 0, n0, n0, 0, inf, -inf, nan, 1}, shape: []int{5, 2},
			want: []float32{0, 0, 0, nan, nan}, f: func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(1, false) }},
		{ad: "toplam (tek eleman)", in: []float32{n0}, shape: []int{1, 1}, want: []float32{0},
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.Sum(1, false) }},
		{ad: "hepsinin toplamı", in: []float32{n0, n0, n0}, want: []float32{0}, f: (*tensor.Tensor).SumAll},
		{ad: "ortalama", in: []float32{n0, n0}, shape: []int{1, 2}, want: []float32{0},
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.Mean(1, false) }},
		{ad: "birikimli_toplam", in: []float32{n0, n0}, shape: []int{1, 2}, want: []float32{0, 0},
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.CumSum(1) }},
		// En büyük: NaN yayılır; eşitlerde (−0 ile +0 eşit) ilk eleman.
		{ad: "en_büyük", in: []float32{1, nan, negZero, 0, 0, negZero, -inf, 1}, shape: []int{4, 2},
			want: []float32{nan, n0, 0, 1}, f: func(x *tensor.Tensor) *tensor.Tensor { return x.Max(1, false) }},
		// Tamamen −∞ (maskeli) satır: softmax 0, log_softmax −∞; +∞ ya da NaN'lı satır NaN.
		{ad: "softmax", in: []float32{-inf, -inf, -inf, 1, inf, 1, nan, 1}, shape: []int{4, 2},
			want: []float32{0, 0, 0, 1, nan, nan, nan, nan}, f: func(x *tensor.Tensor) *tensor.Tensor { return x.Softmax(1) }},
		{ad: "log_softmax", in: []float32{-inf, -inf}, shape: []int{1, 2}, want: []float32{-inf, -inf},
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.LogSoftmax(1) }},
		{ad: "softmax", in: []float32{-inf, -inf}, shape: []int{1, 2}, want: []float32{0, 0}, grad: true,
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.Softmax(1) }},
		// Sınırlama NaN'ı korur; türevi sınırlar dahil aralıkta 1.
		{ad: "sınırla", in: []float32{nan, -inf, inf, n0}, want: []float32{nan, -1, 1, n0},
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.Clamp(-1, 1) }},
		{ad: "sınırla", in: []float32{-1, 1, 0, 2, nan}, want: []float32{1, 1, 1, 0, 0}, grad: true,
			f: func(x *tensor.Tensor) *tensor.Tensor { return x.Clamp(-1, 1) }},
		// Karşılaştırmalar NaN'da 0; koşullu'da NaN koşul doğrudur.
		{ad: "NaN < 1, NaN >= NaN", in: []float32{nan, nan}, want: []float32{0, 0},
			f: func(x *tensor.Tensor) *tensor.Tensor {
				return tensor.Compare(x, tensor.NewOn(x.Device(), []float32{1, nan}, 2), tensor.OpLt).
					Add(tensor.Compare(x, x, tensor.OpGe))
			}},
		{ad: "koşullu", in: []float32{nan, 0, n0}, want: []float32{1, 2, 2},
			f: func(x *tensor.Tensor) *tensor.Tensor {
				return tensor.Where(x, tensor.FullOn(x.Device(), 1, 3), tensor.FullOn(x.Device(), 2, 3))
			}},
	}
	// Üs: C99 Ek F (x^0 = 1 ve 1^p = 1 NaN'da da; −0 ve −∞ tek üste işaretini korur; sonlu
	// negatif taban, tam olmayan üs NaN).
	pow := func(p float32, in, want []float32) specialRule {
		return specialRule{ad: fmt.Sprintf("x^%v", p), in: in, want: want, f: func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(p) }}
	}
	rules = append(rules,
		pow(0, []float32{nan, inf, n0}, []float32{1, 1, 1}),
		pow(3, []float32{n0, -inf, -2}, []float32{n0, -inf, -8}),
		pow(2.5, []float32{n0, -inf, -2, 0}, []float32{0, inf, nan, 0}),
		pow(0.5, []float32{n0, -inf, inf}, []float32{0, inf, inf}),
		pow(-1, []float32{0, n0, -inf, inf}, []float32{inf, -inf, n0, 0}),
		pow(-2, []float32{0, n0, -inf}, []float32{inf, inf, 0}),
		pow(posInf, []float32{1, -1, 0.5, 2, -2, nan}, []float32{1, 1, 0, inf, inf, nan}),
		pow(negInf, []float32{1, -1, 0.5, 2, 0}, []float32{1, 1, inf, 0, inf}),
		pow(qNaN, []float32{1, -1, 2}, []float32{1, nan, nan}),
	)
	for _, r := range rules {
		shape := r.shape
		if shape == nil {
			shape = []int{len(r.in)}
		}
		for c := range devices {
			var got []float32
			if r.grad {
				got = gradOf(c, r.in, shape, true, r.f)
			} else {
				got = r.f(onDevice(c, r.in, false, shape...)).Data()
			}
			if len(got) != len(r.want) {
				t.Errorf("%s (%s%s): %d sonuç, %d beklenen", r.ad, devices[c], gradNote(r.grad), len(got), len(r.want))
				continue
			}
			for i := range got {
				if !sameSpecial(got[i], r.want[i], !r.grad && !r.anyZero, 1e-6) {
					t.Errorf("%s (%s%s), %d. sonuç: %s, beklenen %s", r.ad, devices[c], gradNote(r.grad), i, show(got[i]), show(r.want[i]))
				}
			}
		}
	}
}

func gradNote(grad bool) string {
	if grad {
		return ", türev"
	}
	return ""
}

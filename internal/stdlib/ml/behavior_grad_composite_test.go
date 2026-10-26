package ml

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Bileşik türevlerin gradyan kapısı. Bu paketin yerleşikleri tensör işlemlerini birleştirir
// (karışık hassasiyette girdiyi float32'ye açıp ya da 16 bite çevirip işlemi çağırmak, min'i
// -max(-x) olarak kurmak, listeyi toplamak): türev, tek tek işlemlerin türevlerinin zinciridir
// ve zincirin kendisi sınanır. Düzen tensör paketinin gradyan kapısıyla aynıdır
// (internal/compute/tensor/helpers_test.go): float64 başvurunun merkezi sonlu farkı, bf16
// yuvarlamasının ölçülmüş toleransı. Paketler arası test yardımcısı paylaşılamadığından
// (_test.go dosyaları içe aktarılamaz) motor burada küçük bir kopyadır; tolerans kuralı aynıdır.
// Kapsam denetimi: behavior_grad_composite_coverage_test.go.

// compositeSpec bir bileşik yerleşiğin kaydıdır.
type compositeSpec struct {
	name    string
	funcs   []string // kapsadığı Go fonksiyonları (kapsam denetimi bunlara bakar)
	shapes  [][]int  // girdi şekilleri; girdiler Randn(şekil)
	run     func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error)
	ref     func(in [][]float64) []float64
	bf16Tol float64 // karışık hassasiyette (bf16) sapma toleransı: ölçülmüş en büyüğün 2 katı
}

const (
	compositeF64Tol   = 2e-4
	compositeFwdTol   = 1e-4
	compositeSeeds    = 4  // kapının denediği tohum sayısı
	compositeMeasured = 32 // ölçümün denediği tohum sayısı
	compositeFloor    = 4e-3
)

func (s compositeSpec) rng(k int) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(s.name))
	return rand.New(rand.NewSource(int64(h.Sum64()>>1) + int64(k)*7919))
}

func (s compositeSpec) inputs(k int) []*tensor.Tensor {
	r := s.rng(k)
	xs := make([]*tensor.Tensor, len(s.shapes))
	for i, sh := range s.shapes {
		xs[i] = tensor.Randn(r, sh...)
	}
	return xs
}

// compositeVM bir VM kurar; bf16 doğruysa karışık_hassasiyet("bf16") bloğundaki gibi çalışır.
func compositeVM(bf16 bool) *interp.VM {
	m := interp.New()
	if bf16 {
		m.SetTaskValue(mixedKey{}, tensor.BFloat16)
	}
	return m
}

// callBuiltin kodlu yerleşiği çağırır ve tensör sonucunu verir.
func callBuiltin(m *interp.VM, kd codes.Code, args ...interp.Value) (*tensor.Tensor, error) {
	v, err := m.Builtin(kd).Fn(m, args, nil)
	if err != nil {
		return nil, err
	}
	d, ok := v.(*TensorValue)
	if !ok {
		return nil, fmt.Errorf("tensör beklendi: %s", v.Inspect())
	}
	return d.t, nil
}

func tv(t *tensor.Tensor) interp.Value { return New(t) }

func f64s(xs []*tensor.Tensor) [][]float64 {
	in := make([][]float64, len(xs))
	for k, x := range xs {
		for _, v := range x.Data() {
			in[k] = append(in[k], float64(v))
		}
	}
	return in
}

func dot64(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// checkCompositeFloat64 analitik gradyanı başvurunun float64 merkezi sonlu farkıyla karşılaştırır.
func checkCompositeFloat64(t *testing.T, s compositeSpec) {
	xs := s.inputs(0)
	for _, x := range xs {
		x.SetRequiresGrad(true)
	}
	out, err := s.run(compositeVM(false), xs)
	if err != nil {
		t.Fatal(err)
	}
	in := f64s(xs)
	want := s.ref(in)
	got := out.Data()
	if len(got) != len(want) {
		t.Fatalf("ileri değer uzunluğu %d, başvuru %d", len(got), len(want))
	}
	for i := range got {
		if math.IsNaN(float64(got[i])) || math.Abs(float64(got[i])-want[i]) > compositeFwdTol*(1+math.Abs(want[i])) {
			t.Fatalf("ileri değer [%d] = %v, başvuru %v", i, got[i], want[i])
		}
	}
	if !out.RequiresGrad() {
		t.Fatal("çıktı gradyan istemiyor")
	}
	w := tensor.Randn(s.rng(100), out.Shape()...)
	out.BackwardWith(w)
	wd := f64s([]*tensor.Tensor{w})[0]
	for k := range xs {
		an := xs[k].Grad().Data()
		for j := range in[k] {
			orig := in[k][j]
			h := 1e-6 * math.Max(1, math.Abs(orig))
			in[k][j] = orig + h
			lp := dot64(s.ref(in), wd)
			in[k][j] = orig - h
			lm := dot64(s.ref(in), wd)
			in[k][j] = orig
			num := (lp - lm) / (2 * h)
			if math.IsNaN(float64(an[j])) || math.Abs(float64(an[j])-num) > compositeF64Tol*(1+math.Abs(num)) {
				t.Fatalf("girdi %d eleman %d: analitik %.7g, float64 sayısal %.7g", k, j, an[j], num)
			}
		}
	}
}

// compositeRun işlemi koşar (bf16: bf16 girdiler ve karışık hassasiyet bloğu), çıktıyı (float32'ye çevrilmiş) ve girdi gradyanlarını verir.
func compositeRun(t *testing.T, s compositeSpec, bf16 bool, rounded [][]float32) ([]float32, [][]float32) {
	xs := make([]*tensor.Tensor, len(rounded))
	for i := range xs {
		if bf16 {
			// Karışık hassasiyetin gerçek durumu: etkinlikler (bir önceki matmul çıktısı) bf16'dır.
			xs[i] = tensor.NewOf(tensor.BFloat16, rounded[i], s.shapes[i]...).SetRequiresGrad(true)
		} else {
			xs[i] = tensor.New(rounded[i], s.shapes[i]...).SetRequiresGrad(true)
		}
	}
	out, err := s.run(compositeVM(bf16), xs)
	if err != nil {
		t.Fatal(err)
	}
	res := out.Cast(tensor.Float32)
	res.BackwardWith(tensor.Randn(rand.New(rand.NewSource(99)), res.Shape()...))
	grads := make([][]float32, len(xs))
	for i, x := range xs {
		grads[i] = make([]float32, x.Numel())
		if g := x.Grad(); g != nil {
			copy(grads[i], g.Data())
		}
	}
	return res.Data(), grads
}

// compositeDeviation karışık bf16 koşusunun, bf16'ya yuvarlanmış aynı girdilerle float32
// koşudan en büyük sapmasını (tensörün en büyük mutlak değerine göre) ve yerini verir.
func compositeDeviation(t *testing.T, s compositeSpec, k int) (worst float64, where string) {
	xs := s.inputs(k)
	rounded := make([][]float32, len(xs))
	for i, x := range xs {
		rounded[i] = x.Cast(tensor.BFloat16).Cast(tensor.Float32).Data()
	}
	out32, g32 := compositeRun(t, s, false, rounded)
	out16, g16 := compositeRun(t, s, true, rounded)
	cmp := func(ad string, got, want []float32) {
		var scale float64
		for _, v := range want {
			scale = math.Max(scale, math.Abs(float64(v)))
		}
		scale = math.Max(scale, 1e-6)
		for i := range want {
			d := math.Abs(float64(got[i])-float64(want[i])) / scale
			if math.IsNaN(float64(got[i])) != math.IsNaN(float64(want[i])) {
				d = math.Inf(1)
			}
			if d > worst {
				worst, where = d, fmt.Sprintf("%s [%d]: bf16 %v, float32 %v (ölçek %g)", ad, i, got[i], want[i], scale)
			}
		}
	}
	cmp("çıktı", out16, out32)
	for j := range g32 {
		cmp(fmt.Sprintf("girdi %d gradyanı", j), g16[j], g32[j])
	}
	return worst, where
}

func checkCompositeBF16(t *testing.T, s compositeSpec) {
	for k := 0; k < compositeSeeds; k++ {
		if worst, where := compositeDeviation(t, s, k); worst > s.bf16Tol {
			t.Fatalf("tohum %d: %s, sapma %.3g > tolerans %g", k, where, worst, s.bf16Tol)
		}
	}
}

func TestCompositeGrad(t *testing.T) {
	for _, s := range compositeSpecs() {
		s := s
		t.Run(s.name+"/float64", func(t *testing.T) { checkCompositeFloat64(t, s) })
		t.Run(s.name+"/bf16", func(t *testing.T) { checkCompositeBF16(t, s) })
	}
}

// TestMeasureCompositeBF16 bileşiklerin bf16 sapmasını ölçer ve tabloyu basar (TR_MEASURE_BF16=1).
// Kural tensör paketindekiyle aynı: tolerans = en büyük sapmanın 2 katı, 1e-3'e yukarı yuvarlanır,
// en az 4e-3; sonucu compositeSpecs'teki bf16Tol alanlarına elle yazın.
func TestMeasureCompositeBF16(t *testing.T) {
	if os.Getenv("TR_MEASURE_BF16") == "" {
		t.Skip("ölçüm için TR_MEASURE_BF16=1 verin")
	}
	line := "\n| kayıt | en büyük sapma | tolerans |\n| :-- | --: | --: |\n"
	for _, s := range compositeSpecs() {
		var mx float64
		for k := 0; k < compositeMeasured; k++ {
			if w, _ := compositeDeviation(t, s, k); w > mx {
				mx = w
			}
		}
		line += fmt.Sprintf("| %s | %.4f | %g |\n", s.name, mx, math.Max(math.Ceil(2*mx*1000)/1000, compositeFloor))
	}
	t.Log(line)
}

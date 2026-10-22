package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Eleman bazlı işlemler: tekli (ileri ve türev), ikili (yayınlama, adımlı görünümler),
// skaler işlemler; kenar boyutlar (boş, tek eleman, asal uzunluklar).

var edgeSizes = []int{0, 1, 7, 257, 1031}

func TestUnaryOps(t *testing.T) {
	ops := []struct {
		ad string
		f  func(x *tensor.Tensor) *tensor.Tensor
		// positive: girdi |x| + 0,1 alınır (log, karekök, üs)
		positive bool
	}{
		{"üstel", (*tensor.Tensor).Exp, false},
		{"log", (*tensor.Tensor).Log, true},
		{"karekök", (*tensor.Tensor).Sqrt, true},
		{"tanh", (*tensor.Tensor).Tanh, false},
		{"relu", (*tensor.Tensor).Relu, false},
		{"gelu", (*tensor.Tensor).Gelu, false},
		{"sigmoid", (*tensor.Tensor).Sigmoid, false},
		{"sinüs", (*tensor.Tensor).Sin, false},
		{"kosinüs", (*tensor.Tensor).Cos, false},
		{"kare", (*tensor.Tensor).Square, false},
		{"üs1", func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(1) }, false},
		{"üs0", func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(0) }, false},
		{"üs1.5", func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(1.5) }, true},
		{"üs-2", func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(-2) }, true},
		{"üs3 negatif taban", func(x *tensor.Tensor) *tensor.Tensor { return x.Pow(3) }, false},
		{"+s", func(x *tensor.Tensor) *tensor.Tensor { return x.AddScalar(1.5) }, false},
		{"*s", func(x *tensor.Tensor) *tensor.Tensor { return x.MulScalar(-2.5) }, false},
		{"s-", func(x *tensor.Tensor) *tensor.Tensor { return x.RSubScalar(3) }, false},
		{"s/", func(x *tensor.Tensor) *tensor.Tensor { return x.RDivScalar(2) }, true},
		{"/s", func(x *tensor.Tensor) *tensor.Tensor { return x.DivScalar(3) }, false},
	}
	rng := rand.New(rand.NewSource(1))
	for _, is := range ops {
		for _, n := range edgeSizes {
			data := random(rng, n, 2)
			if is.positive {
				for i, v := range data {
					if v < 0 {
						v = -v
					}
					data[i] = v + 0.1
				}
			}
			compare(t, fmt.Sprintf("%s n=%d", is.ad, n), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(data, true, n)[c]
				return is.f(x), []*tensor.Tensor{x}
			})
		}
	}
}

// TestUnarySpecialValues büyük, küçük ve özel değerlerde (sonsuz, NaN, sıfır) tekli
// işlemlerin CPU ile uyuştuğunu denetler.
func TestUnarySpecialValues(t *testing.T) {
	inf := float32(math.Inf(1))
	data := []float32{0, float32(math.Copysign(0, -1)), 1e-30, -1e-30, 1e-4, -1e-4, 0.29, 0.31, 30, -30, 88, -88, 100, -100, 1e20, -1e20, inf, -inf}
	for _, is := range []struct {
		ad string
		f  func(x *tensor.Tensor) *tensor.Tensor
	}{
		{"üstel", (*tensor.Tensor).Exp}, {"tanh", (*tensor.Tensor).Tanh}, {"sigmoid", (*tensor.Tensor).Sigmoid},
		{"relu", (*tensor.Tensor).Relu}, {"gelu", (*tensor.Tensor).Gelu},
	} {
		compare(t, is.ad, 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			return is.f(twin(data, false, len(data))[c]), nil
		})
	}
}

func TestBinaryBroadcast(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	shapes := [][2][]int{
		{{5, 7}, {5, 7}}, {{5, 7}, {7}}, {{5, 1}, {1, 7}}, {{3, 1, 4}, {2, 1}}, {{1}, {6, 3}},
		{{2, 3, 4, 5}, {3, 1, 5}}, {{0, 3}, {3}}, {{1031}, {1}}, {{2, 1, 3, 1, 2}, {1, 4, 1, 5, 1}},
	}
	ops := []struct {
		ad string
		f  func(a, b *tensor.Tensor) *tensor.Tensor
	}{{"+", tensor.Add}, {"-", tensor.Sub}, {"*", tensor.Mul}, {"/", tensor.Div}}
	for _, sh := range shapes {
		for _, is := range ops {
			av := random(rng, numel(sh[0]), 1)
			bv := random(rng, numel(sh[1]), 1)
			for i := range bv {
				bv[i] += 3 // bölmede sıfırdan uzak
			}
			compare(t, fmt.Sprintf("%v %s %v", sh[0], is.ad, sh[1]), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				a, b := twin(av, true, sh[0]...)[c], twin(bv, true, sh[1]...)[c]
				return is.f(a, b), []*tensor.Tensor{a, b}
			})
		}
	}
}

// TestStridedViews devrik, eksen sıralama, dilim ve adımlı dilim görünümleri
// üzerinde işlemleri ve gradyanlarını (dağıtma) denetler.
func TestStridedViews(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	v := random(rng, 2*3*4*5, 1)
	cases := []struct {
		ad string
		f  func(x *tensor.Tensor) *tensor.Tensor
	}{
		{"devrik+üstel", func(x *tensor.Tensor) *tensor.Tensor { return x.Transpose(1, 3).Exp() }},
		{"eksen sırası*2", func(x *tensor.Tensor) *tensor.Tensor { return x.Permute(3, 0, 2, 1).MulScalar(2) }},
		{"dilim+devrik", func(x *tensor.Tensor) *tensor.Tensor {
			return tensor.Add(x.Slice(2, 1, 3), x.Slice(2, 0, 2).Transpose(0, 1).Transpose(0, 1))
		}},
		{"adımlı dilim", func(x *tensor.Tensor) *tensor.Tensor { return x.SliceStep(3, 0, 5, 2).Tanh() }},
		{"bitişikleştir", func(x *tensor.Tensor) *tensor.Tensor { return x.Permute(2, 1, 0, 3).Contiguous().Reshape(4, -1) }},
		{"kendine yayın", func(x *tensor.Tensor) *tensor.Tensor { return tensor.Mul(x, x.Slice(1, 0, 1)) }},
		{"bitiştir", func(x *tensor.Tensor) *tensor.Tensor { return tensor.Concat([]*tensor.Tensor{x, x.Exp()}, 2) }},
		{"bölüştür", func(x *tensor.Tensor) *tensor.Tensor { return x.Split(2, 2)[1].Sigmoid() }},
	}
	for _, d := range cases {
		compare(t, d.ad, 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, 2, 3, 4, 5)[c]
			return d.f(x), []*tensor.Tensor{x}
		})
	}
}

// TestCasts bfloat16 ve float16 depolamada dönüşümleri ve eleman işlemlerini
// denetler: dönüşüm bit düzeyinde CPU ile aynı olmalıdır.
func TestCasts(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	v := random(rng, 513, 50)
	v = append(v, 0, 1e-8, -1e-8, 65504, 65520, 1e30, 3.3e-39, 6e-8)
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		c := tensor.NewOn(tensor.CPU, v, len(v)).Cast(dt)
		g := tensor.NewOn(gpuDevice, v, len(v)).Cast(dt)
		checkDiff(t, dt.String()+" dönüşüm", g.Data(), c.Data(), 0)
		compare(t, dt.String()+" işlemler", 1e-2, func(k int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v[:513], false, 513)[k].Cast(dt)
			return tensor.Add(x.Exp().MulScalar(0.001), x).Cast(tensor.Float32), nil
		})
	}
}

// TestConcatKeepsSignedZero GPU'da da birleştirmenin -0'ı koruduğunu (indirgeme birikimi -0'dan
// başlar) ve 16 bitlik çıktıda da aynı bitleri verdiğini denetler.
func TestConcatKeepsSignedZero(t *testing.T) {
	neg := float32(math.Copysign(0, -1))
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
		a := tensor.NewOn(gpuDevice, []float32{neg, 1}, 2).Cast(dt)
		b := tensor.NewOn(gpuDevice, []float32{neg, 0}, 2).Cast(dt)
		d := tensor.Concat([]*tensor.Tensor{a, b}, 0).Data()
		for i, want := range []bool{true, false, true, false} {
			if math.Signbit(float64(d[i])) != want {
				t.Fatalf("%s: %v elemanlarının işaretleri yanlış (%d. eleman)", dt, d, i)
			}
		}
	}
}

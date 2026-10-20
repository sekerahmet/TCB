package gpu

import (
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Gradyan kapısının GPU ayağı: türevli işlemlerden başka testlerde GPU-CPU gradyan
// karşılaştırması olmayanlar (tür dönüşümü, cihaz aktarımı, kopya, dış işlem). Öteki işlemlerin
// karşılaştırması kendi dosyalarındadır (backend_elementwise_test.go, backend_wide_test.go,
// backend_reduce_test.go, behavior_nn_test.go, attention_test.go).

// TestGradParityCast float32 yaprağın 16 bitlik dönüşümünden geçen gradyan CPU ile uyuşur.
func TestGradParityCast(t *testing.T) {
	v := random(rand.New(rand.NewSource(11)), 3*7, 1)
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		compare(t, "dönüşüm gradyanı "+dt.String(), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, 3, 7)[c]
			return x.Cast(dt).Tanh().Cast(tensor.Float32), []*tensor.Tensor{x}
		})
	}
}

// TestGradParityTo cihaz aktarımının gradyanı kaynak cihaza geri döner: CPU yaprak GPU'ya,
// GPU yaprak CPU'ya aktarılıp geri alınır.
func TestGradParityTo(t *testing.T) {
	v := random(rand.New(rand.NewSource(12)), 4*5, 1)
	compare(t, "cpu→gpu aktarım", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := tensor.NewOn(tensor.CPU, v, 4, 5).SetRequiresGrad(true)
		if c == 1 {
			return x.To(gpuDevice).Exp(), []*tensor.Tensor{x}
		}
		return x.Exp(), []*tensor.Tensor{x}
	})
	compare(t, "gpu→cpu→gpu aktarım", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := twin(v, true, 4, 5)[c]
		if c == 1 {
			return x.To(tensor.CPU).Exp().To(gpuDevice), []*tensor.Tensor{x}
		}
		return x.Exp(), []*tensor.Tensor{x}
	})
}

// TestGradParityCloneAndCustom kopya ve kullanıcı tanımlı (dış düğümlü) işlemin gradyanı CPU ile uyuşur.
func TestGradParityCloneAndCustom(t *testing.T) {
	v := random(rand.New(rand.NewSource(13)), 3*4, 1)
	compare(t, "kopya", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := twin(v, true, 3, 4)[c]
		return x.Clone().Square(), []*tensor.Tensor{x}
	})
	compare(t, "dış işlem", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x := twin(v, true, 3, 4)[c]
		var y *tensor.Tensor
		tensor.NoGrad(func() { y = x.Mul(x) })
		out := tensor.CustomOp("kare", []*tensor.Tensor{x}, []*tensor.Tensor{y}, []*tensor.Tensor{x},
			func(_ any, g []*tensor.Tensor) []*tensor.Tensor { return []*tensor.Tensor{g[0].Mul(x).MulScalar(2)} })
		return out[0], []*tensor.Tensor{x}
	})
}

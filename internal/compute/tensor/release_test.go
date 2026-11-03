package tensor

import (
	"math/rand"
	"testing"
)

// releasedOps dil katmanının Released içinde çağırdığı tek çekirdekli işlemlerdir (ml: unit,
// gelu, + - * / işleçleri, toplam, ortalama, max). Yeni bir sarmalama buraya da yazılır.
var releasedOps = map[string]func(a, b *Tensor) *Tensor{
	"Exp": func(a, _ *Tensor) *Tensor { return a.Exp() }, "Log": func(a, _ *Tensor) *Tensor { return a.Log() },
	"Sqrt": func(a, _ *Tensor) *Tensor { return a.Sqrt() }, "Tanh": func(a, _ *Tensor) *Tensor { return a.Tanh() },
	"Relu": func(a, _ *Tensor) *Tensor { return a.Relu() }, "Sigmoid": func(a, _ *Tensor) *Tensor { return a.Sigmoid() },
	"Gelu": func(a, _ *Tensor) *Tensor { return a.Gelu() }, "GeluExact": func(a, _ *Tensor) *Tensor { return a.GeluExact() },
	"Silu": func(a, _ *Tensor) *Tensor { return a.Silu() }, "Tan": func(a, _ *Tensor) *Tensor { return a.Tan() },
	"Asin": func(a, _ *Tensor) *Tensor { return a.Asin() }, "Acos": func(a, _ *Tensor) *Tensor { return a.Acos() },
	"Atan": func(a, _ *Tensor) *Tensor { return a.Atan() }, "Sinh": func(a, _ *Tensor) *Tensor { return a.Sinh() },
	"Cosh": func(a, _ *Tensor) *Tensor { return a.Cosh() }, "Log1p": func(a, _ *Tensor) *Tensor { return a.Log1p() },
	"Log2": func(a, _ *Tensor) *Tensor { return a.Log2() }, "Log10": func(a, _ *Tensor) *Tensor { return a.Log10() },
	"Expm1": func(a, _ *Tensor) *Tensor { return a.Expm1() }, "Sin": func(a, _ *Tensor) *Tensor { return a.Sin() },
	"Cos":       func(a, _ *Tensor) *Tensor { return a.Cos() },
	"AddScalar": func(a, _ *Tensor) *Tensor { return a.AddScalar(2) }, "SubScalar": func(a, _ *Tensor) *Tensor { return a.SubScalar(2) },
	"MulScalar": func(a, _ *Tensor) *Tensor { return a.MulScalar(2) }, "DivScalar": func(a, _ *Tensor) *Tensor { return a.DivScalar(2) },
	"Add": Add, "Sub": Sub, "Mul": Mul, "Div": Div,
	"Sum": func(a, _ *Tensor) *Tensor { return a.Sum(0, false) }, "Mean": func(a, _ *Tensor) *Tensor { return a.Mean(0, true) },
	"SumAll": func(a, _ *Tensor) *Tensor { return a.SumAll() }, "MeanAll": func(a, _ *Tensor) *Tensor { return a.MeanAll() },
	"Max": func(a, _ *Tensor) *Tensor { return a.Max(0, false) },
}

// countingRunner çekirdeği kilitsiz yol gibi çalıştırır (izinle) ve çağrıları sayar.
func countingRunner(t *testing.T, calls *int) UnlockRunner {
	return func(prepare func() (*Tensor, *Kernel)) *Tensor {
		*calls++
		r, k := prepare()
		if !k.TryAcquire() {
			t.Fatal("çekirdeğin izni alınamadı")
		}
		k.Run()
		return r
	}
}

// Kural (10.2, S-kilit): dil katmanının Released'la sarmaladığı her işlem tek çekirdektir; kapsam
// yalnız onu bırakır ve sonuç kilit altındakiyle bit bit aynıdır. İki çekirdek arasında kilit
// bırakılsaydı başka görev girdiyi değiştirebilir, sonuç iki girdinin karışığı olurdu.
func TestReleasedOpsAreOneKernel(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	n := unlockedElemWork
	a, b := Randn(rng, n).MulScalar(0.5), Randn(rng, n).AddScalar(3)
	for name, op := range releasedOps {
		want := op(a, b).Data()
		calls, kernels := 0, 0
		got := Released(countingRunner(t, &calls), func() *Tensor {
			s := activeRelease
			r := op(a, b)
			kernels = s.kernels
			return r
		})
		if kernels != 1 || calls != 1 {
			t.Errorf("%s: kapsamda %d çekirdek, %d kilitsiz çağrı; ikisi de 1 olmalı", name, kernels, calls)
			continue
		}
		bitEqual32(t, name, got.Data(), want)
	}
	if activeRelease != nil {
		t.Fatal("kapsam Released'dan sonra kapanmadı")
	}
}

// Kural (10.2, S-kilit): küçük işlem ve kapsamdaki ikinci çekirdek kilit altında çalışır.
func TestReleasedRunsSmallAndSecondKernelsLocked(t *testing.T) {
	calls := 0
	Released(countingRunner(t, &calls), func() *Tensor { return Full(1, 4, 4).Exp() })
	if calls != 0 {
		t.Fatalf("16 öğelik işlem kilit bırakılarak çalıştı (%d çağrı)", calls)
	}
	big := Full(1, unlockedElemWork)
	Released(countingRunner(t, &calls), func() *Tensor { return big.Exp().Log() })
	if calls != 1 {
		t.Fatalf("iki çekirdekli işlemde %d kilitsiz çağrı; yalnız ilki bırakılmalı", calls)
	}
}

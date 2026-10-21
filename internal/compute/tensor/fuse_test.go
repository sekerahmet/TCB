package tensor

import (
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kaynaştırma (fuse.go): ileri yön kaynaşmamış yolla bit düzeyinde aynıdır; geri yön
// float32'de hesaplanır ve kaynaşmamış otomatik türevle (float32'de) aynı sonucu verir.

// rope döner konum uygulaması (lib/katman.tr'deki döner_konum_uygula): x [..., T, D],
// kos/sin [T', D/2] (ilk T satırı kullanılır).
func rope(args []*Tensor) []*Tensor {
	x, c, s := args[0], args[1], args[2]
	T := x.Shape()[len(x.Shape())-2]
	ab := x.Split(2, -1)
	a, b := ab[0], ab[1]
	c, s = c.Slice(0, 0, T), s.Slice(0, 0, T)
	return []*Tensor{Concat([]*Tensor{a.Mul(c).Sub(b.Mul(s)), a.Mul(s).Add(b.Mul(c))}, -1)}
}

// swiglu birleşik [G | U] matrisinin çıktısından silu(G) · U.
func swiglu(args []*Tensor) []*Tensor {
	gu := args[0].Split(2, -1)
	return []*Tensor{gu[0].Silu().Mul(gu[1])}
}

// ple PLE bloğunun eleman bazlı zinciri: (bağlam + √P·tablo) / √2 ve gelu(kapı) · e.
func ple(args []*Tensor) []*Tensor {
	e := args[0].Add(args[1].MulScalar(8)).DivScalar(float32(math.Sqrt2))
	return []*Tensor{args[2].GeluExact().Mul(e), e}
}

func fuseInfos(ts []*Tensor) []FuseInputInfo {
	out := make([]FuseInputInfo, len(ts))
	for i, t := range ts {
		out[i] = FuseInputInfo{Shape: t.Shape(), DType: t.DType(), RequiresGrad: t.RequiresGrad()}
	}
	return out
}

// runFusedFn fn'i kaynaştırıp çalıştırır.
func runFusedFn(fn func([]*Tensor) []*Tensor, ins []*Tensor) []*Tensor {
	p := TraceFused(ins[0].be, fuseInfos(ins), fn)
	return p.Run("kaynaştır", ins)
}

func sameBits(t *testing.T, what string, a, b *Tensor) {
	t.Helper()
	if !sameShape(a.Shape(), b.Shape()) || a.DType() != b.DType() {
		t.Fatalf("%s: şekil/tür %v %s, beklenen %v %s", what, a.Shape(), a.DType(), b.Shape(), b.DType())
	}
	x, y := a.Data(), b.Data()
	for i := range x {
		if math.Float32bits(x[i]) != math.Float32bits(y[i]) {
			t.Fatalf("%s: %d. eleman %v, kaynaşmamış %v", what, i, x[i], y[i])
		}
	}
}

func closeTo(t *testing.T, what string, a, b *Tensor, tol float64) {
	t.Helper()
	x, y := a.Data(), b.Data()
	for i := range x {
		if d := math.Abs(float64(x[i] - y[i])); d > tol*(1+math.Abs(float64(y[i]))) {
			t.Fatalf("%s: %d. eleman %v, kaynaşmamış %v", what, i, x[i], y[i])
		}
	}
}

func fuseInputs(dt DType, grad bool) []*Tensor {
	rng := rand.New(rand.NewSource(3))
	// x adımlı bir görünümdür (dikkat başlarına ayırmadaki gibi eksen sırası değişmiş).
	x := Randn(rng, 2, 5, 3, 8).Permute(0, 2, 1, 3).Cast(dt)
	c := Randn(rng, 7, 4).Cast(dt)
	s := Randn(rng, 7, 4).Cast(dt)
	if grad {
		x = x.Detach().SetRequiresGrad(true)
	}
	return []*Tensor{x, c, s}
}

func TestFusedForwardMatchesUnfusedBits(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		ins := fuseInputs(dt, false)
		sameBits(t, "rope "+dt.String(), runFusedFn(rope, ins)[0], rope(ins)[0])
		rng := rand.New(rand.NewSource(4))
		gu := Randn(rng, 3, 4, 12).Cast(dt)
		sameBits(t, "swiglu "+dt.String(), runFusedFn(swiglu, []*Tensor{gu})[0], swiglu([]*Tensor{gu})[0])
		pi := []*Tensor{Randn(rng, 3, 6).Cast(dt), Randn(rng, 3, 6), Randn(rng, 3, 6).Cast(dt)}
		f, u := runFusedFn(ple, pi), ple(pi)
		sameBits(t, "ple "+dt.String(), f[0], u[0])
		sameBits(t, "ple e "+dt.String(), f[1], u[1])
	}
}

// Gradyanlar: kaynaşmış geri yön (float32) kaynaşmamış otomatik türevle aynı sonucu verir.
func TestFusedGradientsMatchUnfused(t *testing.T) {
	check := func(name string, fn func([]*Tensor) []*Tensor, mk func() []*Tensor) {
		t.Helper()
		a, b := mk(), mk()
		fa := runFusedFn(fn, a)
		ua := fn(b)
		lossA, lossB := fa[0].Mul(fa[0]).SumAll(), ua[0].Mul(ua[0]).SumAll()
		for i := 1; i < len(fa); i++ {
			lossA, lossB = lossA.Add(fa[i].SumAll()), lossB.Add(ua[i].SumAll())
		}
		lossA.Backward()
		lossB.Backward()
		for i := range a {
			if a[i].RequiresGrad() {
				closeTo(t, name+" gradyanı", a[i].Grad(), b[i].Grad(), 1e-5)
			}
		}
	}
	rng := rand.New(rand.NewSource(5))
	check("rope", rope, func() []*Tensor { return fuseInputs(Float32, true) })
	gu := Randn(rng, 3, 4, 12)
	check("swiglu", swiglu, func() []*Tensor { return []*Tensor{gu.Detach().SetRequiresGrad(true)} })
	p0, p1, p2 := Randn(rng, 3, 6), Randn(rng, 3, 6), Randn(rng, 3, 6)
	check("ple", ple, func() []*Tensor {
		return []*Tensor{p0.Detach().SetRequiresGrad(true), p1.Detach().SetRequiresGrad(true), p2.Detach().SetRequiresGrad(true)}
	})
}

func TestFusedErrors(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	x := Randn(rng, 4, 6)
	expectCode := func(name string, code messages.Code, fn func([]*Tensor) []*Tensor, ins ...*Tensor) {
		t.Helper()
		defer func() {
			if !messages.Has(asError(recover()), code) {
				t.Fatalf("%s: %s bekleniyordu", name, code)
			}
		}()
		runFusedFn(fn, ins)
	}
	expectCode("indirgeme", messages.H3107, func(a []*Tensor) []*Tensor { return []*Tensor{a[0].SumAll()} }, x)
	outside := Randn(rng, 4, 6)
	expectCode("dış tensör", messages.H3108, func(a []*Tensor) []*Tensor { return []*Tensor{a[0].Add(outside)} }, x)
	expectCode("şekillendirme", messages.H3110, func(a []*Tensor) []*Tensor { return []*Tensor{a[0].Reshape(6, 4)} }, x)
	b := Randn(rng, 6).SetRequiresGrad(true)
	expectCode("yayınlanan türev", messages.H3109, func(a []*Tensor) []*Tensor { return []*Tensor{a[0].Mul(a[1])} }, x, b)
}

// asError kurtarılan paniği hataya çevirir.
func asError(r any) error {
	if e, ok := r.(error); ok {
		return e
	}
	return nil
}

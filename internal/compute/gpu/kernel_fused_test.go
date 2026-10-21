package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kaynaşmış çekirdekler (kernel_fused.go): ileri yön kaynaşmamış GPU yoluyla bit düzeyinde
// aynıdır; çıktı ve gradyanlar CPU'nun kaynaşmış yürütücüsüyle karşılaştırılır.

// fusedRope lib/katman.tr'deki döner_konum_uygula: x [..., T, D], kos/sin [T', D/2].
func fusedRope(args []*tensor.Tensor) []*tensor.Tensor {
	x, c, s := args[0], args[1], args[2]
	T := x.Shape()[len(x.Shape())-2]
	ab := x.Split(2, -1)
	c, s = c.Slice(0, 0, T), s.Slice(0, 0, T)
	return []*tensor.Tensor{tensor.Concat([]*tensor.Tensor{ab[0].Mul(c).Sub(ab[1].Mul(s)), ab[0].Mul(s).Add(ab[1].Mul(c))}, -1)}
}

// fusedGated silu(G) · U ve gelu(G) · U / √2 (SwiGLU ve PLE kapısı).
func fusedGated(args []*tensor.Tensor) []*tensor.Tensor {
	gu := args[0].Split(2, -1)
	return []*tensor.Tensor{gu[0].Silu().Mul(gu[1]), gu[0].GeluExact().Mul(gu[1]).DivScalar(float32(math.Sqrt2))}
}

func runFused(fn func([]*tensor.Tensor) []*tensor.Tensor, ins []*tensor.Tensor) []*tensor.Tensor {
	info := make([]tensor.FuseInputInfo, len(ins))
	for i, x := range ins {
		info[i] = tensor.FuseInputInfo{Shape: x.Shape(), DType: x.DType(), RequiresGrad: x.RequiresGrad()}
	}
	return tensor.TraceFused(tensor.BackendFor(ins[0].Device()), info, fn).Run("kaynaştır", ins)
}

func TestFusedMatchesUnfusedBits(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	xv, cv, sv, gv := random(rng, 2*5*3*8, 1), random(rng, 7*4, 1), random(rng, 7*4, 1), random(rng, 3*4*12, 1)
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
		// x adımlı bir görünümdür (dikkat başlarındaki gibi eksen sırası değişmiş).
		x := tensor.NewOn(gpuDevice, xv, 2, 5, 3, 8).Permute(0, 2, 1, 3).Cast(dt)
		c, s := tensor.NewOn(gpuDevice, cv, 7, 4).Cast(dt), tensor.NewOn(gpuDevice, sv, 7, 4).Cast(dt)
		ins := []*tensor.Tensor{x, c, s}
		bitsEqual(t, "rope "+dt.String(), runFused(fusedRope, ins)[0], fusedRope(ins)[0])
		g := []*tensor.Tensor{tensor.NewOn(gpuDevice, gv, 3, 4, 12).Cast(dt)}
		f, u := runFused(fusedGated, g), fusedGated(g)
		bitsEqual(t, "silu·u "+dt.String(), f[0], u[0])
		bitsEqual(t, "gelu·u "+dt.String(), f[1], u[1])
	}
}

func bitsEqual(t *testing.T, what string, a, b *tensor.Tensor) {
	t.Helper()
	x, y := a.Data(), b.Data()
	if len(x) != len(y) || a.DType() != b.DType() {
		t.Fatalf("%s: boy/tür farklı", what)
	}
	for i := range x {
		if math.Float32bits(x[i]) != math.Float32bits(y[i]) {
			t.Fatalf("%s: %d. eleman kaynaşmış %v, kaynaşmamış %v", what, i, x[i], y[i])
		}
	}
}

// Çıktılar ve geri yön (gradyanlar) GPU'da CPU'nun kaynaşmış yürütücüsüyle aynıdır.
func TestFusedMatchesCPU(t *testing.T) {
	rng := rand.New(rand.NewSource(22))
	xv, cv, sv, gv := random(rng, 2*5*3*8, 1), random(rng, 7*4, 1), random(rng, 7*4, 1), random(rng, 3*4*12, 1)
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
		tol := 2e-5
		if dt != tensor.Float32 {
			tol = 2e-2
		}
		compare(t, fmt.Sprintf("kaynaşmış rope %s", dt), tol, func(d int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(xv, true, 2, 5, 3, 8)[d]
			c, s := twin(cv, false, 7, 4)[d].Cast(dt), twin(sv, false, 7, 4)[d].Cast(dt)
			y := runFused(fusedRope, []*tensor.Tensor{x.Permute(0, 2, 1, 3).Cast(dt), c, s})[0]
			return y.Mul(y).SumAll(), []*tensor.Tensor{x}
		})
		compare(t, fmt.Sprintf("kaynaşmış kapı %s", dt), tol, func(d int) (*tensor.Tensor, []*tensor.Tensor) {
			g := twin(gv, true, 3, 4, 12)[d]
			o := runFused(fusedGated, []*tensor.Tensor{g.Cast(dt)})
			return o[0].Mul(o[0]).SumAll().Add(o[1].SumAll()), []*tensor.Tensor{g}
		})
	}
}

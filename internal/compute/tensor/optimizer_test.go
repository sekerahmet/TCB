package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bulgu 15a: AdamW'nin sapma düzeltmesi parametre başına adım sayısını kullanır: ilk
// adımlarda gradyanı olmayan bir parametrenin ilk güncellemesi, yeni bir iyileştiricinin
// ilk güncellemesiyle aynıdır.
func TestAdamWStepPerParam(t *testing.T) {
	a := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	b := New([]float32{3, 4}, 2).SetRequiresGrad(true)
	op := NewAdamW([]*Tensor{a, b}, 0.1, 0)
	for i := 0; i < 3; i++ {
		op.ZeroGrad()
		a.Square().SumAll().Backward() // b'nin gradyanı yok
		op.Step()
	}
	op.ZeroGrad()
	b.Square().SumAll().Backward()
	op.Step()
	single := New([]float32{3, 4}, 2).SetRequiresGrad(true)
	op2 := NewAdamW([]*Tensor{single}, 0.1, 0)
	single.Square().SumAll().Backward()
	op2.Step()
	near(t, "gecikmeli ilk adım", b.Data(), single.Data(), 0)
	if op.StepCount() != 4 {
		t.Fatalf("StepCount %d", op.StepCount())
	}
}

// Bulgu 15b: iyileştirici durumu (momentler, adım sayıları, 16 bitlik parametrelerin
// float32 ana kopyası, SGD hızı) dışa alınıp yeni bir iyileştiriciye yüklenebilir;
// sürdürülen eğitim kesintisiz eğitimle bit düzeyinde aynıdır.
func TestOptimizerStateResume(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	w0 := Randn(rng, 4, 3)
	b0 := Randn(rng, 3).Cast(BFloat16)
	x := Randn(rng, 5, 4)
	step := func(params []*Tensor, op Optimizer) {
		op.ZeroGrad()
		Add(x.MatMul(params[0]), params[1]).Square().MeanAll().Backward()
		op.Step()
	}
	for _, dtyp := range []string{"AdamW", "SGD"} {
		freshT := func(params []*Tensor) interface {
			Optimizer
			State() map[string]*Tensor
			LoadState(map[string]*Tensor)
		} {
			if dtyp == "AdamW" {
				return NewAdamW(params, 0.01, 0.1)
			}
			return NewSGD(params, 0.05, 0.9)
		}
		dup := func() []*Tensor {
			return []*Tensor{w0.Clone().Detach().SetRequiresGrad(true), b0.Clone().Detach().SetRequiresGrad(true)}
		}
		// Kesintisiz: 8 adım.
		pa := dup()
		oa := freshT(pa)
		for i := 0; i < 8; i++ {
			step(pa, oa)
		}
		// 5 adım, durumu al, yeni iyileştiriciyle 3 adım daha.
		pb := dup()
		ob := freshT(pb)
		for i := 0; i < 5; i++ {
			step(pb, ob)
		}
		state := ob.State()
		pc := []*Tensor{pb[0].Clone().Detach().SetRequiresGrad(true), pb[1].Clone().Detach().SetRequiresGrad(true)}
		oc := freshT(pc)
		oc.LoadState(state)
		for i := 0; i < 3; i++ {
			step(pc, oc)
		}
		for k := range pa {
			near(t, fmt.Sprintf("%s param %d", dtyp, k), pc[k].Data(), pa[k].Data(), 0)
		}
		// Uyumsuz durum açık hata verir.
		panicCode(t, dtyp+" uyumsuz", messages.H3059, func() {
			freshT([]*Tensor{Zeros(2).SetRequiresGrad(true)}).LoadState(state)
		})
	}
}

// Bozuk bir durumdaki adım sayacı (negatif, kesirli, NaN) ya da momentli ama sayaçsız
// parametre yüklenmez: sapma düzeltmesi sıfıra bölünüp ağırlıklar NaN olurdu.
func TestAdamWLoadStateRejectsBadCounters(t *testing.T) {
	mk := func() (*AdamW, map[string]*Tensor) {
		w := New([]float32{1, 2}, 2).SetRequiresGrad(true)
		o := NewAdamW([]*Tensor{w}, 0.1, 0.0)
		w.MulScalar(3).SumAll().Backward()
		o.Step()
		return NewAdamW([]*Tensor{New([]float32{1, 2}, 2).SetRequiresGrad(true)}, 0.1, 0.0), o.State()
	}
	nan := float32(math.NaN())
	for _, bad := range [][]float32{{0, -1}, {-1, 0}, {nan, nan}, {0, 0.5}, {1 << 24, 0}} {
		fresh, st := mk()
		st["step.0"] = New(bad, 2)
		panicCode(t, fmt.Sprintf("sayaç %v", bad), messages.H3057, func() { fresh.LoadState(st) })
	}
	fresh, st := mk()
	delete(st, "step.0")
	panicCode(t, "sayaçsız moment", messages.H3062, func() { fresh.LoadState(st) })
	fresh, st = mk()
	fresh.LoadState(st) // sağlam durum yüklenir
}

// Listede iki kez geçen (bağlı) parametrenin gradyanı norma bir kez girer ve bir kez
// ölçeklenir: |g| = 5, en çok 1 → norm 5, kırpma sonrası |g| 1.
func TestClipGradNormTiedParameter(t *testing.T) {
	for _, tensorForm := range []bool{false, true} {
		w := New([]float32{0, 0}, 2).SetRequiresGrad(true)
		Mul(w, New([]float32{3, 4}, 2)).SumAll().Backward()
		var norm float64
		if tensorForm {
			norm = float64(ClipGradNormTensor([]*Tensor{w, w}, 1).Item())
		} else {
			norm = ClipGradNorm([]*Tensor{w, w}, 1)
		}
		g := w.Grad().Data()
		after := math.Hypot(float64(g[0]), float64(g[1]))
		if math.Abs(norm-5) > 1e-5 || math.Abs(after-1) > 1e-5 {
			t.Fatalf("tensör biçimi %v: norm %v (5 olmalı), kırpma sonrası %v (1 olmalı)", tensorForm, norm, after)
		}
	}
}

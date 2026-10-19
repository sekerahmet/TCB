package tensor

import (
	"fmt"
	"math/rand"
	"testing"
)

// Bulgu 15a: AdamW'nin sapma düzeltmesi parametre başına adım sayısını kullanır: ilk
// adımlarda gradyanı olmayan bir parametrenin ilk güncellemesi, yeni bir iyileştiricinin
// ilk güncellemesiyle aynıdır.
func TestAdamWParametreBasinaAdim(t *testing.T) {
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
	tek := New([]float32{3, 4}, 2).SetRequiresGrad(true)
	op2 := NewAdamW([]*Tensor{tek}, 0.1, 0)
	tek.Square().SumAll().Backward()
	op2.Step()
	yakin(t, "gecikmeli ilk adım", b.Data(), tek.Data(), 0)
	if op.StepCount() != 4 {
		t.Fatalf("StepCount %d", op.StepCount())
	}
}

// Bulgu 15b: iyileştirici durumu (momentler, adım sayıları, 16 bitlik parametrelerin
// float32 ana kopyası, SGD hızı) dışa alınıp yeni bir iyileştiriciye yüklenebilir;
// sürdürülen eğitim kesintisiz eğitimle bit düzeyinde aynıdır.
func TestIyilestiriciDurumuSurdurme(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	w0 := Randn(rng, 4, 3)
	b0 := Randn(rng, 3).Cast(BFloat16)
	x := Randn(rng, 5, 4)
	adim := func(params []*Tensor, op Optimizer) {
		op.ZeroGrad()
		Add(x.MatMul(params[0]), params[1]).Square().MeanAll().Backward()
		op.Step()
	}
	for _, tur := range []string{"AdamW", "SGD"} {
		yeni := func(params []*Tensor) interface {
			Optimizer
			State() map[string]*Tensor
			LoadState(map[string]*Tensor)
		} {
			if tur == "AdamW" {
				return NewAdamW(params, 0.01, 0.1)
			}
			return NewSGD(params, 0.05, 0.9)
		}
		kopya := func() []*Tensor {
			return []*Tensor{w0.Clone().Detach().SetRequiresGrad(true), b0.Clone().Detach().SetRequiresGrad(true)}
		}
		// Kesintisiz: 8 adım.
		pa := kopya()
		oa := yeni(pa)
		for i := 0; i < 8; i++ {
			adim(pa, oa)
		}
		// 5 adım, durumu al, yeni iyileştiriciyle 3 adım daha.
		pb := kopya()
		ob := yeni(pb)
		for i := 0; i < 5; i++ {
			adim(pb, ob)
		}
		durum := ob.State()
		pc := []*Tensor{pb[0].Clone().Detach().SetRequiresGrad(true), pb[1].Clone().Detach().SetRequiresGrad(true)}
		oc := yeni(pc)
		oc.LoadState(durum)
		for i := 0; i < 3; i++ {
			adim(pc, oc)
		}
		for k := range pa {
			yakin(t, fmt.Sprintf("%s param %d", tur, k), pc[k].Data(), pa[k].Data(), 0)
		}
		// Uyumsuz durum açık hata verir.
		panikIcerir(t, tur+" uyumsuz", "durum", func() {
			yeni([]*Tensor{Zeros(2).SetRequiresGrad(true)}).LoadState(durum)
		})
	}
}

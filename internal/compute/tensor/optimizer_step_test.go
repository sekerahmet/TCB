package tensor

import (
	"math/rand"
	"testing"
)

// stepper adımını çekirdek olarak da verebilen iyileştiricidir (AdamW, SGD). Dosya düzeyinde: codegraph
// fonksiyon içindeki tipin metoduna düğüm kurmuyor (TestUseCoverageOnTheRepository).
type stepper interface {
	Optimizer
	StepKernel() *Kernel
}

// Kural (10.2, S-kilit): adımın çekirdeği izinle (kilitsiz yol: TryAcquire, Run, End) çalışınca
// sonuç Step'le bit bit aynıdır: AdamW ve momentumlu SGD, 16 bitlik parametrenin ana kopyası ve
// listede iki kez geçen parametre (sıralı yol) dahil. Hazırlık bir kez boşa yinelense de
// (çakışmada olduğu gibi) sayaçlar ve sonuç değişmez.
func TestStepKernelMatchesStep(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	w0, b0, x := Randn(rng, 4, 3), Randn(rng, 3).Cast(BFloat16), Randn(rng, 5, 4)
	for _, name := range []string{"AdamW", "SGD", "AdamW-repeated"} {
		run := func(viaKernel bool) [][]float32 {
			w, b := w0.Clone().SetRequiresGrad(true), b0.Clone().SetRequiresGrad(true)
			params := []*Tensor{w, b}
			if name == "AdamW-repeated" {
				params = []*Tensor{w, b, w}
			}
			var op stepper = NewAdamW(params, 0.01, 0.1)
			if name == "SGD" {
				op = NewSGD(params, 0.05, 0.9)
			}
			for range 4 {
				op.ZeroGrad()
				Add(x.MatMul(w), b).Square().MeanAll().Backward()
				if !viaKernel {
					op.Step()
					continue
				}
				_ = op.StepKernel() // boşa hazırlık: çakışmada yinelenen hazırlık gibi
				k := op.StepKernel()
				if !k.TryAcquire() {
					t.Fatalf("%s: adımın izinleri alınamadı", name)
				}
				k.Run()
				k.End()
			}
			if a, ok := op.(*AdamW); ok && a.StepCount() != 4 {
				t.Fatalf("%s: adım sayısı %d, 4 olmalı", name, a.StepCount())
			}
			return [][]float32{w.Data(), b.Data()}
		}
		want, got := run(false), run(true)
		for i := range want {
			bitEqual32(t, name, got[i], want[i])
		}
	}
	if unlockedOps.Load() != 0 {
		t.Fatal("adımdan sonra izin kaldı")
	}
}

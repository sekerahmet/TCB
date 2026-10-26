package tensor

import (
	"slices"
	"testing"
)

// Kural (20.7, 20.12): AdamW.LoadState ya hep ya hiç çalışır: CheckState yazmadan önce
// loadState'in isteyeceği her şeyi (şekil, cihaz, canlı ve verisi olan tensör) denetler;
// ikinci parametrenin durumu bozuksa (bırakılmış tensör) iyileştirici hiç değişmez.
// Neden: eskiden CheckState yalnızca şekli denetliyor, LoadState 0. parametreyi yazdıktan sonra
// 1.'de paniğe geçiyordu (Gemini kararlılık denetimi, bulgu 2).
func TestLoadStateChecksEverythingBeforeWriting(t *testing.T) {
	p0 := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	p1 := New([]float32{3, 4}, 2).SetRequiresGrad(true)
	op := NewAdamW([]*Tensor{p0, p1}, 0.01, 0)

	// Başlangıç durumu: 1 adım atılmış (step = 1, steps = [1, 1]).
	p0.Add(p1).SumAll().Backward()
	op.Step()
	origState := op.State()
	origM0 := origState["m.0"].Data()
	origM1 := origState["m.1"].Data()

	// Yeni durum: 7. adıma ait değerler içeriyor, ancak m.1 bırakılmış (Drop).
	badState := map[string]*Tensor{
		"step":   counterTensor(7),
		"step.0": counterTensor(7),
		"m.0":    New([]float32{99, 99}, 2),
		"v.0":    New([]float32{88, 88}, 2),
		"step.1": counterTensor(7),
		"m.1":    New([]float32{77, 77}, 2),
		"v.1":    New([]float32{66, 66}, 2),
	}
	badState["m.1"].Drop()

	// LoadState paniğe geçer, ancak CheckState bunu önceden yakalayamadığı için op yarıda bozulur.
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		op.LoadState(badState)
	}()
	if !panicked {
		t.Fatal("LoadState bırakılmış m.1 tensöründe paniğe geçmeliydi")
	}

	after := op.State()
	gotM0 := after["m.0"].Data()
	gotM1 := after["m.1"].Data()
	if op.StepCount() != 1 || !slices.Equal(gotM0, origM0) || !slices.Equal(gotM1, origM1) {
		t.Fatalf("LoadState paniğe geçtiğinde iyileştirici hiç değişmemeliydi (ya hep ya hiç); "+
			"ancak yarı yüklenmiş kaldı: StepCount=%d (beklenen 1), m.0=%v (beklenen %v), m.1=%v (beklenen %v)",
			op.StepCount(), gotM0, origM0, gotM1, origM1)
	}
}

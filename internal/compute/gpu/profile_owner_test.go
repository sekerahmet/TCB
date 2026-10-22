package gpu

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Çekirdek süresi onu başlatan işleme yazılır (ileri ve geri ayrı); adım sınırı profile
// bırakılınca iyileştiricinin kendi işareti yok sayılır, MarkStep işaretler.
func TestOwnerSamplesAndExternalSteps(t *testing.T) {
	EnableProfile(true)
	defer EnableProfile(false)
	g := gpuBackend()
	before := map[tensor.Owner]int64{}
	for _, o := range OwnerSamples() {
		before[o.Owner] = o.Calls
	}
	a := tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	b := tensor.NewOn(gpuDevice, []float32{1, 0, 0, 1, 1, 1}, 3, 2)
	y := tensor.MatMul(a, b).SumAll()
	y.Backward()
	_ = y.Item()
	var fwd, back int64
	for _, o := range OwnerSamples() {
		if o.Owner.Kind == tensor.OpKindMatMul {
			if o.Owner.Backward {
				back += o.Calls - before[o.Owner]
			} else {
				fwd += o.Calls - before[o.Owner]
			}
		}
	}
	if fwd == 0 || back == 0 {
		t.Errorf("matris çarpımının çekirdekleri: ileri %d, geri %d; ikisi de sayılmalı", fwd, back)
	}

	SetExternalSteps(true)
	defer SetExternalSteps(false)
	steps := func() int {
		g.profiler.mu.Lock()
		defer g.profiler.mu.Unlock()
		return len(g.profiler.steps)
	}
	n := steps()
	g.enter()
	g.markStep() // iyileştiricinin işareti: yok sayılır
	g.exit()
	if steps() != n {
		t.Errorf("profil adımı verirken iyileştiricinin işareti adım açtı")
	}
	MarkStep()
	if steps() != n+1 {
		t.Errorf("MarkStep adım açmadı: %d → %d", n, steps())
	}
}

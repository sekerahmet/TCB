package tensor

import (
	"testing"
	"time"
	"unsafe"
)

// Geri yayılımın süresi ve bittiği an sayılır; iyileştiricinin adımı kancayı kendi kimliğiyle
// çağırır.
func TestPhaseBackwardTimesAndOptimizerHook(t *testing.T) {
	w := Ones(64, 64).SetRequiresGrad(true)
	opt := NewAdamW([]*Tensor{w}, 0.01, 0)
	var ids []uintptr
	SetOptimizerStepHook(func(id uintptr, _ time.Time) { ids = append(ids, id) })
	defer SetOptimizerStepHook(nil)
	before, _ := BackwardTimes()
	start := time.Now()
	MatMul(w, w).Reshape(-1).SumAll().Backward()
	after, end := BackwardTimes()
	if after <= before || end.Before(start.Add(-time.Millisecond)) {
		t.Errorf("geri süresi %v → %v, bitiş %v (başlangıç %v); süre artmalı, bitiş sonra olmalı", before, after, end, start)
	}
	opt.Step()
	opt.Step()
	if len(ids) != 2 || ids[0] != uintptr(unsafe.Pointer(opt)) {
		t.Errorf("kanca çağrıları %v; iki kez, iyileştiricinin kimliğiyle olmalı", ids)
	}
}

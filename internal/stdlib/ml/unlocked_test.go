package ml

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// matMulCalls CPU'daki ileri matris çarpımlarının süreç başından beri sayısıdır.
func matMulCalls() int64 {
	for _, r := range tensor.CounterSnapshot() {
		if r.Device == tensor.KindCPU && !r.Backward && r.OpKind == tensor.OpKindMatMul {
			return r.Calls
		}
	}
	return 0
}

// Kural (H1 profil payı A-5; DENETİM 902 Not 1): runUnlocked izin alamayınca hazırlığı bırakır
// (Kernel.Abandon) ve çakışan işlemin bitmesini bekler; yeniden hazırlanıp çalışan çarpım bir kez
// sayılır. Çakışan işlem, izinleri tutulan ilk çarpımdır: runUnlocked onun yazdığı depoyu okur.
// Sayaçlar süreç geneli olduğundan test t.Parallel kullanmaz.
func TestRunUnlockedAbandonsWaitingPreparation(t *testing.T) {
	a, b := tensor.New(make([]float32, 64), 8, 8), tensor.New(make([]float32, 64), 8, 8)
	m := interp.New()
	before := matMulCalls()
	first, k1 := tensor.MatMulKernel(a, b, tensor.Float32)
	if !k1.TryAcquire() {
		t.Fatal("ilk çarpım izin alamadı")
	}
	started, done := make(chan struct{}), make(chan struct{})
	go func() {
		close(started)
		runUnlocked(m, func() (*tensor.Tensor, *tensor.Kernel) { return tensor.MatMulKernel(first, b, tensor.Float32) })
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("runUnlocked çakışan işlemi beklemeden bitti")
	case <-time.After(50 * time.Millisecond):
	}
	k1.Run()
	k1.End()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runUnlocked çakışan işlem bittikten sonra da bekledi")
	}
	if got := matMulCalls() - before; got != 2 {
		t.Errorf("iki çarpım %d kez sayıldı, beklenen 2 (bırakılan hazırlık sayılmaz)", got)
	}
}

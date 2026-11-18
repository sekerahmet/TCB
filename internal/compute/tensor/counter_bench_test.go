package tensor

import "testing"

// Profil sayaçlarının ek yükünü ölçen küçük tensörlü kıyaslar: her işlem birkaç yüz
// nanosaniye sürer, sayaçların payı en çok burada görünür. Önce/sonra ölçümünde iki
// sürümün test ikilileri aynı koşuda, sırası turdan tura dönerek koşar:
//
//	go test -run XXX -bench BenchmarkCounter -count 3 ./internal/compute/tensor/

func BenchmarkCounterSmallAdd(b *testing.B) {
	x := New(make([]float32, 16), 4, 4)
	y := New(make([]float32, 16), 4, 4)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Add(x, y)
	}
}

// BenchmarkCounterStartStop bir işlemin sayaç yükünün kendisidir (sayma, etkin işlem, iki saat
// okuması).
func BenchmarkCounterStartStop(b *testing.B) {
	be := Backend(cpuBackend{})
	for i := 0; i < b.N; i++ {
		sy := counterStart(be, OpKindAdd).start()
		sy.finish()
	}
}

func BenchmarkCounterSmallForwardBackward(b *testing.B) {
	w := New(make([]float32, 16), 4, 4).SetRequiresGrad(true)
	x := New(make([]float32, 16), 4, 4)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		k := MatMul(x, w).Relu().MulScalar(0.5).SumAll()
		k.Backward()
		w.ZeroGrad()
	}
}

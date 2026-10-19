package tensor

import "testing"

// Profil sayaçlarının ek yükünü ölçen küçük tensörlü kıyaslar: her işlem birkaç yüz
// nanosaniye sürer, sayaçların payı en çok burada görünür. Önce/sonra ölçümü için aynı
// dosya sayaçsız sürümde de derlenir (yalnızca dışa açık işlemleri çağırır):
//
//	go test -run XXX -bench SayacKucuk -count 6 ./internal/compute/tensor/

func BenchmarkSayacKucukToplama(b *testing.B) {
	x := New(make([]float32, 16), 4, 4)
	y := New(make([]float32, 16), 4, 4)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Add(x, y)
	}
}

// BenchmarkSayacBaslaBitir bir işlemin sayaç yükünün kendisidir (sayma, etkin işlem, 1/32
// olasılıkla saat okuması); yalnızca sayaçlı sürümde derlenir.
func BenchmarkSayacBaslaBitir(b *testing.B) {
	be := Backend(cpuBackend{})
	for i := 0; i < b.N; i++ {
		sy := sayacBasla(be, IslemToplama).baslat()
		sy.bitir()
	}
}

func BenchmarkSayacKucukIleriGeri(b *testing.B) {
	w := New(make([]float32, 16), 4, 4).SetRequiresGrad(true)
	x := New(make([]float32, 16), 4, 4)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		k := MatMul(x, w).Relu().MulScalar(0.5).SumAll()
		k.Backward()
		w.ZeroGrad()
	}
}

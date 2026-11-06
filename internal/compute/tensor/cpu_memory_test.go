package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"runtime/debug"
	"slices"
	"sync"
	"testing"
	"weak"
)

// Geri yayılımda iade edilen ara gradyanlar havuzdan yeniden kullanıldığında
// sonuçlar değişmemeli (yeniden kullanılan depolama sıfırlanmış olmalı).
func TestPoolReuse(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	x := Randn(rng, 64, 128)
	W := Randn(rng, 128, 96).SetRequiresGrad(true)
	b := Randn(rng, 96).SetRequiresGrad(true)
	step := func() ([]float32, []float32) {
		W.n.grad, b.n.grad = nil, nil
		h := Add(MatMul(x, W), b).Gelu()
		h.Reshape(32, -1).Square().Mean(1, false).SumAll().Backward()
		return W.Grad().Data(), b.Grad().Data()
	}
	w0, b0 := step()
	for i := 0; i < 5; i++ {
		w1, b1 := step()
		near(t, "W gradyanı", w1, w0, 0)
		near(t, "b gradyanı", b1, b0, 0)
	}

	be := cpuBackend{}
	s := be.Alloc(poolMin, Float32)
	be.Fill(s, 7)
	be.Release(s)
	expectPanic(t, "iki kez iade", func() { be.Release(s) })
	for i := 0; i < 3; i++ {
		y := be.Alloc(poolMin, Float32)
		for _, v := range cd(y) {
			if v != 0 {
				t.Fatal("havuzdan gelen depolama sıfırlanmamış")
			}
		}
		be.Release(y)
	}
}

// Sıfırlanmadan ayrılan (emptyOf) çıktılar: havuzda NaN dolu depolamalar
// bekliyorken işlemler tüm çıktıyı yazmalı; sonuç taze ayırmayla aynı olmalı.
func TestEmptyAllocPoolGarbage(t *testing.T) {
	rng := rand.New(rand.NewSource(22))
	const R, D = 64, 128 // R*D = 8192 >= havuzEnAz
	x := Randn(rng, R, D)
	y := Randn(rng, R, D)
	g := Randn(rng, D)
	bt := Randn(rng, D)
	idx := make([]int, R)
	for i := range idx {
		idx[i] = (i * 7) % D
	}
	w := Randn(rng, D, D)
	ops := []struct {
		ad string
		f  func() *Tensor
	}{
		{"add", func() *Tensor { return Add(x, y) }},
		{"mul_yayin", func() *Tensor { return Mul(x, g) }},
		{"gelu", func() *Tensor { return x.Gelu() }},
		{"softmax", func() *Tensor { return x.Softmax(-1) }},
		{"softmax_eksen0", func() *Tensor { return x.Softmax(0) }},
		{"layer_norm", func() *Tensor { return LayerNorm(x, g, bt, 1e-5) }},
		{"rms_norm", func() *Tensor { return RMSNorm(x, g, 1e-5) }},
		{"embedding", func() *Tensor { return Embedding(w, idx, R) }},
		{"contiguous", func() *Tensor { return x.Reshape(D, R).T().Contiguous() }},
		{"clone", func() *Tensor { return x.Clone() }},
		{"matmul_devrik", func() *Tensor { return MatMul(x.Reshape(D, R).T(), w) }},
		{"bf16", func() *Tensor { return x.Cast(BFloat16).Cast(Float32) }},
	}
	be := cpuBackend{}
	nan := float32(math.NaN())
	for _, is := range ops {
		want := is.f().Data()
		for _, dt := range []DType{Float32, BFloat16} {
			var storages []Storage
			for k := 0; k < 4; k++ {
				s := be.Alloc(R*D, dt)
				be.Fill(s, nan)
				storages = append(storages, s)
			}
			for _, s := range storages {
				be.Release(s)
			}
		}
		got := is.f().Data()
		near(t, is.ad, got, want, 0)
	}
}

// Kural (15.53): büyük depo toplamalar arasında havuzdan yeniden gelir, ama toplayıcı onu canlı saymaz:
// boş tampon canlı sayılınca hedef onun 3 katı büyüyordu. sync.Pool iş parçacığı değişirse boş dönebilir,
// yeniden kullanım birkaç denemeyle sınanır.
func TestPoolHoldsLargeStoragesWeakly(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	b := cpuBackend{}
	const big = poolWeakBytes/4 + 1
	s := b.Alloc(big, Float32)
	reused := false
	for range 100 {
		b.Release(s)
		got := b.allocUninit(big, Float32)
		reused = got == s
		s = got
		if reused {
			break
		}
	}
	if !reused {
		t.Fatal("toplama olmadan büyük depo havuzdan gelmedi")
	}
	b.Release(s)
	gone := weak.Make(s.(*cpuStorage))
	s = nil
	runtime.GC()
	if gone.Value() != nil {
		t.Fatal("havuz büyük depoyu toplamadan geçirdi: canlı sayılır, hedef büyür")
	}
	if got := b.allocUninit(big, Float32).(*cpuStorage); len(got.d) != big {
		t.Fatalf("yeni ayırma %d öğe", len(got.d))
	}
}

// Toplama sürerken havuzdan alınan büyük depo başka işçiyle paylaşılmaz, Alloc'un verdiği sıfırlıdır;
// -race yarışı da görür (DENETİM 223 N1).
func TestPoolLargeStoragesSurviveCollections(t *testing.T) {
	const big, workers, turns = poolWeakBytes/4 + 1, 8, 200
	stop := make(chan struct{})
	var gc sync.WaitGroup
	gc.Add(1)
	go func() {
		defer gc.Done()
		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
			}
		}
	}()
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := cpuBackend{}
			mark := float32(w + 1)
			for i := range turns {
				s := b.Alloc(big, Float32).(*cpuStorage)
				if i%2 == 1 {
					b.Release(s)
					s = b.allocUninit(big, Float32).(*cpuStorage)
				} else if s.d[0] != 0 || s.d[big-1] != 0 {
					t.Errorf("Alloc sıfırlı değil: %v %v", s.d[0], s.d[big-1])
					return
				}
				s.d[0], s.d[big-1] = mark, mark
				runtime.Gosched()
				if s.d[0] != mark || s.d[big-1] != mark {
					t.Errorf("depo başka işçiye de verildi: %v %v, %v olmalıydı", s.d[0], s.d[big-1], mark)
					return
				}
				b.Release(s)
			}
		}()
	}
	wg.Wait()
	close(stop)
	gc.Wait()
}

// Havuzun yeni mi eski mi depo verdiği sonucu değiştirmez: ileri ve geri, araya toplama girse de bit
// bit aynıdır (DENETİM 223 N1).
func TestPoolCollectionsKeepTheBits(t *testing.T) {
	step := func(collect bool) ([]float32, []float32) {
		const m, k = 512, 1024
		x, w := make([]float32, m*k), make([]float32, k*k)
		for i := range x {
			x[i] = float32(i%13) - 6
		}
		for i := range w {
			w[i] = float32(i%7)/64 - 0.05
		}
		a, b := New(x, m, k), New(w, k, k)
		b.SetRequiresGrad(true)
		if collect {
			runtime.GC()
		}
		y := a.MatMul(b).MatMul(b)
		if collect {
			runtime.GC()
			runtime.GC()
		}
		y.Sum(0, false).Sum(0, false).Backward()
		return y.Data(), b.Grad().Data()
	}
	y0, g0 := step(false)
	for _, collect := range []bool{true, false, true} {
		y, g := step(collect)
		if !slices.Equal(y, y0) || !slices.Equal(g, g0) {
			t.Fatalf("toplama %v: sonuç değişti", collect)
		}
	}
}

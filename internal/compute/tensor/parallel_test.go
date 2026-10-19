package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// Paralel çekirdek yollarının doğruluk ve belirlenimcilik testleri. Boyutlar,
// paralelleştirme eşiklerini aşacak şekilde seçilmiştir.

// naiveSum float64 birikimli başvuru eksen toplamı.
func naiveSum(xd []float32, shape []int, axis int) []float32 {
	outer, dim, inner := axisOrder(shape, axis)
	out := make([]float32, outer*inner)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			var s float64
			for d := 0; d < dim; d++ {
				s += float64(xd[(o*dim+d)*inner+i])
			}
			out[o*inner+i] = float32(s)
		}
	}
	return out
}

func TestSumParallelPaths(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	cases := []struct {
		shape []int
		axis  int
	}{
		{[]int{3, 100000}, 1},    // son eksen, birkaç satır
		{[]int{1, 1 << 17}, 1},   // son eksen, tek satır (parçalı toplam)
		{[]int{100, 7, 300}, 1},  // çok dış satır
		{[]int{2, 5, 4096}, 1},   // az dış satır, geniş iç eksen (sütun blokları)
		{[]int{1, 5000, 10}, 1},  // az dış satır, dar iç eksen (eksen parçaları)
		{[]int{5000, 10}, 0},     // eksen 0
		{[]int{2048, 512}, 0},    // eksen 0, orta genişlik
		{[]int{3, 4}, 0},         // küçük
		{[]int{0, 4}, 0},         // boş
		{[]int{4, 0, 3}, 1},      // boş eksen
		{[]int{7, 33, 1, 5}, -2}, // uzunluğu 1 olan eksen
	}
	for _, d := range cases {
		x := Randn(rng, d.shape...)
		got := x.Sum(d.axis, false).Data()
		want := naiveSum(x.Data(), x.shape, normAxis(d.axis, len(d.shape)))
		near(t, "sum", got, want, 1e-5)
		// Belirlenimcilik: tekrar ve farklı işçi sayısıyla bit düzeyinde aynı.
		old := runtime.GOMAXPROCS(3)
		repeat := x.Sum(d.axis, false).Data()
		runtime.GOMAXPROCS(old)
		for i := range got {
			if math.Float32bits(got[i]) != math.Float32bits(repeat[i]) {
				t.Fatalf("sum %v eksen %d belirlenimci değil: [%d] %v != %v", d.shape, d.axis, i, got[i], repeat[i])
			}
		}
	}
}

func TestSumAllParallel(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	x := Randn(rng, 3, 100003)
	var s float64
	for _, v := range x.Data() {
		s += float64(v)
	}
	got := x.SumAll().Item()
	if math.Abs(float64(got)-s) > 1e-3 {
		t.Fatalf("SumAll %v, beklenen %v", got, s)
	}
	old := runtime.GOMAXPROCS(2)
	repeat := x.SumAll().Item()
	runtime.GOMAXPROCS(old)
	if got != repeat {
		t.Fatalf("SumAll belirlenimci değil: %v != %v", got, repeat)
	}
	// Devrik (bitişik olmayan) görünüm.
	near(t, "devrik SumAll", []float32{x.T().SumAll().Item()}, []float32{float32(s)}, 1e-5)
}

// singleAndMany fn'i tek işçiyle ve çok işçiyle çalıştırıp sonuçların bit düzeyinde
// aynı olduğunu denetler (paralel yollar, sıralı yolla aynı toplama sırasını
// korumalıdır).
func singleAndMany(t *testing.T, ad string, fn func() []float32) {
	t.Helper()
	old := runtime.GOMAXPROCS(1)
	single := fn()
	runtime.GOMAXPROCS(max(old, 8))
	many := fn()
	runtime.GOMAXPROCS(old)
	if len(single) != len(many) {
		t.Fatalf("%s: uzunluk %d != %d", ad, len(single), len(many))
	}
	for i := range single {
		if math.Float32bits(single[i]) != math.Float32bits(many[i]) {
			t.Fatalf("%s: [%d] tek işçi %v, çok işçi %v", ad, i, single[i], many[i])
		}
	}
}

func TestParallelKernelsMatchSerial(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	x := Randn(rng, 256, 3, 200)
	w := Randn(rng, 200)
	k := Randn(rng, 256, 3, 1)
	table := Randn(rng, 300, 128)
	idx := make([]int, 1000)
	for i := range idx {
		idx[i] = rng.Intn(40) * 7 // yinelenen indeksler
	}
	gradlar := func(f func() *Tensor, ps ...*Tensor) []float32 {
		for _, p := range ps {
			p.SetRequiresGrad(true)
			p.grad = nil
		}
		out := f()
		out.BackwardWith(Ones(out.shape...))
		var r []float32
		r = append(r, out.Data()...)
		for _, p := range ps {
			r = append(r, p.Grad().Data()...)
			p.SetRequiresGrad(false)
			p.grad = nil
		}
		return r
	}
	cases := []struct {
		ad string
		fn func() []float32
	}{
		{"devrik kopya", func() []float32 { return x.Permute(2, 0, 1).Contiguous().Data() }},
		{"dilim geri yayılımı", func() []float32 {
			return gradlar(func() *Tensor { return x.Slice(2, 10, 150).Mul(x.Slice(2, 50, 190)) }, x)
		}},
		{"eksen toplamı geri yayılımı", func() []float32 {
			return gradlar(func() *Tensor { return x.Sum(-1, true).Mul(k) }, x, k)
		}},
		{"sapma gradyanı", func() []float32 { return gradlar(func() *Tensor { return Add(x, w) }, x, w) }},
		{"orta eksen yayını", func() []float32 { return gradlar(func() *Tensor { return Mul(x, k) }, x, k) }},
		{"gömme geri yayılımı", func() []float32 {
			return gradlar(func() *Tensor { return Embedding(table, idx).Square() }, table)
		}},
		{"kare toplamı", func() []float32 {
			p := x.Detach().SetRequiresGrad(true)
			p.grad = p.be.Alloc(p.Numel(), Float32)
			p.be.Copy(p.grad, 0, x.view())
			n := ClipGradNorm([]*Tensor{p}, 1)
			return append([]float32{float32(n)}, p.Grad().Data()...)
		}},
		{"doldurma", func() []float32 { return Full(2.5, 300, 300).Data() }},
	}
	for _, d := range cases {
		singleAndMany(t, d.ad, d.fn)
	}
	// Başvuru değerleriyle karşılaştırma (sıralı yol da doğru olmalı).
	gw := gradlar(func() *Tensor { return Embedding(table, idx) }, table)[len(idx)*128:]
	expected := make([]float32, 300*128)
	for _, ix := range idx {
		for j := 0; j < 128; j++ {
			expected[ix*128+j]++
		}
	}
	near(t, "gömme gradyanı", gw, expected, 0)
	dev := x.Permute(2, 0, 1).Contiguous().Data()
	xd := x.Data()
	for c := 0; c < 200; c += 37 {
		for a := 0; a < 256; a += 51 {
			if dev[(c*256+a)*3+1] != xd[(a*3+1)*200+c] {
				t.Fatalf("devrik kopya yanlış (%d, %d)", c, a)
			}
		}
	}
}

// LayerNorm/RMSNorm ölçek ve sapma gradyanları belirlenimci olmalı: tekrar eden
// çalıştırmalarda ve farklı işçi sayılarında bit düzeyinde aynı.
func TestNormGradDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	x := Randn(rng, 3000, 48)
	gamma := Randn(rng, 48)
	beta := Randn(rng, 48)
	w := Randn(rng, 3000, 48)
	for _, ad := range []string{"layer_norm", "rms_norm"} {
		fn := func() []float32 {
			x.SetRequiresGrad(true)
			gamma.SetRequiresGrad(true)
			beta.SetRequiresGrad(true)
			x.grad, gamma.grad, beta.grad = nil, nil, nil
			var y *Tensor
			if ad == "layer_norm" {
				y = LayerNorm(x, gamma, beta, 1e-5)
			} else {
				y = RMSNorm(x, gamma, 1e-5)
			}
			Mul(y, w).SumAll().Backward()
			r := append(gamma.Grad().Data(), x.Grad().Data()...)
			if beta.grad != nil {
				r = append(r, beta.Grad().Data()...)
			}
			return r
		}
		first := fn()
		for k := 0; k < 3; k++ {
			singleAndMany(t, ad, fn)
			repeat := fn()
			for i := range first {
				if math.Float32bits(first[i]) != math.Float32bits(repeat[i]) {
					t.Fatalf("%s gradyanı çalıştırmadan çalıştırmaya değişti: [%d] %v != %v", ad, i, first[i], repeat[i])
				}
			}
		}
	}
}

// İşçi havuzu: her parça tam bir kez yürütülmeli, katılımcı numarası [0, w)
// içinde kalmalı ve aynı numara aynı anda iki goroutine'de çalışmamalı.
func TestDistributePartsExactlyOnce(t *testing.T) {
	old := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(old)
	for _, w := range []int{1, 2, 3, 4, 8, 20} {
		for _, part := range []int{1, 2, 7, 64, 1000} {
			counter := make([]atomic.Int32, part)
			busy := make([]atomic.Int32, w)
			var failure atomic.Value
			distribute(w, part, func(k, c int) {
				if k < 0 || k >= w {
					failure.Store("katılımcı numarası aralık dışında")
					return
				}
				if busy[k].Add(1) != 1 {
					failure.Store("aynı katılımcı numarası eşzamanlı kullanıldı")
				}
				counter[c].Add(1)
				busy[k].Add(-1)
			})
			if h := failure.Load(); h != nil {
				t.Fatalf("w=%d parça=%d: %v", w, part, h)
			}
			for c := range counter {
				if n := counter[c].Load(); n != 1 {
					t.Fatalf("w=%d parça=%d: parça %d %d kez yürütüldü", w, part, c, n)
				}
			}
		}
	}
}

// Eşzamanlı ve iç içe çağrılar (havuz meşgulken geçici goroutine'lere düşer)
// ve çalışırken değişen GOMAXPROCS doğru sonuç vermeli.
func TestDistributeConcurrentAndNested(t *testing.T) {
	old := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(old)
	const n = 1 << 18
	sumFn := func() int64 {
		var s atomic.Int64
		parallelFor(n, 1, func(lo, hi int) {
			// İç içe: her parça kendi aralığını yine paralel toplar.
			var ic atomic.Int64
			parallelFor(hi-lo, 4, func(a, b int) {
				var y int64
				for i := lo + a; i < lo+b; i++ {
					y += int64(i)
				}
				ic.Add(y)
			})
			s.Add(ic.Load())
		})
		return s.Load()
	}
	expected := int64(n) * (n - 1) / 2
	for _, p := range []int{8, 2, 20, 1, 4} {
		runtime.GOMAXPROCS(p)
		var wg sync.WaitGroup
		errs := make(chan int64, 8)
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 20; k++ {
					if s := sumFn(); s != expected {
						errs <- s
						return
					}
				}
			}()
		}
		wg.Wait()
		close(errs)
		for s := range errs {
			t.Fatalf("GOMAXPROCS=%d: toplam %d, beklenen %d", p, s, expected)
		}
	}
}

// İşçi sayısı seçimi: küçük işler dönen işçilerle sınırlı, büyük işler bütün
// çekirdekleri kullanabilir, her katılımcıya en az enAz birim düşer.
func TestWorkerCount(t *testing.T) {
	old := runtime.GOMAXPROCS(20)
	defer runtime.GOMAXPROCS(old)
	cases := []struct{ is, minN, large, want int }{
		{100, 1000, 1 << 20, 1},
		{2500, 1000, 1 << 20, 2},
		{1 << 19, 1000, 1 << 20, returningWorker + 1},
		{1 << 20, 1000, 1 << 20, 20},
		{1 << 20, 1 << 17, 1 << 20, 8},
	}
	for _, d := range cases {
		if got := workerCount(d.is, d.minN, d.large); got != d.want {
			t.Errorf("isciSayisi(%d, %d, %d) = %d, beklenen %d", d.is, d.minN, d.large, got, d.want)
		}
	}
	runtime.GOMAXPROCS(2)
	if got := workerCount(1<<30, 1, 1); got != 2 {
		t.Errorf("GOMAXPROCS=2'de %d işçi seçildi", got)
	}
}

// Bulgu 10: işçi goroutine'inde (ya da çağıranın parçasında) oluşan panik kurtarılıp
// çağırana taşınır; işçiler yarım işi sonraki çağrılarla eşzamanlı yürütmeye devam etmez ve
// havuz kullanılabilir kalır. İç içe (geçici goroutine'li) yolda da aynı.
func TestWorkerPanicReachesCaller(t *testing.T) {
	if workers() < 2 {
		t.Skip("tek işlemci")
	}
	for _, nested := range []bool{false, true} {
		execute := func(fn func()) {
			if nested {
				distribute(2, 2, func(k, c int) {
					if c == 0 {
						fn()
					}
				})
				return
			}
			fn()
		}
		var executed atomic.Int64
		m := panicMessage(func() {
			execute(func() {
				distribute(4, 64, func(_, c int) {
					executed.Add(1)
					if c == 5 {
						panic("tensor: işçide hata")
					}
				})
			})
		})
		if m != "tensor: işçide hata" {
			t.Fatalf("iç içe=%v: panik çağırana taşınmadı: %q", nested, m)
		}
		// Havuz hâlâ çalışır ve bütün parçaları yürütür.
		var n atomic.Int64
		distribute(4, 100, func(_, c int) { n.Add(1) })
		if n.Load() != 100 {
			t.Fatalf("panik sonrası havuz %d/100 parça yürüttü", n.Load())
		}
	}
}

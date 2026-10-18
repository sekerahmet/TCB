package tensor

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// paralelEsik bu kadar elemandan küçük işler tek çekirdekte yürütülür;
// goroutine kurma maliyeti kazancı aşmasın diye.
const paralelEsik = 1 << 15

// workers kullanılacak işçi sayısı (GOMAXPROCS).
func workers() int {
	w := runtime.GOMAXPROCS(0)
	if w < 1 {
		w = 1
	}
	return w
}

// parallelFor [0, n) aralığını parçalara bölüp fn(lo, hi) çağrılarını işçilere
// dağıtır. cost, bir birimin yaklaşık eleman maliyetidir; toplam iş paralelEsik
// altındaysa doğrudan çalıştırılır. İşçi sayısı işe göre sınırlanır: her işçiye
// en az paralelEsik kadar iş düşer (küçük işlerde gereksiz goroutine kurulmaz).
func parallelFor(n, cost int, fn func(lo, hi int)) {
	if n <= 0 {
		return
	}
	w := workers()
	is := n * cost
	if w == 1 || n == 1 || is < paralelEsik {
		fn(0, n)
		return
	}
	if lim := is / paralelEsik; w > lim {
		w = max(lim, 1)
	}
	if w == 1 {
		fn(0, n)
		return
	}
	// Yük dengesi için işçi başına birkaç parça.
	chunks := w * 4
	if chunks > n {
		chunks = n
	}
	size := (n + chunks - 1) / chunks
	chunks = (n + size - 1) / size
	var next atomic.Int64
	var wg sync.WaitGroup
	if w > chunks {
		w = chunks
	}
	wg.Add(w)
	for k := 0; k < w; k++ {
		go func() {
			defer wg.Done()
			for {
				c := int(next.Add(1) - 1)
				if c >= chunks {
					return
				}
				lo := c * size
				hi := lo + size
				if hi > n {
					hi = n
				}
				fn(lo, hi)
			}
		}()
	}
	wg.Wait()
}

// indirgemeParca belirlenimci paralel toplamlarda sabit parça uzunluğu. Parça
// sınırları yalnızca eleman sayısına bağlıdır (işçi sayısına değil); kısmi
// toplamlar parça sırasıyla birleştirildiği için sonuç her çalıştırmada ve her
// makinede bit düzeyinde aynıdır.
const indirgemeParca = 1 << 14

// parallelSum [0, n) aralığını indirgemeParca uzunluklu parçalara böler,
// fn(lo, hi) kısmi toplamlarını paralel hesaplar ve sabit sırayla toplar.
func parallelSum(n, cost int, fn func(lo, hi int) float64) float64 {
	if n <= indirgemeParca {
		return fn(0, n)
	}
	np := (n + indirgemeParca - 1) / indirgemeParca
	kismi := make([]float64, np)
	parallelFor(np, indirgemeParca*cost, func(lo, hi int) {
		for c := lo; c < hi; c++ {
			a := c * indirgemeParca
			kismi[c] = fn(a, min(a+indirgemeParca, n))
		}
	})
	var s float64
	for _, v := range kismi {
		s += v
	}
	return s
}

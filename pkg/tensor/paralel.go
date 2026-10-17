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

// parallelFor [0, n) aralığını en az minChunk uzunluklu parçalara bölüp
// fn(lo, hi) çağrılarını işçilere dağıtır. cost, bir birimin yaklaşık eleman
// maliyetidir; toplam iş paralelEsik altındaysa doğrudan çalıştırılır.
func parallelFor(n, cost int, fn func(lo, hi int)) {
	if n <= 0 {
		return
	}
	w := workers()
	if w == 1 || n == 1 || n*cost < paralelEsik {
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

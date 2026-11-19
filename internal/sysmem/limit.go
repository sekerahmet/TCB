package sysmem

import (
	"runtime/debug"
	"sync"
)

// Sürecin bellek tavanı (dil tanımı 19.12a): çöp toplayıcının öbeği ve öbek dışı bloklar (yüklenen
// ağırlıklar, GPU'nun sabitlenmiş kopya halkası) birlikte tavanın altında kalır. Go'nun sınırı yalnız
// kendi eşlediği belleği sayar, syscall'la eşlenen öbek dışı bloğu saymaz; bu yüzden tavan Go'ya
// öbek dışının düşüldüğü kadarını verir: max(tavan − öbek dışı, 0). Öbek dışı büyüdükçe Go'nun payı
// küçülür, toplayıcı sıklaşır; program durmaz (Go sıfır sınırda da ilerler).
//
// Tavan yoksa öbek dışı yine sayılır ama Go'nun sınırına dokunulmaz: Go'nun kendi ayarı kalır.
// Dosya eşlemesi ve GPU'nun belleği sayılmaz (19.12a).
//
// Temizleyiciler ve yükleyiciler AddExternal'ı aynı anda çağırır: sayım ve Go'nun sınırı tek kilitle
// sıralanır, yoksa eski bir toplam yeni toplamın üstüne yazılabilirdi.
var limit struct {
	sync.Mutex
	cap     int64 // 0: tavan yok
	outside int64 // öbek dışı bloklar, bayt
	before  int64 // tavan verilmeden önceki Go sınırı
}

// SetLimit sürecin bellek tavanını n bayt yapar ve Go'nun sınırını kurar; n ≤ 0 tavanı kaldırır ve
// Go'ya tavandan önceki sınırını geri verir. trlang onu TR_MEMORY_LIMIT'ten süreç başında bir kez
// çağırır; kaldırmayı testler kullanır.
func SetLimit(n int64) {
	limit.Lock()
	defer limit.Unlock()
	if n <= 0 {
		if limit.cap > 0 {
			debug.SetMemoryLimit(limit.before)
		}
		limit.cap = 0
		return
	}
	if limit.cap == 0 {
		limit.before = debug.SetMemoryLimit(-1)
	}
	limit.cap = n
	applyLimit()
}

// Limit sürecin bellek tavanını ve verilip verilmediğini döner.
func Limit() (int64, bool) {
	limit.Lock()
	defer limit.Unlock()
	return limit.cap, limit.cap > 0
}

// AddExternal öbek dışı bellek sayımına n bayt ekler (bırakılınca eksi n). Tavan varsa Go'nun
// sınırı yeni toplama göre yeniden kurulur.
func AddExternal(n int64) {
	limit.Lock()
	defer limit.Unlock()
	limit.outside += n
	if limit.cap > 0 {
		applyLimit()
	}
}

// applyLimit Go'nun sınırını tavandan öbek dışını düşerek kurar; kilit tutulurken çağrılır.
func applyLimit() {
	debug.SetMemoryLimit(max(limit.cap-limit.outside, 0))
}

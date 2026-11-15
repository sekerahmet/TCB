package tensor

import (
	"os"
	"runtime/debug"
	"testing"
)

// Blok sınırı çekirdeğin değerinden türer: yarısı Go'ya kalır, blok en çok üç eşleme harcar (DENETİM 239);
// dörtte biri önbelleğin, kalanı ağırlıkların.
func TestHostBlockLimitFollowsMaxMapCount(t *testing.T) {
	text, _ := os.ReadFile("/proc/sys/vm/max_map_count")
	total := hostMapCount(string(text)) / 2 / 3
	if got := hostLimits.blocks(); got != total-total/4 {
		t.Fatalf("ağırlıkların blok sınırı %d, beklenen %d", got, total-total/4)
	}
	if got := hostCacheLimits.blocks(); got != total/4 {
		t.Fatalf("önbelleğin blok sınırı %d, beklenen %d", got, total/4)
	}
}

func TestHostBlockReadAfterReleaseFaults(t *testing.T) {
	stale := releasedSlice(t)
	if n := settleHostMemory(func(n int64) bool { return n == 0 }); n != 0 {
		t.Fatalf("blok bırakılmadı: %d bayt", n)
	}
	defer debug.SetPanicOnFault(debug.SetPanicOnFault(true))
	defer func() {
		if recover() == nil {
			t.Fatal("bırakılan bloktan okuma sessizce geçti")
		}
	}()
	t.Log(stale[7])
}

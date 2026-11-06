package tensor

import (
	"os"
	"testing"
)

// Blok sınırı çekirdeğin değerinden türer: yarısı Go'ya kalır, blok en çok üç eşleme harcar (DENETİM 239).
func TestHostBlockLimitFollowsMaxMapCount(t *testing.T) {
	text, _ := os.ReadFile("/proc/sys/vm/max_map_count")
	want := hostMapCount(string(text)) / 2 / 3
	if got := hostLimits.blocks(); got != want {
		t.Fatalf("blok sınırı %d, beklenen %d", got, want)
	}
}

package sysmem

import (
	"runtime"
	"testing"
)

func TestAvailable(t *testing.T) {
	b, ok := Available()
	switch runtime.GOOS {
	case "windows", "linux":
		if !ok || b == 0 {
			t.Fatalf("kullanılabilir bellek okunamadı: %d bayt (ok=%v)", b, ok)
		}
		total, tok := Physical()
		if !tok {
			t.Fatalf("toplam bellek okunamadı")
		}
		if b > total {
			t.Fatalf("kullanılabilir bellek (%d) toplamdan (%d) büyük", b, total)
		}
	default:
		if ok || b != 0 {
			t.Fatalf("%s üzerinde ok=false bekleniyordu, alınan: %d bayt (ok=%v)", runtime.GOOS, b, ok)
		}
	}
}

package sysmem

import (
	"runtime"
	"testing"
)

func TestPhysical(t *testing.T) {
	b, ok := Physical()
	if ok && b == 0 {
		t.Fatalf("fiziksel bellek okundu ama 0 bayt döndü")
	}
	if !ok && b != 0 {
		t.Fatalf("fiziksel bellek okunamadı ama %d bayt döndü", b)
	}
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		if !ok || b < 256<<20 {
			t.Fatalf("fiziksel bellek okunamadı ya da akla yatkın değil: %d bayt (ok=%v)", b, ok)
		}
	default:
		if ok {
			t.Fatalf("%s üzerinde ok=false bekleniyordu, alınan: %d bayt", runtime.GOOS, b)
		}
	}
}

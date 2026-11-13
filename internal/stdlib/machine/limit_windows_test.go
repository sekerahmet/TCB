package machine

import (
	"testing"
	"unsafe"
)

// TestJobLimitLayout yapının bellekteki düzeni Windows'unkiyle aynı mı (amd64: 144 bayt,
// LimitFlags 16. baytta, ProcessMemoryLimit 112., JobMemoryLimit 120.; hipotez programının
// ölçtüğü yerler).
func TestJobLimitLayout(t *testing.T) {
	var info extendedLimitInformation
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("düzen yalnız 64 bitte sınanır")
	}
	if s := unsafe.Sizeof(info); s != 144 {
		t.Fatalf("boyut %d, beklenen 144", s)
	}
	if o := unsafe.Offsetof(info.basic) + unsafe.Offsetof(info.basic.limitFlags); o != 16 {
		t.Fatalf("LimitFlags %d. baytta, beklenen 16", o)
	}
	if o := unsafe.Offsetof(info.processMemoryLimit); o != 112 {
		t.Fatalf("ProcessMemoryLimit %d. baytta, beklenen 112", o)
	}
	if o := unsafe.Offsetof(info.jobMemoryLimit); o != 120 {
		t.Fatalf("JobMemoryLimit %d. baytta, beklenen 120", o)
	}
}

// TestMemoryLimitOutsideJob iş nesnesi dışında koşan sınamada çağrı okunur ve sınır yoktur.
func TestMemoryLimitOutsideJob(t *testing.T) {
	b, limited, ok := memoryLimit()
	if !ok {
		t.Fatalf("QueryInformationJobObject okunamadı")
	}
	t.Logf("sınır: %d bayt, sınırlı=%v", b, limited)
	if limited && b <= 0 {
		t.Fatalf("sınırlı ama değer %d", b)
	}
}

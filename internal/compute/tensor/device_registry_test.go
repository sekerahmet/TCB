package tensor

import (
	"fmt"
	"strings"
	"testing"
)

// fakeGPU kayıt sınaması için yalnızca cihazı GPU olan bir arka uç.
type fakeGPU struct {
	cpuBackend
	i int
}

func (s fakeGPU) Device() Device { return GPU(s.i) }

func TestLazyBackendRegistration(t *testing.T) {
	built := 0
	RegisterLazyBackend(KindGPU, LazyBackend{
		Count: func() int { return 2 },
		New: func(i int) Backend {
			built++
			return fakeGPU{i: i}
		},
	})
	defer func() {
		isLazy.Lock()
		delete(lazies, KindGPU)
		for d := range tried {
			delete(tried, d)
		}
		isLazy.Unlock()
		backendMu.Lock()
		delete(backends, GPU(0))
		delete(backends, GPU(1))
		backendMu.Unlock()
	}()
	if !HasDevice(GPU(1)) || !HasDevice(GPU(1)) {
		t.Fatal("tembel GPU kurulmadı")
	}
	if HasDevice(GPU(2)) {
		t.Fatal("sayının dışındaki cihaz görünüyor")
	}
	if built != 1 {
		t.Fatalf("kurulum %d kez çağrıldı (1 olmalı)", built)
	}
	if got := fmt.Sprint(Devices()); got != "[cpu gpu gpu:1]" {
		t.Fatalf("cihazlar %s", got)
	}
	if built != 2 {
		t.Fatalf("Devices her cihazı bir kez kurmalı: %d", built)
	}
}

func TestDeviceNames(t *testing.T) {
	for _, d := range []struct {
		c  Device
		ad string
	}{{CPU, "cpu"}, {GPU(0), "gpu"}, {GPU(3), "gpu:3"}} {
		if d.c.String() != d.ad {
			t.Fatalf("%v adı %q, %q bekleniyordu", d.c, d.c.String(), d.ad)
		}
	}
	if m := registerPanic(func() { BackendFor(GPU(5)) }); !strings.Contains(m, "gpu:5 cihazı için arka uç kayıtlı değil") {
		t.Fatalf("kayıtsız cihaz mesajı: %q", m)
	}
}

func registerPanic(fn func()) (m string) {
	defer func() { m = fmt.Sprint(recover()) }()
	fn()
	return ""
}

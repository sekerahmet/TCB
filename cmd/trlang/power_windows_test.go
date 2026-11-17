//go:build windows

package main

import (
	"syscall"
	"testing"
	"unsafe"
)

// TestPowerThrottlingIsOff paketin init'i süreci Windows'un güç kısmasından çıkardı mı: çalışma hızı
// denetlenir ve kısma kapalıdır (power_windows.go).
func TestPowerThrottlingIsOff(t *testing.T) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessInformation")
	if proc.Find() != nil {
		t.Skip("GetProcessInformation yok (Windows 8'den eski)")
	}
	state := struct{ version, controlMask, stateMask uint32 }{1, 0, 0}
	self, _ := syscall.GetCurrentProcess()
	if r, _, err := proc.Call(uintptr(self), 4, uintptr(unsafe.Pointer(&state)), unsafe.Sizeof(state)); r == 0 {
		t.Fatalf("GetProcessInformation: %v", err)
	}
	if state.controlMask&0x1 == 0 || state.stateMask&0x1 != 0 {
		t.Fatalf("güç kısması açık kaldı: denetim %#x, durum %#x; beklenen denetimde 0x1, durumda 0", state.controlMask, state.stateMask)
	}
}

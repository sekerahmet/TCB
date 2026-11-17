//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Windows 11, penceresi önde olmayan süreci kısar: verimli çekirdeklere ve düşük frekansa iter
// (EcoQoS). Dost'ta tarayıcı öndeyken dil modeli 3,5 token/sn, trlang'ın penceresi öndeyken
// 15,1 token/sn üretti; kısılırken iş verimli çekirdeklerde, frekans %51–55'teydi (ölçüldü,
// dizüstü i7-13700H, 2026-11-17). trlang hesap dilidir: süreç kısmadan çıkar. Ayar yalnız bu
// süreci etkiler; Windows'un genel ayarlarına dokunmaz. Eski Windows'ta çağrı yoksa ya da
// başarısızsa trlang olduğu gibi sürer.
func init() {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("SetProcessInformation")
	if proc.Find() != nil {
		return
	}
	// PROCESS_POWER_THROTTLING_STATE: sürüm 1; denetlenen: çalışma hızı; durum 0: kısma kapalı.
	state := struct{ version, controlMask, stateMask uint32 }{1, 0x1, 0}
	const processPowerThrottling = 4 // PROCESS_INFORMATION_CLASS
	self, _ := syscall.GetCurrentProcess()
	_, _, _ = proc.Call(uintptr(self), processPowerThrottling, uintptr(unsafe.Pointer(&state)), unsafe.Sizeof(state))
}

package profil

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	konsolKipiAl     = kernel32.NewProc("GetConsoleMode")
	konsolKipiAyarla = kernel32.NewProc("SetConsoleMode")
)

// sanalTerminalIsleme konsolun denetim dizilerini (imleç taşıma, satır silme) işleme kipi
// (ENABLE_VIRTUAL_TERMINAL_PROCESSING).
const sanalTerminalIsleme = 0x0004

// denetimDizileriniAc konsolda denetim dizilerini açar; geri, eski kipi geri yükler. Konsol
// desteklemiyorsa (eski konsol) ok yanlıştır.
func denetimDizileriniAc(f *os.File) (geri func(), ok bool) {
	var kip uint32
	if r, _, _ := konsolKipiAl.Call(f.Fd(), uintptr(unsafe.Pointer(&kip))); r == 0 {
		return nil, false
	}
	if kip&sanalTerminalIsleme != 0 {
		return func() {}, true
	}
	if r, _, _ := konsolKipiAyarla.Call(f.Fd(), uintptr(kip|sanalTerminalIsleme)); r == 0 {
		return nil, false
	}
	return func() { konsolKipiAyarla.Call(f.Fd(), uintptr(kip)) }, true
}

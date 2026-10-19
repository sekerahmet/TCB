package profiler

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode = kernel32.NewProc("GetConsoleMode")
	setConsoleMode = kernel32.NewProc("SetConsoleMode")
)

// virtualTerminalProcessing konsolun denetim dizilerini (imleç taşıma, satır silme) işleme kipi
// (ENABLE_VIRTUAL_TERMINAL_PROCESSING).
const virtualTerminalProcessing = 0x0004

// enableControlSeqs konsolda denetim dizilerini açar; geri, eski kipi geri yükler. Konsol
// desteklemiyorsa (eski konsol) ok yanlıştır.
func enableControlSeqs(f *os.File) (restore func(), ok bool) {
	var mode uint32
	if r, _, _ := getConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return nil, false
	}
	if mode&virtualTerminalProcessing != 0 {
		return func() {}, true
	}
	if r, _, _ := setConsoleMode.Call(f.Fd(), uintptr(mode|virtualTerminalProcessing)); r == 0 {
		return nil, false
	}
	return func() { setConsoleMode.Call(f.Fd(), uintptr(mode)) }, true
}

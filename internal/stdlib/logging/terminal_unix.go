//go:build linux || darwin

package logging

import (
	"os"
	"syscall"
	"unsafe"
)

// sistemGenisligi terminalin sütun sayısıdır (pencere boyutu sorgusu); terminal değilse 0.
func sistemGenisligi(f *os.File) int {
	var boyut struct{ satir, sutun, x, y uint16 }
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&boyut)))
	if e != 0 {
		return 0
	}
	return int(boyut.sutun)
}

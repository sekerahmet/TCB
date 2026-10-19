//go:build linux || darwin

package logging

import (
	"os"
	"syscall"
	"unsafe"
)

// systemWidth terminalin sütun sayısıdır (pencere boyutu sorgusu); terminal değilse 0.
func systemWidth(f *os.File) int {
	var size struct{ rows, cols, x, y uint16 }
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
	if e != 0 {
		return 0
	}
	return int(size.cols)
}

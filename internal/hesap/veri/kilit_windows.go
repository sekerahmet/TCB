//go:build windows

package veri

import (
	"os"
	"syscall"
	"unsafe"
)

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

const (
	kilitHemenDon = 0x1 // LOCKFILE_FAIL_IMMEDIATELY
	kilitOzel     = 0x2 // LOCKFILE_EXCLUSIVE_LOCK
)

// dosyaKilitle f üzerinde bekletmeden özel kilit alır; kilit başka bir
// tutamaktaysa hata döner. Kilit, tutamak kapanınca (süreç çökse de)
// bırakılır.
func dosyaKilitle(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := lockFileEx.Call(f.Fd(), kilitOzel|kilitHemenDon, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return os.NewSyscallError("LockFileEx", err)
	}
	return nil
}

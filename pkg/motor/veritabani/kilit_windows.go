//go:build windows

package veritabani

import (
	"os"
	"syscall"
	"unsafe"
)

const kilitDestekli = true

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2
)

// dosyaKilitle dosyanın ilk baytına beklemeden özel kilit koyar; kilit tutamaç kapanınca
// ya da süreç bitince işletim sistemince bırakılır.
func dosyaKilitle(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := lockFileEx.Call(f.Fd(), lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

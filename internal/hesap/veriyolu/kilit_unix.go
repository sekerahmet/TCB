//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package veriyolu

import (
	"os"
	"syscall"
)

// dosyaKilitle f üzerinde bekletmeden özel (yazma) kilidi alır; kilit başka
// bir açık tutamaktaysa hata döner. Kilit, tutamak kapanınca (süreç
// çökse de) bırakılır.
func dosyaKilitle(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return os.NewSyscallError("flock", err)
	}
	return nil
}

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package veritabani

import (
	"os"
	"syscall"
)

const kilitDestekli = true

// dosyaKilitle dosyaya beklemeden özel kilit koyar; kilit dosya kapanınca ya da süreç
// bitince işletim sistemince bırakılır.
func dosyaKilitle(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

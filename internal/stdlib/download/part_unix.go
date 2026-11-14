//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package download

import (
	"errors"
	"os"
	"syscall"
)

// openLocked .part'ı açar ve flock ile kilitler (emsal internal/compute/dataload/lock_unix.go).
// Kilit dosyaya bağlıdır, süreç bitince (çökse de) bırakılır.
func openLocked(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errBusy
		}
		return nil, err
	}
	return f, nil
}

// renameLocked açık ve kilitli dosyanın adını hedefe çevirir.
func renameLocked(_ *os.File, from, to string) error { return os.Rename(from, to) }

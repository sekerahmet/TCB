//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package download

import "os"

// openLocked bu sistemlerde kilitsiz açar (emsal internal/compute/dataload/lock_other.go).
func openLocked(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
}

// renameLocked dosyanın adını hedefe çevirir.
func renameLocked(_ *os.File, from, to string) error { return os.Rename(from, to) }

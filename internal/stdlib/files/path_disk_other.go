//go:build !windows && !linux && !darwin && !freebsd

package files

import "errors"

// freeBytesForUser bu işletim sisteminde okunmaz: okuma hatası verir, 0 değil.
func freeBytesForUser(string) (uint64, error) {
	return 0, errors.ErrUnsupported
}

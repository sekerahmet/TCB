//go:build !windows && !linux && !darwin

package sysmem

// Physical bu işletim sisteminde okunmaz; varsayılan sınır kullanılır.
func Physical() (uint64, bool) { return 0, false }

//go:build !windows && !linux && !darwin

package tensor

// fizikselBellek bu işletim sisteminde okunmaz; varsayılan sınır kullanılır.
func physicalMemory() (uint64, bool) { return 0, false }

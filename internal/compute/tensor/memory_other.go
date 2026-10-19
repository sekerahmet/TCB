//go:build !windows && !linux && !darwin

package tensor

// physicalMemory bu işletim sisteminde okunmaz; varsayılan sınır kullanılır.
func physicalMemory() (uint64, bool) { return 0, false }

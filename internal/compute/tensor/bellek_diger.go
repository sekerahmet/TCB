//go:build !windows && !linux && !darwin

package tensor

// fizikselBellek bu işletim sisteminde okunmaz; varsayılan sınır kullanılır.
func fizikselBellek() (uint64, bool) { return 0, false }

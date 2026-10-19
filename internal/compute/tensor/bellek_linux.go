package tensor

import "syscall"

// fizikselBellek makinenin toplam fiziksel belleğini (bayt) sistem çağrısıyla okur.
func fizikselBellek() (uint64, bool) {
	var b syscall.Sysinfo_t
	if syscall.Sysinfo(&b) != nil {
		return 0, false
	}
	birim := uint64(b.Unit)
	if birim == 0 {
		birim = 1
	}
	return uint64(b.Totalram) * birim, true
}

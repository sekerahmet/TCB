package tensor

import "syscall"

// physicalMemory makinenin toplam fiziksel belleğini (bayt) sistem çağrısıyla okur.
func physicalMemory() (uint64, bool) {
	var b syscall.Sysinfo_t
	if syscall.Sysinfo(&b) != nil {
		return 0, false
	}
	unit := uint64(b.Unit)
	if unit == 0 {
		unit = 1
	}
	return uint64(b.Totalram) * unit, true
}

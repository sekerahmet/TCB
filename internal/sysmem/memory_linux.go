// Package sysmem, makinenin toplam fiziksel belleğini işletim sisteminden okur. Dil çekirdeği
// ve hesap çekirdeği ayırma sınırlarında bu sorguyu ortak kullanır.
//
// Layer: system memory (internal/sysmem); hiçbir iç paketi içe aktarmaz.
package sysmem

import "syscall"

// Physical makinenin toplam fiziksel belleğini (bayt) sistem çağrısıyla okur.
func Physical() (uint64, bool) {
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

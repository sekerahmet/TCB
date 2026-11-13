package sysmem

import (
	"syscall"
	"unsafe"
)

// Available şu an yeni işe verilebilecek fiziksel belleği (bayt) okur:
// GlobalMemoryStatusEx'in ullAvailPhys alanı (memState.freePhysical).
func Available() (uint64, bool) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if proc.Find() != nil {
		return 0, false
	}
	var d memState
	d.length = uint32(unsafe.Sizeof(d))
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&d)))
	if r == 0 {
		return 0, false
	}
	return d.freePhysical, true
}

package sysmem

import (
	"syscall"
	"unsafe"
)

// memState çekirdek kitaplığının bellek durumu yapısıdır (64 bayt).
type memState struct {
	length           uint32
	load             uint32
	totalPhysical    uint64
	freePhysical     uint64
	totalPages       uint64
	freePages        uint64
	totalVirtual     uint64
	freeVirtual      uint64
	freeLargeVirtual uint64
}

// Physical makinenin toplam fiziksel belleğini (bayt) sistem çağrısıyla okur.
func Physical() (uint64, bool) {
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
	return d.totalPhysical, true
}

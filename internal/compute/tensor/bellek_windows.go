package tensor

import (
	"syscall"
	"unsafe"
)

// bellekDurumu çekirdek kitaplığının bellek durumu yapısıdır (64 bayt).
type bellekDurumu struct {
	uzunluk        uint32
	yuk            uint32
	toplamFiziksel uint64
	bosFiziksel    uint64
	toplamSayfa    uint64
	bosSayfa       uint64
	toplamSanal    uint64
	bosSanal       uint64
	bosGenisSanal  uint64
}

// fizikselBellek makinenin toplam fiziksel belleğini (bayt) sistem çağrısıyla okur.
func fizikselBellek() (uint64, bool) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if proc.Find() != nil {
		return 0, false
	}
	var d bellekDurumu
	d.uzunluk = uint32(unsafe.Sizeof(d))
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&d)))
	if r == 0 {
		return 0, false
	}
	return d.toplamFiziksel, true
}

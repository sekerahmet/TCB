package tensor

import (
	"testing"
	"unsafe"
)

var virtualQuery = kernel32.NewProc("VirtualQuery")

// memoryBasicInformation Windows'un MEMORY_BASIC_INFORMATION yapısıdır (64 bit, 48 bayt).
type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	_                 uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
	_                 uint32
}

// Bırakılan blok Windows'ta okunmaz, işletim sistemine sorulur: aralık ayrılmış kalır (MEM_RESERVE,
// erişim kapalı, adres yeni bloğa gitmez) ve bölge aralığın tamamını kaplar (ortada bağlı kalan sayfa
// bölgeyi bölerdi). Okumak Windows'ta istisnanın çerçevesini gorutin yığınına yazar; AMX'li makinede
// çerçeve Go'nun yığın altına ayırdığı yeri aşıp öbeği bozar (golang/go#81238; CI 43251746027'de
// "invalid g status"). Linux'ta okuma testi kalır (host_block_linux_test.go): sinyal ayrı yığında işlenir.
func TestHostBlockReleasedRangeStaysReserved(t *testing.T) {
	stale := releasedSlice(t)
	if n := settleHostMemory(func(n int64) bool { return n == 0 }); n != 0 {
		t.Fatalf("blok bırakılmadı: %d bayt", n)
	}
	base := uintptr(unsafe.Pointer(unsafe.SliceData(stale)))
	var mi memoryBasicInformation
	if r, _, err := virtualQuery.Call(base, uintptr(unsafe.Pointer(&mi)), unsafe.Sizeof(mi)); r == 0 {
		t.Fatalf("VirtualQuery: %v", err)
	}
	if mi.State != memReserve || mi.AllocationBase != base {
		t.Fatalf("bırakılan aralık: durum %#x, taban %#x; beklenen %#x, %#x", mi.State, mi.AllocationBase, memReserve, base)
	}
	if want := uintptr(len(stale) + hostTail); mi.RegionSize < want {
		t.Fatalf("ayrılmış bölge %d bayt, bırakılan aralık %d bayt: aralığın bir kısmı bağlı kaldı", mi.RegionSize, want)
	}
}

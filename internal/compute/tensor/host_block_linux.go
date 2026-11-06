package tensor

import (
	"syscall"
	"unsafe"
)

// hugePage Linux'un saydam büyük sayfasının boyudur (amd64 ve 4 KiB sayfalı arm64).
const hugePage = 2 << 20

// hostMap n baytlık anonim bellek ve onu geri veren işlev verir. Büyük tensör büyük sayfa sınırında
// başlar ve tam sayfaları büyük sayfa ister: Go öbeği istemez, ağırlığı baştan sona okuyan çarpım
// küçük sayfada TLB'yi harcar [hipotez]. Son eksik sayfa küçük kalır, bellek şişmez. İstek yerine
// gelmezse (sistem kapalıysa) küçük sayfada sürer, bitler değişmez.
//
// İşlev önce erişimi kapatır, sonra sayfaları bırakır: tersi sırada araya giren okuma sıfır görürdü.
// Aralık eşli kalır (host_block.go).
func hostMap(n int) (mem []byte, free func(), ok bool) {
	size := n + hostTail
	if n >= hugePage {
		size += hugePage
	}
	all, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return nil, nil, false
	}
	start := 0
	if n >= hugePage {
		start = int(-uintptr(unsafe.Pointer(unsafe.SliceData(all))) & (hugePage - 1))
		_ = syscall.Madvise(all[start:start+n&^(hugePage-1)], syscall.MADV_HUGEPAGE)
	}
	return all[start : start+n : start+n], func() {
		_ = syscall.Mprotect(all, syscall.PROT_NONE)
		_ = syscall.Madvise(all, syscall.MADV_DONTNEED)
	}, true
}

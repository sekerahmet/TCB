//go:build windows

package tensor

import (
	"syscall"
	"unsafe"
)

const (
	memCommit     = 0x1000
	memReserve    = 0x2000
	memDecommit   = 0x4000
	pageReadWrite = 0x04
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	virtualAlloc = kernel32.NewProc("VirtualAlloc")
	virtualFree  = kernel32.NewProc("VirtualFree")
)

// hostMap n baytlık anonim bellek ve onu geri veren işlev verir. İşlev sayfaları bırakır, ayrılmış
// aralık kalır: erişim düşer, adres yeni bloğa gitmez (host_block.go).
func hostMap(n int) (mem []byte, free func(), ok bool) {
	address, _, _ := virtualAlloc.Call(0, uintptr(n+hostTail), memReserve|memCommit, pageReadWrite)
	if address == 0 {
		return nil, nil, false
	}
	// uintptr → işaretçi dönüşümü: adres işletim sisteminin verdiği bellektir, çöp toplayıcının
	// yönettiği bellek değildir.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&address))
	return unsafe.Slice((*byte)(p), n), func() { _, _, _ = virtualFree.Call(address, uintptr(n+hostTail), memDecommit) }, true
}

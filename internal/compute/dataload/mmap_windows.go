//go:build windows

package dataload

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// mmapSupported, bu platformda bellek eşlemenin denenip denenmeyeceğidir.
const mmapSupported = true

// mmapFile, f'nin ilk boyut baytını salt okunur olarak belleğe eşler.
// Dönen kapat işlevi eşlemeyi ve eşleme tutamağını serbest bırakır. Eşleme
// açıldıktan sonra dosya tutamağı kapatılabilir; eşleme geçerli kalır.
func mmapFile(f *os.File, dim int64) ([]byte, func() error, error) {
	if dim <= 0 {
		return nil, nil, errMmapEmpty
	}
	if uint64(dim) > uint64(^uintptr(0)>>1) {
		return nil, nil, errMmapTooLarge
	}
	h, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY,
		uint32(uint64(dim)>>32), uint32(uint64(dim)), nil)
	if err != nil {
		return nil, nil, os.NewSyscallError("CreateFileMapping", err)
	}
	address, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_READ, 0, 0, uintptr(dim))
	if err != nil {
		_ = syscall.CloseHandle(h)
		return nil, nil, os.NewSyscallError("MapViewOfFile", err)
	}
	// uintptr → işaretçi dönüşümü: adres işletim sisteminin verdiği eşleme
	// başlangıcıdır ve çöp toplayıcının yönettiği bellek değildir.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&address))
	b := unsafe.Slice((*byte)(p), int(dim))
	closeFn := func() error {
		e1 := syscall.UnmapViewOfFile(address)
		e2 := syscall.CloseHandle(h)
		if e1 != nil {
			e1 = os.NewSyscallError("UnmapViewOfFile", e1)
		}
		if e2 != nil {
			e2 = os.NewSyscallError("CloseHandle", e2)
		}
		return errors.Join(e1, e2)
	}
	return b, closeFn, nil
}

//go:build windows

package veri

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// eslemeDestekli, bu platformda bellek eşlemenin denenip denenmeyeceğidir.
const eslemeDestekli = true

// dosyaEsle, f'nin ilk boyut baytını salt okunur olarak belleğe eşler.
// Dönen kapat işlevi eşlemeyi ve eşleme tutamağını serbest bırakır. Eşleme
// açıldıktan sonra dosya tutamağı kapatılabilir; eşleme geçerli kalır.
func dosyaEsle(f *os.File, boyut int64) ([]byte, func() error, error) {
	if boyut <= 0 {
		return nil, nil, errEslemeBos
	}
	if uint64(boyut) > uint64(^uintptr(0)>>1) {
		return nil, nil, errEslemeBuyuk
	}
	h, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY,
		uint32(uint64(boyut)>>32), uint32(uint64(boyut)), nil)
	if err != nil {
		return nil, nil, os.NewSyscallError("CreateFileMapping", err)
	}
	adres, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_READ, 0, 0, uintptr(boyut))
	if err != nil {
		_ = syscall.CloseHandle(h)
		return nil, nil, os.NewSyscallError("MapViewOfFile", err)
	}
	// uintptr → işaretçi dönüşümü: adres işletim sisteminin verdiği eşleme
	// başlangıcıdır ve çöp toplayıcının yönettiği bellek değildir.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&adres))
	b := unsafe.Slice((*byte)(p), int(boyut))
	kapat := func() error {
		e1 := syscall.UnmapViewOfFile(adres)
		e2 := syscall.CloseHandle(h)
		if e1 != nil {
			e1 = os.NewSyscallError("UnmapViewOfFile", e1)
		}
		if e2 != nil {
			e2 = os.NewSyscallError("CloseHandle", e2)
		}
		return errors.Join(e1, e2)
	}
	return b, kapat, nil
}

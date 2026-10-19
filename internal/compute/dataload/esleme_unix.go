//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package dataload

import (
	"os"
	"syscall"
)

// eslemeDestekli, bu platformda bellek eşlemenin denenip denenmeyeceğidir.
const eslemeDestekli = true

// dosyaEsle, f'nin ilk boyut baytını salt okunur ve paylaşımlı olarak
// belleğe eşler. Eşleme açıldıktan sonra dosya tutamağı kapatılabilir.
func dosyaEsle(f *os.File, boyut int64) ([]byte, func() error, error) {
	if boyut <= 0 {
		return nil, nil, errEslemeBos
	}
	if uint64(boyut) > uint64(^uint(0)>>1) {
		return nil, nil, errEslemeBuyuk
	}
	b, err := syscall.Mmap(int(f.Fd()), 0, int(boyut), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, nil, os.NewSyscallError("mmap", err)
	}
	kapat := func() error {
		if err := syscall.Munmap(b); err != nil {
			return os.NewSyscallError("munmap", err)
		}
		return nil
	}
	return b, kapat, nil
}

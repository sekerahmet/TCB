//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package dataload

import (
	"os"
	"syscall"
)

// eslemeDestekli, bu platformda bellek eşlemenin denenip denenmeyeceğidir.
const mmapSupported = true

// dosyaEsle, f'nin ilk boyut baytını salt okunur ve paylaşımlı olarak
// belleğe eşler. Eşleme açıldıktan sonra dosya tutamağı kapatılabilir.
func mmapFile(f *os.File, dim int64) ([]byte, func() error, error) {
	if dim <= 0 {
		return nil, nil, errMmapEmpty
	}
	if uint64(dim) > uint64(^uint(0)>>1) {
		return nil, nil, errMmapTooLarge
	}
	b, err := syscall.Mmap(int(f.Fd()), 0, int(dim), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, nil, os.NewSyscallError("mmap", err)
	}
	closeFn := func() error {
		if err := syscall.Munmap(b); err != nil {
			return os.NewSyscallError("munmap", err)
		}
		return nil
	}
	return b, closeFn, nil
}

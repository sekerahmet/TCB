//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package veri

import "os"

// eslemeDestekli, bu platformda bellek eşlemenin denenip denenmeyeceğidir.
const eslemeDestekli = false

// dosyaEsle bu platformda desteklenmez; çağıran ReadAt yoluna döner.
func dosyaEsle(f *os.File, boyut int64) ([]byte, func() error, error) {
	return nil, nil, errEslemeYok
}

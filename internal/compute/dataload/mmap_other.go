//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package dataload

import "os"

// mmapSupported, bu platformda bellek eşlemenin denenip denenmeyeceğidir.
const mmapSupported = false

// mmapFile bu platformda desteklenmez; çağıran ReadAt yoluna döner.
func mmapFile(f *os.File, dim int64) ([]byte, func() error, error) {
	return nil, nil, errNoMmap
}

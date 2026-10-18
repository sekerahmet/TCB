//go:build !linux && !darwin && !windows

package gunluk

import "os"

// sistemGenisligi bu işletim sisteminde sorulmaz; varsayılan genişlik kullanılır.
func sistemGenisligi(*os.File) int { return 0 }

//go:build !linux && !darwin && !windows

package logging

import "os"

// systemWidth bu işletim sisteminde sorulmaz; varsayılan genişlik kullanılır.
func systemWidth(*os.File) int { return 0 }

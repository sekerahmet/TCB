//go:build !unix && !windows

package messages

import "syscall"

// systemErrorReason bu platformda hata kodlarını tanımaz; bilinmeyen durum iletisi kullanılır.
func systemErrorReason(syscall.Errno) Code { return 0 }

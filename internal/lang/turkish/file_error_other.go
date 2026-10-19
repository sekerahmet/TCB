//go:build !unix && !windows

package turkish

import "syscall"

// systemErrorReason bu platformda hata kodlarını tanımaz; bilinmeyen durum mesajı kullanılır.
func systemErrorReason(syscall.Errno) string { return "" }

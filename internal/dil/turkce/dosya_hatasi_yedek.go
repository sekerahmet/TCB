//go:build !unix && !windows

package turkce

import "syscall"

// sistemHatasiNedeni bu platformda hata kodlarını tanımaz; bilinmeyen durum mesajı kullanılır.
func sistemHatasiNedeni(syscall.Errno) string { return "" }

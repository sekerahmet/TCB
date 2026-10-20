//go:build !unix && !windows

package turkish

import (
	"syscall"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// systemErrorReason bu platformda hata kodlarını tanımaz; bilinmeyen durum mesajı kullanılır.
func systemErrorReason(syscall.Errno) messages.Code { return 0 }

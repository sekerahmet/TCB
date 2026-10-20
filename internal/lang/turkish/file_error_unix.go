//go:build unix

package turkish

import (
	"syscall"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// systemErrorReason POSIX hata kodlarından bilinenleri ileti koduyla anlatır; bilinmeyende 0 döner.
func systemErrorReason(code syscall.Errno) messages.Code {
	switch code {
	case syscall.ENOSPC:
		return messages.H2143
	case syscall.EDQUOT:
		return messages.H2154
	case syscall.EROFS:
		return messages.H2155
	case syscall.ENOTDIR:
		return messages.H2156
	case syscall.ENAMETOOLONG:
		return messages.H2145
	case syscall.EMFILE, syscall.ENFILE:
		return messages.H2140
	case syscall.EBUSY, syscall.ETXTBSY:
		return messages.H2142
	case syscall.ENOTEMPTY:
		return messages.H2144
	case syscall.EIO:
		return messages.H2141
	case syscall.ELOOP:
		return messages.H2157
	case syscall.EFBIG:
		return messages.H2158
	}
	return 0
}

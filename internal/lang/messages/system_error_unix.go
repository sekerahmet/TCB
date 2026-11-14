//go:build unix

package messages

import "syscall"

// systemErrorReason POSIX hata kodlarından bilinenleri ileti koduyla anlatır; bilinmeyende 0 döner.
func systemErrorReason(code syscall.Errno) Code {
	switch code {
	case syscall.ENOSPC:
		return H2143
	case syscall.EDQUOT:
		return H2154
	case syscall.EROFS:
		return H2155
	case syscall.ENOTDIR:
		return H2156
	case syscall.ENAMETOOLONG:
		return H2145
	case syscall.EMFILE, syscall.ENFILE:
		return H2140
	case syscall.EBUSY, syscall.ETXTBSY:
		return H2142
	case syscall.ENOTEMPTY:
		return H2144
	case syscall.EIO:
		return H2141
	case syscall.ELOOP:
		return H2157
	case syscall.EFBIG:
		return H2158
	}
	return 0
}

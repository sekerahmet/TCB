//go:build windows

package turkish

import (
	"syscall"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// systemErrorReason Windows hata kodlarından bilinenleri ileti koduyla anlatır; bilinmeyende 0 döner.
func systemErrorReason(code syscall.Errno) messages.Code {
	switch code {
	case 4: // ERROR_TOO_MANY_OPEN_FILES
		return messages.H2140
	case 15: // ERROR_INVALID_DRIVE
		return messages.H2146
	case 19: // ERROR_WRITE_PROTECT
		return messages.H2147
	case 21: // ERROR_NOT_READY
		return messages.H2148
	case 23, 1117: // ERROR_CRC, ERROR_IO_DEVICE
		return messages.H2141
	case 32: // ERROR_SHARING_VIOLATION
		return messages.H2142
	case 33: // ERROR_LOCK_VIOLATION
		return messages.H2149
	case 39, 112: // ERROR_HANDLE_DISK_FULL, ERROR_DISK_FULL
		return messages.H2143
	case 53, 67: // ERROR_BAD_NETPATH, ERROR_BAD_NET_NAME
		return messages.H2150
	case 123: // ERROR_INVALID_NAME
		return messages.H2151
	case 145: // ERROR_DIR_NOT_EMPTY
		return messages.H2144
	case 206: // ERROR_FILENAME_EXCED_RANGE
		return messages.H2145
	case 267: // ERROR_DIRECTORY
		return messages.H2152
	case 1392: // ERROR_FILE_CORRUPT
		return messages.H2153
	}
	return 0
}

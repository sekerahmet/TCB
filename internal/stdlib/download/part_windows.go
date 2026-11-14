//go:build windows

package download

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx              = kernel32.NewProc("LockFileEx")
	procSetFileInformationByHnd = kernel32.NewProc("SetFileInformationByHandle")
)

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2
	fileShareDelete         = 0x4
	deleteAccess            = 0x10000
	fileRenameInfoEx        = 22 // FILE_INFO_BY_HANDLE_CLASS
	renameReplaceIfExists   = 0x1
	renamePosixSemantics    = 0x2
	errorLockViolation      = syscall.Errno(33)
	errorInvalidParameter   = syscall.Errno(87)
	errorNotSupported       = syscall.Errno(50)
)

// openLocked .part'ı Go'nun açışından ayrı açar: Go FILE_SHARE_DELETE vermez ve öyle açık dosyanın
// adı değiştirilemez (K7 1. adım). Kilit dosya sonunun ötesindeki tek bayttadır: bayt kilidi Windows'ta
// zorunludur, 0. bayttaki kilit dosyayı okuyanları da durdururdu (K7 4. adım; defter 8).
func openLocked(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE|deleteAccess,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|fileShareDelete, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	ol := syscall.Overlapped{Offset: 0xFFFFFFFF, OffsetHigh: 0xFFFFFFFF}
	r, _, lockErr := procLockFileEx.Call(uintptr(h), lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		syscall.CloseHandle(h)
		if errors.Is(lockErr, errorLockViolation) {
			return nil, errBusy
		}
		return nil, lockErr
	}
	return os.NewFile(uintptr(h), path), nil
}

// fileRenameInfo Windows'un FILE_RENAME_INFO yapısıdır (Flags birleşimin DWORD yüzü).
type fileRenameInfo struct {
	Flags          uint32
	RootDirectory  syscall.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// renameLocked açık ve kilitli dosyanın adını tutamakla, POSIX anlamıyla ve hedefi ezerek değiştirir
// (K7 3. adım). Dosya sistemi bu sınıfı desteklemezse os.Rename (MoveFileEx), dosya FILE_SHARE_DELETE
// ile açıkken (K7 2. adım).
func renameLocked(f *os.File, from, to string) error {
	name, err := syscall.UTF16FromString(to)
	if err != nil {
		return err
	}
	n := len(name) - 1
	off := unsafe.Offsetof(fileRenameInfo{}.FileName)
	buf := make([]byte, int(off)+(n+1)*2)
	info := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	info.Flags = renameReplaceIfExists | renamePosixSemantics
	info.FileNameLength = uint32(n * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[off])), n), name[:n])
	r, _, callErr := procSetFileInformationByHnd.Call(f.Fd(), fileRenameInfoEx, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r != 0 {
		return nil
	}
	if errors.Is(callErr, errorInvalidParameter) || errors.Is(callErr, errorNotSupported) {
		return os.Rename(from, to)
	}
	return callErr
}

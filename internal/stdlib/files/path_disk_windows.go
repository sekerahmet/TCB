package files

import (
	"syscall"
	"unsafe"
)

// freeBytesForUser GetDiskFreeSpaceExW'nin lpFreeBytesAvailableToCaller'ıdır: kullanıcının
// kotası düşülmüş boş yer. dir bir klasördür.
func freeBytesForUser(dir string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	if err := proc.Find(); err != nil {
		return 0, err
	}
	var free, total, totalFree uint64
	r, _, callErr := proc.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)),
		uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree)))
	if r == 0 {
		return 0, callErr
	}
	return free, nil
}

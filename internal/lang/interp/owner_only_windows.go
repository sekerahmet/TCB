//go:build windows

package interp

import (
	"syscall"
	"unsafe"
)

// Windows'ta yalnız kullanıcıya açık dosya (18.13): erişim listesinde yalnız kullanıcı ve
// Administrators tam yetkilidir, üst klasörden miras kapalıdır (SDDL'de "P"). golang.org/x/sys
// kullanılmaz; çağrılar advapi32'ye doğrudandır.

const (
	sddlRevision1                    = 1
	seFileObject                     = 1
	daclSecurityInformation          = 0x00000004
	protectedDaclSecurityInformation = 0x80000000
)

var (
	advapi32                      = syscall.NewLazyDLL("advapi32.dll")
	procConvertStringSDToSD       = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procGetSecurityDescriptorDacl = advapi32.NewProc("GetSecurityDescriptorDacl")
	procSetNamedSecurityInfo      = advapi32.NewProc("SetNamedSecurityInfoW")
)

// restrictToOwner dosyanın erişim listesini kullanıcıya ve Administrators'a daraltır.
func restrictToOwner(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	sddl, err := syscall.UTF16PtrFromString("D:P(A;;FA;;;" + sid + ")(A;;FA;;;BA)")
	if err != nil {
		return err
	}
	var sd uintptr
	if r, _, e := procConvertStringSDToSD.Call(uintptr(unsafe.Pointer(sddl)), sddlRevision1, uintptr(unsafe.Pointer(&sd)), 0); r == 0 {
		return e
	}
	defer syscall.LocalFree(syscall.Handle(sd))
	var present, defaulted int32
	var dacl uintptr
	if r, _, e := procGetSecurityDescriptorDacl.Call(sd, uintptr(unsafe.Pointer(&present)), uintptr(unsafe.Pointer(&dacl)), uintptr(unsafe.Pointer(&defaulted))); r == 0 {
		return e
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r, _, _ := procSetNamedSecurityInfo.Call(uintptr(unsafe.Pointer(name)), seFileObject,
		daclSecurityInformation|protectedDaclSecurityInformation, 0, 0, dacl, 0)
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

// currentUserSID sürecin kullanıcısının SID'idir (S-1-5-21-...).
func currentUserSID() (string, error) {
	tok, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer tok.Close()
	u, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String()
}

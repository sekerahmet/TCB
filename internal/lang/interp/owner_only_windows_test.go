//go:build windows

package interp

import (
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

var (
	procGetNamedSecurityInfo = advapi32.NewProc("GetNamedSecurityInfoW")
	procSDToStringSD         = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
)

// checkOwnerOnly dosyanın erişim listesinin yalnız kullanıcıyı ve Administrators'ı taşıdığını,
// mirasın kapalı olduğunu SDDL metninden okur (18.13).
func checkOwnerOnly(t *testing.T, path string) {
	t.Helper()
	name, _ := syscall.UTF16PtrFromString(path)
	var sd uintptr
	if r, _, _ := procGetNamedSecurityInfo.Call(uintptr(unsafe.Pointer(name)), seFileObject, daclSecurityInformation,
		0, 0, 0, 0, uintptr(unsafe.Pointer(&sd))); r != 0 {
		t.Fatalf("erişim listesi okunamadı: %v", syscall.Errno(r))
	}
	defer syscall.LocalFree(syscall.Handle(sd))
	var text *uint16
	if r, _, e := procSDToStringSD.Call(sd, sddlRevision1, daclSecurityInformation, uintptr(unsafe.Pointer(&text)), 0); r == 0 {
		t.Fatalf("SDDL'e çevrilemedi: %v", e)
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(text)))
	got := syscall.UTF16ToString(unsafe.Slice(text, 512))
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	// Bayraklar "(" öncesindedir: P (miras kapalı) olmalı; AI (kendiliğinden miras bilgisi) zararsız.
	// Girdilerde ID (miras alınmış) yok: liste tam bu iki girdidir.
	flags, aces, _ := strings.Cut(strings.TrimPrefix(got, "D:"), "(")
	if !strings.Contains(flags, "P") || "("+aces != "(A;;FA;;;"+sid+")(A;;FA;;;BA)" {
		t.Fatalf("erişim listesi %s; miras kapalı, yalnız kullanıcı ve Administrators bekleniyordu", got)
	}
}

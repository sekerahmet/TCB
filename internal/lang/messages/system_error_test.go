package messages

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Kural (8.5): SystemError işletim sisteminin ve Go'nun ham hata metnini değere sokmaz; bilinen
// durumları H2130-H2158'e, kendi koduna inmeyen io.ErrUnexpectedEOF'u H2153'e, bilinmeyen errno'yu
// H2628'e, öbür kodsuz hatayı H2629'a indirir ve özgün hatayı Unwrap ile korur.
func TestSystemErrorCarriesNoSystemText(t *testing.T) {
	if SystemError("x", nil) != nil {
		t.Fatal("nil hata nil kalmalı")
	}
	dir := t.TempDir()
	if got := SystemError(dir, &fs.PathError{Op: "read", Path: dir, Err: fs.ErrInvalid}); !Has(got, H2130) {
		t.Errorf("klasör yolu H2130 olmalı: %v", got)
	}
	missingNested := filepath.Join(dir, "yok_ust", "alt", "veri.npy")
	if got := SystemError(missingNested, &fs.PathError{Op: "open", Path: missingNested, Err: fs.ErrNotExist}); !Has(got, H2131) {
		t.Errorf("eksik üst klasör H2131 olmalı: %v", got)
	}
	for _, n := range []syscall.Errno{1168, 71} {
		raw := &fs.PathError{Op: "open", Path: "x", Err: n}
		got := SystemError("x", raw)
		want := fmt.Sprintf("işletim sistemi beklenmeyen bir hata bildirdi (hata kodu %d)", n)
		if !Has(got, H2628) || got.Error() != want || strings.Contains(got.Error(), n.Error()) {
			t.Errorf("%d: sistemin metni ya da beklenmeyen ileti: %v", n, got)
		}
		if !errors.Is(got, n) {
			t.Errorf("%d: özgün errno Unwrap ile korunmadı", n)
		}
	}
	for _, eofErr := range []error{io.ErrUnexpectedEOF, &fs.PathError{Op: "read", Path: "x", Err: io.ErrUnexpectedEOF}} {
		got := SystemError("x", eofErr)
		if !Has(got, H2153) || strings.Contains(got.Error(), "EOF") {
			t.Errorf("io.ErrUnexpectedEOF H2153 olmalı ve ham metin sızmamalı: %v", got)
		}
		if !errors.Is(got, io.ErrUnexpectedEOF) {
			t.Errorf("io.ErrUnexpectedEOF Unwrap ile korunmalı: %v", got)
		}
	}
	plain := errors.New("raw English driver error")
	gotPlain := SystemError("", plain)
	if !Has(gotPlain, H2629) || strings.Contains(gotPlain.Error(), "raw English") || !errors.Is(gotPlain, plain) {
		t.Errorf("kodsuz Go hatası H2629 olmalı ve Unwrap korumalı: %v", gotPlain)
	}
	coded := New(H7004, "bozuk")
	if same := SystemError("x", coded); same != coded {
		t.Errorf("sistem hatası taşımayan kodlu ileti olduğu gibi kalmalı: %v", same)
	}
}

// Kural: *os.LinkError ve *os.SyscallError da HasSystemError ve SystemError tarafından tanınır;
// Go hata zincirinde errors.As ve errors.Is çalışmaya devam eder.
func TestSystemErrorUnwrapsLinkAndSyscall(t *testing.T) {
	dir := t.TempDir()
	linkErr := &os.LinkError{
		Op:  "rename",
		Old: filepath.Join(dir, "gecici.tmp"),
		New: filepath.Join(dir, "eksik_klasor", "hedef.bin"),
		Err: fs.ErrNotExist,
	}
	if !HasSystemError(linkErr) {
		t.Fatal("HasSystemError *os.LinkError için doğru dönmeli")
	}
	wrappedLink := New(H5009, "hedef.bin", SystemError("", linkErr))
	if !Has(wrappedLink, H2131) || strings.Contains(wrappedLink.Error(), "rename") {
		t.Errorf("LinkError H2131'e inmeli ve ham op sızmamalı: %v", wrappedLink)
	}
	var gotLink *os.LinkError
	if !errors.As(wrappedLink, &gotLink) || !errors.Is(wrappedLink, fs.ErrNotExist) {
		t.Fatal("LinkError Unwrap zincirinde korunmalı")
	}

	sysErr := os.NewSyscallError("CreatePipe", fs.ErrPermission)
	if !HasSystemError(sysErr) {
		t.Fatal("HasSystemError *os.SyscallError için doğru dönmeli")
	}
	wrappedSys := New(H7501, "tr_calistir()", SystemError("", sysErr))
	if !Has(wrappedSys, H2133) || strings.Contains(wrappedSys.Error(), "CreatePipe") {
		t.Errorf("SyscallError H2133'e inmeli ve sistem metni sızmamalı: %v", wrappedSys)
	}
	var gotSys *os.SyscallError
	if !errors.As(wrappedSys, &gotSys) || !errors.Is(wrappedSys, fs.ErrPermission) {
		t.Fatal("SyscallError Unwrap zincirinde korunmalı")
	}
}

// Kural (8.2, 8.3, 8.5): SystemError iletinin yakalandığı dilde çevrilir (Localized + translatesValue)
// ve neden yuvalarından (error, reason) Root içteki neden koduna inerken neden olmayan yuvalarda
// (ör. {detail}) dıştaki kod kalır.
func TestSystemErrorLocalizedAndRootCode(t *testing.T) {
	useLanguage(t, "tr")
	raw := &fs.PathError{Op: "read", Path: "a.npy", Err: fs.ErrPermission}
	cause := New(H5024, SystemError("a.npy", raw))
	if tr := ErrorIn(cause, "tr"); !strings.Contains(tr, "erişim izni yok") {
		t.Errorf("tr metni: %q", tr)
	}
	if en := ErrorIn(cause, "en"); !strings.Contains(en, "permission denied") {
		t.Errorf("en metni: %q", en)
	}
	if root, ok := Root(cause); !ok || root.Code != H2133 {
		t.Errorf("neden yuvasında Root = %v (%v), H2133 bekleniyordu", root, ok)
	}
	if root, ok := Root(New(H5089, "a.npy", SystemError("a.npy", io.ErrUnexpectedEOF))); !ok || root.Code != H2153 {
		t.Errorf("io.ErrUnexpectedEOF için Root = %v (%v), H2153 bekleniyordu", root, ok)
	}
	nonCause := New(H5620, "tok.json", New(H5272, SystemError("tok.json", raw)))
	if root, ok := Root(nonCause); !ok || root.Code != H5620 {
		t.Errorf("neden olmayan {detail} yuvasında Root = %v (%v), H5620 bekleniyordu", root, ok)
	}
	useLanguage(t, "en")
	withoutTranslation(t, "en", H2133)
	if got := cause.Error(); !strings.Contains(got, "erişim izni yok") {
		t.Errorf("iç neden çevrilmemişken çerçeve kaynak dile düşmeli: %q", got)
	}
}

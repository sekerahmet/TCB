package gpu

import (
	"syscall"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Windows: nvcuda.dll standart syscall paketiyle yüklenir. DLL yoksa (NVIDIA sürücüsü
// kurulu değil) GPU arka ucu kaydolmaz ve her şey CPU'da çalışır.

type windowsLibrary struct{ h syscall.Handle }

// fromSystemDir LoadLibraryExW'nin LOAD_LIBRARY_SEARCH_SYSTEM32 bayrağı: DLL yalnızca
// sistem klasöründen (sürücünün kurulduğu yer) yüklenir; çalışma klasöründeki aynı adlı
// bir dosya yüklenmez.
const fromSystemDir = 0x00000800

func openLib() (library, error) {
	loader := syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")
	if err := loader.Find(); err != nil {
		return nil, messages.New(messages.H4000, err)
	}
	ad, err := syscall.UTF16PtrFromString("nvcuda.dll")
	if err != nil {
		return nil, err
	}
	h, _, e := loader.Call(uintptr(unsafe.Pointer(ad)), 0, fromSystemDir)
	if h == 0 {
		return nil, messages.New(messages.H4001, "nvcuda.dll", e)
	}
	return &windowsLibrary{h: syscall.Handle(h)}, nil
}

func (w *windowsLibrary) find(ad string) (uintptr, error) { return syscall.GetProcAddress(w.h, ad) }

func (w *windowsLibrary) call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(fn, args...)
	return r
}

// runExitHandlers Windows'ta gerekmez: araçlar süreç sonunu kendileri yakalar.
func runExitHandlers() {}

// Bağımlılıklar yalnızca DLL'in kendi klasöründen ve sistem klasöründen yüklenir.
const fromDLLDirAndSystem = 0x00000100 | fromSystemDir

// openLibrary bir DLL'i tam yolundan yükler (arama yolu kullanılmaz).
func openLibrary(path string) (library, error) {
	loader := syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")
	if err := loader.Find(); err != nil {
		return nil, err
	}
	ad, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, _, e := loader.Call(uintptr(unsafe.Pointer(ad)), 0, fromDLLDirAndSystem)
	if h == 0 {
		return nil, e
	}
	return &windowsLibrary{h: syscall.Handle(h)}, nil
}

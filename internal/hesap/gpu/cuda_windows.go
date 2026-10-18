package gpu

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Windows: nvcuda.dll standart syscall paketiyle yüklenir. DLL yoksa (NVIDIA sürücüsü
// kurulu değil) GPU arka ucu kaydolmaz ve her şey CPU'da çalışır.

type windowsKutuphane struct{ h syscall.Handle }

// sistemKlasorunden LoadLibraryExW'nin LOAD_LIBRARY_SEARCH_SYSTEM32 bayrağı: DLL yalnızca
// sistem klasöründen (sürücünün kurulduğu yer) yüklenir; çalışma klasöründeki aynı adlı
// bir dosya yüklenmez.
const sistemKlasorunden = 0x00000800

func kutuphaneAc() (kutuphane, error) {
	yukleyici := syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")
	if err := yukleyici.Find(); err != nil {
		return nil, fmt.Errorf("LoadLibraryExW bulunamadı: %v", err)
	}
	ad, err := syscall.UTF16PtrFromString("nvcuda.dll")
	if err != nil {
		return nil, err
	}
	h, _, e := yukleyici.Call(uintptr(unsafe.Pointer(ad)), 0, sistemKlasorunden)
	if h == 0 {
		return nil, fmt.Errorf("NVIDIA sürücüsü (nvcuda.dll) yüklenemedi: %v", e)
	}
	return &windowsKutuphane{h: syscall.Handle(h)}, nil
}

func (w *windowsKutuphane) bul(ad string) (uintptr, error) { return syscall.GetProcAddress(w.h, ad) }

func (w *windowsKutuphane) cagir(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(fn, args...)
	return r
}

// cikisIsleyicileriniCalistir Windows'ta gerekmez: araçlar süreç sonunu kendileri yakalar.
func cikisIsleyicileriniCalistir() {}

// Bağımlılıklar yalnızca DLL'in kendi klasöründen ve sistem klasöründen yüklenir.
const dllKlasorundenVeSistemden = 0x00000100 | sistemKlasorunden

// kitaplikAc bir DLL'i tam yolundan yükler (arama yolu kullanılmaz).
func kitaplikAc(yol string) (kutuphane, error) {
	yukleyici := syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")
	if err := yukleyici.Find(); err != nil {
		return nil, err
	}
	ad, err := syscall.UTF16PtrFromString(yol)
	if err != nil {
		return nil, err
	}
	h, _, e := yukleyici.Call(uintptr(unsafe.Pointer(ad)), 0, dllKlasorundenVeSistemden)
	if h == 0 {
		return nil, e
	}
	return &windowsKutuphane{h: syscall.Handle(h)}, nil
}

//go:build linux && !gpusuz

package gpu

import (
	"fmt"
	"os"

	"github.com/ebitengine/purego"
)

// Linux: libcuda.so.1 cgo'suz dinamik yükleyiciyle (go.mod'daki tek dış bağımlılık) yüklenir. Kitaplık yoksa (NVIDIA sürücüsü
// kurulu değil) GPU arka ucu kaydolmaz ve her şey CPU'da çalışır.

type linuxLibrary struct{ handle uintptr }

func openLib() (library, error) {
	var last error
	for _, ad := range driverLibCandidates(os.Getenv("LD_LIBRARY_PATH")) {
		h, err := purego.Dlopen(ad, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return &linuxLibrary{handle: h}, nil
		}
		last = err
	}
	return nil, fmt.Errorf("NVIDIA sürücüsü (libcuda.so.1) yüklenemedi: %v", last)
}

func (l *linuxLibrary) find(ad string) (uintptr, error) { return purego.Dlsym(l.handle, ad) }

func (l *linuxLibrary) call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

// runExitHandlers C kitaplığının çıkış işleyicilerini (atexit ile kaydedilenler)
// çalıştırır: __cxa_finalize(NULL). Yalnızca program sonunda, dış bir profilleyici bağlıyken
// çağrılır (bkz. Kapat).
func runExitHandlers() {
	h, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	if fn, err := purego.Dlsym(h, "__cxa_finalize"); err == nil {
		purego.SyscallN(fn, 0)
	}
}

// openLibrary bir kitaplığı (tam yol ya da standart arama yolundaki dosya adı) yükler.
func openLibrary(path string) (library, error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	return &linuxLibrary{handle: h}, nil
}

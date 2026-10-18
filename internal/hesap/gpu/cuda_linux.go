//go:build linux && !gpusuz

package gpu

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// Linux: libcuda.so.1 cgo'suz dinamik yükleyiciyle (go.mod'daki tek dış bağımlılık) yüklenir. Kitaplık yoksa (NVIDIA sürücüsü
// kurulu değil) GPU arka ucu kaydolmaz ve her şey CPU'da çalışır.

type linuxKutuphane struct{ tutamac uintptr }

func kutuphaneAc() (kutuphane, error) {
	var son error
	for _, ad := range []string{"libcuda.so.1", "libcuda.so"} {
		h, err := purego.Dlopen(ad, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return &linuxKutuphane{tutamac: h}, nil
		}
		son = err
	}
	return nil, fmt.Errorf("NVIDIA sürücüsü (libcuda.so.1) yüklenemedi: %v", son)
}

func (l *linuxKutuphane) bul(ad string) (uintptr, error) { return purego.Dlsym(l.tutamac, ad) }

func (l *linuxKutuphane) cagir(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

// cikisIsleyicileriniCalistir C kitaplığının çıkış işleyicilerini (atexit ile kaydedilenler)
// çalıştırır: __cxa_finalize(NULL). Yalnızca program sonunda, dış bir profilleyici bağlıyken
// çağrılır (bkz. araclariBosalt).
func cikisIsleyicileriniCalistir() {
	h, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	if fn, err := purego.Dlsym(h, "__cxa_finalize"); err == nil {
		purego.SyscallN(fn, 0)
	}
}

// kitaplikAc bir kitaplığı (tam yol ya da standart arama yolundaki dosya adı) yükler.
func kitaplikAc(yol string) (kutuphane, error) {
	h, err := purego.Dlopen(yol, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	return &linuxKutuphane{tutamac: h}, nil
}

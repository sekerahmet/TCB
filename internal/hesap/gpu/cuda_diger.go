//go:build (!linux && !windows) || (linux && gpusuz)

package gpu

import "errors"

// Diğer işletim sistemlerinde NVIDIA sürücüsü yoktur; GPU arka ucu kaydolmaz. Linux'ta
// "gpusuz" yapı etiketiyle derlenen ikili de buradadır: cgo'suz dinamik yükleyici (dış bağımlılık)
// bağlanmaz, ikili statik kalır (glibc gerekmez) ve GPU kullanılamaz.
func kutuphaneAc() (kutuphane, error) {
	return nil, errors.New("bu TR derlemesinde GPU desteği yok (işletim sistemi desteklenmiyor ya da ikili GPU'suz derlendi)")
}

//go:build (!linux && !windows) || (linux && gpusuz)

package gpu

import "errors"

// Diğer işletim sistemlerinde NVIDIA sürücüsü yoktur; GPU arka ucu kaydolmaz. Linux'ta
// "gpusuz" yapı etiketiyle derlenen ikili de buradadır: cgo'suz dinamik yükleyici (dış bağımlılık)
// bağlanmaz, ikili statik kalır (glibc gerekmez) ve GPU kullanılamaz.
func openLib() (library, error) {
	return nil, errors.New("bu TR derlemesinde GPU desteği yok (işletim sistemi desteklenmiyor ya da ikili GPU'suz derlendi)")
}

func runExitHandlers() {}

func openLibrary(string) (library, error) {
	return nil, errors.New("bu derlemede dış kitaplık yüklenmez")
}

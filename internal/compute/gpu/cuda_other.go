//go:build (!linux && !windows) || (linux && gpusuz)

package gpu

import "github.com/sekerahmet/TCB/internal/lang/messages"

// Diğer işletim sistemlerinde NVIDIA sürücüsü yoktur; GPU arka ucu kaydolmaz. Linux'ta
// "gpusuz" yapı etiketiyle derlenen ikili de buradadır: cgo'suz dinamik yükleyici (dış bağımlılık)
// bağlanmaz, ikili statik kalır (glibc gerekmez) ve GPU kullanılamaz.
func openLib() (library, error) {
	return nil, messages.New(messages.H4002)
}

func runExitHandlers() {}

func openLibrary(string) (library, error) {
	return nil, messages.New(messages.H4003)
}

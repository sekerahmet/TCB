//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package veritabani

import "os"

// Bu platformlarda süreçler arası dosya kilidi yoktur; süreç içinde dosya başına tek nesne
// kuralı yine geçerlidir (KULLANIM_KILAVUZU.md §7).
const kilitDestekli = false

func dosyaKilitle(*os.File) error { return nil }

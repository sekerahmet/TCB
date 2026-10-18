//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package veriyolu

import "os"

// dosyaKilitle bu platformda süreçler arası kilit desteklemez; süreç içi
// paylaşım yine tek yöneticiyle korunur.
func dosyaKilitle(f *os.File) error { return nil }

//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package dataload

import "os"

// dosyaKilitle bu platformda süreçler arası kilit desteklemez; süreç içi
// paylaşım yine tek yöneticiyle korunur.
func lockFile(f *os.File) error { return nil }

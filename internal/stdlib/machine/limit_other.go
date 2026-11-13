//go:build !linux && !windows

package machine

// memoryLimit bu işletim sisteminde okunmaz (macOS dahil; 19.12): üyesi H7735 verir.
func memoryLimit() (bytes int64, limited, ok bool) { return 0, false, false }

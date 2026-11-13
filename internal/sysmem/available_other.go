//go:build !windows && !linux

package sysmem

// Available bu işletim sisteminde okunmaz (macOS dahil; 19.12 bunu kodlu hatayla söyler).
func Available() (uint64, bool) { return 0, false }

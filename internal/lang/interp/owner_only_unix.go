//go:build !windows

package interp

import "os"

// restrictToOwner dosyayı yalnız kullanıcıya açar (18.13): 0600.
func restrictToOwner(path string) error { return os.Chmod(path, 0o600) }

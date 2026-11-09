//go:build !windows

package interp

import (
	"os"
	"testing"
)

// checkOwnerOnly dosyanın yalnız kullanıcıya açık olduğunu (0600) okur (18.13).
func checkOwnerOnly(t *testing.T, path string) {
	t.Helper()
	b, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if m := b.Mode().Perm(); m != 0o600 {
		t.Fatalf("izinler %v, beklenen 0600", m)
	}
}

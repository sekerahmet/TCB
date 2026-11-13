package machine

import (
	"runtime"
	"testing"
)

// TestMachineOSTable kapalı tablonun çevirisi (19.12): darwin → macos, amd64 → x86_64; öbür Go
// adları olduğu gibi ve tablodaki hiçbir değere eşit değil.
func TestMachineOSTable(t *testing.T) {
	for goos, want := range map[string]string{"windows": "windows", "linux": "linux", "darwin": "macos",
		"freebsd": "freebsd", "android": "android", "ios": "ios"} {
		if got := osName(goos); got != want {
			t.Errorf("osName(%q) = %q, beklenen %q", goos, got, want)
		}
	}
	for goarch, want := range map[string]string{"amd64": "x86_64", "arm64": "arm64", "riscv64": "riscv64", "386": "386"} {
		if got := archName(goarch); got != want {
			t.Errorf("archName(%q) = %q, beklenen %q", goarch, got, want)
		}
	}
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		if o := osName(runtime.GOOS); o != "windows" && o != "linux" && o != "macos" {
			t.Fatalf("bu makinenin işletim sistemi tabloda değil: %q", o)
		}
	}
}

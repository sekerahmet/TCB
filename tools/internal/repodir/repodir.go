// Package repodir üretici araçlara (tools/gen/...) çalışma klasöründen yukarı doğru go.mod'un
// bulunduğu ilk klasörü, yani depo kökünü verir (go generate paket klasörlerinden çalıştırır).
//
// Layer: tools (tools); kullanıcıya gitmez.
package repodir

import (
	"os"
	"path/filepath"
)

// Find çalışma klasöründen yukarı doğru go.mod'un bulunduğu ilk klasörü verir; bulunamazsa ya da
// çalışma klasörü okunamazsa ".".
func Find() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "."
		}
		d = parent
	}
}

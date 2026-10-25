// Package reporoot, testlerin depo kökünü bulmasına yarayan küçük bir yardımcıdır. Kök, çalışma
// klasöründen yukarı doğru go.mod dosyasının bulunduğu ilk klasördür. Testler depo içindeki
// dosyalara (belgeler, tests/, examples/, test verileri) "../../" gibi göreli yollarla değil
// bu yardımcıyla ulaşır; böylece bir paket başka bir klasöre taşındığında testler kırılmaz.
//
// Layer: test helper (internal/reporoot); hiçbir iç paketi içe aktarmaz, her katmanın testi
// kullanabilir.
package reporoot

import (
	"os"
	"path/filepath"
	"sync"
)

var (
	rootOnce sync.Once
	root     string
)

// Root depo kökünün mutlak yoludur. go.mod bulunamazsa panikler (yalnızca testlerde
// kullanıldığı için bu, testin anlaşılır bir iletiyle düşmesi demektir).
func Root() string {
	rootOnce.Do(func() {
		var ok bool
		if root, ok = findRoot(); !ok {
			panic("reporoot: no go.mod found above the working directory")
		}
	})
	return root
}

// findRoot çalışma klasöründen yukarı doğru go.mod'un bulunduğu ilk klasördür (bulunamazsa ok false).
func findRoot() (string, bool) {
	d, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

// Path depo köküne göre verilen yolu (bölü çizgisiyle yazılmış parçalar) mutlak yola çevirir:
// reporoot.Path("tests", "data") ya da reporoot.Path("docs/spec").
func Path(parts ...string) string {
	full := []string{Root()}
	for _, p := range parts {
		full = append(full, filepath.FromSlash(p))
	}
	return filepath.Join(full...)
}

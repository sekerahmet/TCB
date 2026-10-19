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
	kokBir sync.Once
	kok    string
)

// Kok depo kökünün mutlak yoludur. go.mod bulunamazsa panikler (yalnızca testlerde
// kullanıldığı için bu, testin anlaşılır bir iletiyle düşmesi demektir).
func Kok() string {
	kokBir.Do(func() {
		d, err := os.Getwd()
		if err != nil {
			panic("depo: çalışma klasörü okunamadı: " + err.Error())
		}
		for {
			if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
				kok = d
				return
			}
			ust := filepath.Dir(d)
			if ust == d {
				panic("depo: çalışma klasöründen yukarı doğru go.mod bulunamadı")
			}
			d = ust
		}
	})
	return kok
}

// Yol depo köküne göre verilen yolu (bölü çizgisiyle yazılmış parçalar) mutlak yola çevirir:
// depo.Yol("tests", "data") ya da depo.Yol("docs/spec").
func Yol(parcalar ...string) string {
	tam := []string{Kok()}
	for _, p := range parcalar {
		tam = append(tam, filepath.FromSlash(p))
	}
	return filepath.Join(tam...)
}

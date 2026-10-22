// Package repodir araçlara (tools/gen/..., tools/codegraph) depo kökünü verir: kök, go.mod'u TR
// modülünü söyleyen klasördür; bulunamazsa git deposunun kökü; o da yoksa açık bir hata.
//
// Layer: tools (tools); kullanıcıya gitmez.
package repodir

import "os"

// Find çalışma klasöründen yukarı doğru depo kökünü verir (go generate paket klasörlerinden
// çalıştırır); bulunamazsa ya da çalışma klasörü okunamazsa ".".
func Find() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	root, err := FindFrom(d)
	if err != nil {
		return "."
	}
	return root
}

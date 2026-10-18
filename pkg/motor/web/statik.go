package web

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Statik klasör sunumu (sunucu.statik_klasör). Kurallar:
//   - Yalnızca klasörün içindeki düzenli dosyalar sunulur. Bağlantı (sembolik bağ, Windows
//     dizin bağlantısı) ile klasörün dışına çıkan yollar 404 verir.
//   - Adı nokta ile başlayan dosya ve klasörler (.env, .git ...) hiç sunulmaz.
//   - Dizin listelenmez: bir klasör istenirse içindeki index.html sunulur, yoksa 404.
//   - Ters bölü, iki nokta (Windows alternatif veri akışı: a.txt::$DATA) ve NUL içeren yollar
//     reddedilir.

// statikKlasorHazirla URL önekini ("/dosyalar" → "/dosyalar/") ve klasörü (TR'nin tek yol
// kuralıyla) hazırlar; klasör yoksa ya da klasör değilse hata verir.
func statikKlasorHazirla(v *vm.VM, onek, klasor string) (string, string, error) {
	if !strings.HasPrefix(onek, "/") {
		onek = "/" + onek
	}
	if !strings.HasSuffix(onek, "/") {
		onek += "/"
	}
	yol, _ := v.YolCoz(klasor)
	bilgi, err := os.Stat(yol)
	if err != nil {
		return "", "", turkce.DosyaHatasi("sunucu.statik_klasör", klasor, "açılamadı", err)
	}
	if !bilgi.IsDir() {
		return "", "", fmt.Errorf("sunucu.statik_klasör: %q bir klasör değil", klasor)
	}
	kok, err := filepath.EvalSymlinks(yol)
	if err != nil {
		return "", "", turkce.DosyaHatasi("sunucu.statik_klasör", klasor, "açılamadı", err)
	}
	if kok, err = filepath.Abs(kok); err != nil {
		return "", "", turkce.DosyaHatasi("sunucu.statik_klasör", klasor, "açılamadı", err)
	}
	return onek, kok, nil
}

// enUzunOnek yolu kapsayan en uzun statik öneki ve klasörünü verir; yoksa boş.
func enUzunOnek(klasorler map[string]string, yol string) (string, string) {
	onek, kok := "", ""
	for o, k := range klasorler {
		if (strings.HasPrefix(yol, o) || yol+"/" == o) && len(o) > len(onek) {
			onek, kok = o, k
		}
	}
	return onek, kok
}

// gizliParca yol parçası sunulmayacak bir ad mı (nokta ile başlayan ya da boş olmayan
// özel karakterli)?
func gizliParca(p string) bool { return strings.HasPrefix(p, ".") }

// statikSun kök klasörün altındaki göreli yolu (URL biçiminde) güvenle sunar.
func statikSun(rw http.ResponseWriter, req *http.Request, kok, goreli string) {
	bulunamadi := func() { jsonHata(rw, http.StatusNotFound, "Dosya bulunamadı", "yol", req.URL.Path) }
	if strings.ContainsAny(goreli, "\\:\x00") {
		bulunamadi()
		return
	}
	temiz := strings.TrimPrefix(path.Clean("/"+goreli), "/")
	for _, p := range strings.Split(temiz, "/") {
		if gizliParca(p) {
			bulunamadi()
			return
		}
	}
	hedef := filepath.Join(kok, filepath.FromSlash(temiz))
	gercek, err := filepath.EvalSymlinks(hedef)
	if err != nil {
		bulunamadi()
		return
	}
	icMi := func(g string) (string, bool) {
		rel, err := filepath.Rel(kok, g)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", false
		}
		// Gerçek yoldaki (bağlantılar ve Windows kısa adları çözülmüş) parçalar da gizli olamaz
		if rel != "." {
			for _, p := range strings.Split(rel, string(filepath.Separator)) {
				if gizliParca(p) {
					return "", false
				}
			}
		}
		return rel, true
	}
	if _, ok := icMi(gercek); !ok {
		bulunamadi()
		return
	}
	bilgi, err := os.Stat(gercek)
	if err != nil {
		bulunamadi()
		return
	}
	if bilgi.IsDir() {
		// Dizin listelenmez; varsa index.html sunulur
		dizin, err := filepath.EvalSymlinks(filepath.Join(gercek, "index.html"))
		if err != nil {
			bulunamadi()
			return
		}
		if _, ok := icMi(dizin); !ok {
			bulunamadi()
			return
		}
		gercek = dizin
		if bilgi, err = os.Stat(gercek); err != nil {
			bulunamadi()
			return
		}
	}
	if !bilgi.Mode().IsRegular() {
		bulunamadi()
		return
	}
	f, err := os.Open(gercek)
	if err != nil {
		bulunamadi()
		return
	}
	defer f.Close()
	http.ServeContent(rw, req, bilgi.Name(), bilgi.ModTime(), f)
}

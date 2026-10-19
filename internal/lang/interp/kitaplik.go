package interp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// KitaplikOneki dilin kendi kitaplığına ayrılmış 'aktar' önekidir (11.6): "tr/katman" diskten
// değil, ikiliye gömülü kitaplıktan (depoda lib/katman.tr) yüklenir. Kullanıcının diskteki
// dosyaları bu öneki gölgeleyemez; kullanıcının "tr" adlı bir klasörü "./tr/..." ile aktarılır.
const KitaplikOneki = "tr/"

// kitaplikDosyalari dilin gömülü kitaplığıdır; komut satırı (cmd/tr) KitaplikBagla ile bağlar.
// Yorumlayıcı kitaplık paketini içe aktarmaz (katman kuralı, docs/mimari.md).
var kitaplikDosyalari fs.FS

// KitaplikBagla dilin TR ile yazılı kitaplığını (kitaplik paketinin gömülü dosyaları) bağlar.
// Program başlamadan önce bir kez çağrılır; nil kitaplığı kaldırır.
func KitaplikBagla(f fs.FS) { kitaplikDosyalari = f }

// KitaplikModulu yol dilin kitaplığını gösteriyorsa (tr/ önekli) modülün kitaplık köküne göre
// adını ".tr" uzantısıyla verir: "tr/katman" → "katman.tr". Ters bölü de ayırıcı sayılır.
func KitaplikModulu(yol string) (ad string, ok bool) {
	y := strings.ReplaceAll(yol, `\`, "/")
	if !strings.HasPrefix(y, KitaplikOneki) {
		return "", false
	}
	ad = strings.TrimPrefix(y, KitaplikOneki)
	if !strings.HasSuffix(ad, ".tr") {
		ad += ".tr"
	}
	return ad, true
}

// KitaplikOku kitaplıktaki bir modülün kaynağını okur. Modül yoksa hata, kitaplıktaki
// modülleri ve (varsa) en yakın adı söyler. Test dosyaları (*_test.tr) modül değildir.
func KitaplikOku(ad string) ([]byte, error) {
	if kitaplikDosyalari == nil {
		return nil, errors.New("dilin kitaplığı bu programa bağlı değil")
	}
	gecerli := fs.ValidPath(ad) && pathpkg.Clean(ad) == ad && !strings.HasSuffix(ad, "_test.tr")
	if gecerli {
		if veri, err := fs.ReadFile(kitaplikDosyalari, ad); err == nil {
			return veri, nil
		}
	}
	moduller := KitaplikModulleri()
	aranan := KitaplikOneki + strings.TrimSuffix(ad, ".tr")
	mesaj := fmt.Sprintf("%q dilin kitaplığında yok; kitaplıktaki modüller: %s", aranan, strings.Join(moduller, ", "))
	if oneri := turkish.EnYakin(aranan, moduller); oneri != "" {
		mesaj += fmt.Sprintf(". Bunu mu demek istediniz: %q?", oneri)
	}
	return nil, errors.New(mesaj)
}

// KitaplikModulleri kitaplıktaki modüllerin 'aktar' yollarıdır ("tr/katman", ...), sıralı.
func KitaplikModulleri() []string {
	if kitaplikDosyalari == nil {
		return nil
	}
	var adlar []string
	fs.WalkDir(kitaplikDosyalari, ".", func(yol string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(yol, ".tr") && !strings.HasSuffix(yol, "_test.tr") {
			adlar = append(adlar, KitaplikOneki+strings.TrimSuffix(yol, ".tr"))
		}
		return nil
	})
	sort.Strings(adlar)
	return adlar
}

// ModulYolunuCoz göreli bir 'aktar' yolunun TEK kuralıdır (11.1): yol sırayla klasorler'e
// (önce aktaran dosyanın klasörü, sonra programın klasörü) göre denenir ve dosyanın bulunduğu
// ilk yer kullanılır; klasör verilmemişse (etkileşimli kabuk, dosyasız program) çalışma
// klasörüne göre. Çalışma klasörü, dosyası olan bir programda bakılan yerlerden değildir:
// başka klasörden çalıştırılan bir modül, yanındaki dosya yerine çalışma klasöründeki aynı
// adlı dosyayı yüklememelidir. Mutlak yol olduğu gibi denenir. Yorumlayıcı ve 'tr derle' /
// 'tr kontrol' bu işlevi kullanır. bakilanlar hata mesajı için denenen yerlerdir.
func ModulYolunuCoz(yol string, klasorler []string) (cozulen string, bakilanlar []string, ok bool) {
	if filepath.IsAbs(yol) {
		_, err := os.Stat(yol)
		return yol, []string{yol}, err == nil
	}
	if len(klasorler) == 0 {
		klasorler = []string{"."}
	}
	var adaylar []string
	for _, k := range klasorler {
		aday := filepath.Join(k, yol)
		mutlak := mutlakYol(aday)
		if slices.Contains(bakilanlar, mutlak) {
			continue
		}
		adaylar = append(adaylar, aday)
		bakilanlar = append(bakilanlar, mutlak)
	}
	for _, aday := range adaylar {
		if bilgi, err := os.Stat(aday); err == nil && !bilgi.IsDir() {
			return aday, bakilanlar, true
		}
	}
	return adaylar[0], bakilanlar, false
}

// DosyaKlasorleri dosya yollarının klasörlerini verir; boş ve "<...>" adlı (dosyası olmayan)
// kaynaklar atlanır. ModulYolunuCoz'a aktaran ve program dosyalarından klasör listesi kurar.
func DosyaKlasorleri(dosyalar ...string) []string {
	var klasorler []string
	for _, d := range dosyalar {
		if d != "" && !strings.HasPrefix(d, "<") {
			klasorler = append(klasorler, filepath.Dir(d))
		}
	}
	return klasorler
}

package dosya

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ortak/baglama"
)

// kopyalaYerlesik kopyala(kaynak, hedef, üzerine_yaz: yanlış) yerleşiğidir.
func kopyalaYerlesik(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	return tasimaYap(m, kod.G124, args, named, false)
}

// tasiYerlesik taşı(kaynak, hedef, üzerine_yaz: yanlış) yerleşiğidir.
func tasiYerlesik(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	return tasimaYap(m, kod.G125, args, named, true)
}

// tasimaIsi bir kopyalama ya da taşımanın denetlenmiş hâlidir; Blocking içinde yapılır.
type tasimaIsi struct {
	fn          string
	tasi        bool
	kaynakHam   string
	kaynak      string // çözülmüş
	kaynakBilgi fs.FileInfo
	son         string // çözülmüş son hedef
	sonHam      string // kullanıcının yazımıyla son hedef
	varKlasor   bool   // son hedef var olan bir klasör (üzerine_yaz ile yerine konacak)
	atlanan     []string
}

func tasimaYap(m *yurut.VM, kd kod.Kod, args []yurut.Value, named map[string]yurut.Value, tasi bool) (yurut.Value, error) {
	is, err := tasimaHazirla(m, kd, args, named, tasi)
	if err != nil {
		return nil, err
	}
	m.Blocking(func() { err = is.yap() })
	if len(is.atlanan) > 0 {
		m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: %s: %d bağlantı ya da özel dosya kopyalanmadı (ilki: %q); TR bağlantıları izlemez\n", is.fn, len(is.atlanan), is.atlanan[0]))
	}
	if err != nil {
		islem := "kopyalanamadı"
		if tasi {
			islem = "taşınamadı"
		}
		return nil, turkce.DosyaHatasi(is.fn, is.kaynakHam, islem, err)
	}
	return YeniYol(is.sonHam), nil
}

// tasimaHazirla argümanları okur, kaynağı ve son hedefi bulur, güvenlik kurallarını denetler.
func tasimaHazirla(m *yurut.VM, kd kod.Kod, args []yurut.Value, named map[string]yurut.Value, tasi bool) (*tasimaIsi, error) {
	fn := yurut.Cagri(kd)
	if err := baglama.AdliDenetle(fn, named, adUzerineYaz); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, yurut.ArgumanSayisiHatasi(kd, "2", len(args), "kaynak, hedef")
	}
	kaynakHam, err := yolOku(kd, 1, args[0])
	if err != nil {
		return nil, err
	}
	hedefHam, err := yolOku(kd, 2, args[1])
	if err != nil {
		return nil, err
	}
	uzerineYaz, err := mantikAdli(kd, named, adUzerineYaz)
	if err != nil {
		return nil, err
	}
	is := &tasimaIsi{fn: fn, tasi: tasi, kaynakHam: kaynakHam}
	if err := is.kaynakBul(m); err != nil {
		return nil, err
	}
	if err := is.hedefBul(m, hedefHam, klasorIstendi(args[1])); err != nil {
		return nil, err
	}
	if err := is.cakismaDenetle(m, uzerineYaz); err != nil {
		return nil, err
	}
	return is, nil
}

// klasorIstendi hedef metni / ile bitiyor mu ("yedek/": var olan bir klasörün içine)?
func klasorIstendi(d yurut.Value) bool {
	s, ok := d.(*yurut.StringVal)
	if !ok {
		return false
	}
	return strings.HasSuffix(s.Val, "/") || (runtime.GOOS == "windows" && strings.HasSuffix(s.Val, `\`))
}

func (is *tasimaIsi) kaynakBul(m *yurut.VM) error {
	cozulen, bakilanlar := m.YolCoz(filepath.FromSlash(is.kaynakHam))
	b, err := os.Lstat(cozulen)
	if err != nil {
		return yurut.OkumaHatasi(is.fn, is.kaynakHam, "bulunamadı", err, bakilanlar)
	}
	switch {
	case baglantiMi(b.Mode()) && !is.tasi:
		return fmt.Errorf("%s: %q bir bağlantı; TR bağlantıları izlemez ve kopyalamaz (bağlantının gösterdiği yolu verin)", is.fn, is.kaynakHam)
	case !b.IsDir() && !b.Mode().IsRegular() && !baglantiMi(b.Mode()):
		return fmt.Errorf("%s: %q sıradan bir dosya ya da klasör değil (aygıt, boru ...); kopyalanmaz", is.fn, is.kaynakHam)
	}
	if is.tasi {
		if err := acikDosyaHatasi(is.fn, is.kaynakHam, gercekYol(cozulen)); err != nil {
			return err
		}
		if neden := silmeKorumasi(m, gercekYol(cozulen)); neden != "" {
			return fmt.Errorf("%s: %q (%s) %s; TR bunu yerinden kaldırmaz", is.fn, is.kaynakHam, filepath.ToSlash(gercekYol(cozulen)), neden)
		}
	}
	is.kaynak, is.kaynakBilgi = cozulen, b
	return nil
}

// hedefBul son hedefi bulur: hedef var olan bir klasörse kaynak onun içine aynı adla gider.
func (is *tasimaIsi) hedefBul(m *yurut.VM, hedefHam string, klasorIste bool) error {
	hedef, _ := m.YolCoz(filepath.FromSlash(hedefHam))
	b, err := os.Lstat(hedef)
	switch {
	case err == nil && b.IsDir():
		ad := filepath.Base(is.kaynak)
		is.son, is.sonHam = filepath.Join(hedef, ad), path.Join(temizle(hedefHam), ad)
		return nil
	case err == nil && baglantiMi(b.Mode()):
		return fmt.Errorf("%s: hedef %q bir bağlantı; TR bağlantıların gösterdiği yere yazmaz (önce %s(...) ile kaldırın)", is.fn, hedefHam, kod.Ad(kod.G123))
	case err == nil && klasorIste:
		return fmt.Errorf("%s: hedef %q bir dosya, klasör değil", is.fn, hedefHam)
	case err != nil && !baglama.YokMu(err):
		return turkce.DosyaHatasi(is.fn, hedefHam, "açılamadı", err)
	case err != nil && klasorIste:
		return fmt.Errorf("%s: hedef klasör %q yok; önce %s(%q) ile oluşturun", is.fn, hedefHam, kod.Ad(kod.G122), strings.TrimRight(hedefHam, `/\`))
	}
	if _, err := os.Stat(filepath.Dir(hedef)); err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = &fs.PathError{Op: pe.Op, Path: hedef, Err: pe.Err}
		}
		return turkce.DosyaHatasi(is.fn, hedefHam, "yazılamadı", err)
	}
	is.son, is.sonHam = hedef, hedefHam
	return nil
}

// cakismaDenetle son hedef zaten varsa üzerine_yaz'ı ve türlerin uyuşmasını, klasörün kendi
// içine kopyalanmamasını denetler.
func (is *tasimaIsi) cakismaDenetle(m *yurut.VM, uzerineYaz bool) error {
	gk, gs := gercekYol(is.kaynak), gercekYol(is.son)
	if ayniYol(gk, gs) {
		return fmt.Errorf("%s: kaynak ile hedef aynı (%q)", is.fn, is.sonHam)
	}
	if is.kaynakBilgi.IsDir() && icindeMi(gs, gk) {
		return fmt.Errorf("%s: %q klasörü kendi içine (%q) konamaz", is.fn, is.kaynakHam, is.sonHam)
	}
	v, err := os.Lstat(is.son)
	if err != nil {
		return nil
	}
	if err := acikDosyaHatasi(is.fn, is.sonHam, gs); err != nil {
		return err
	}
	switch {
	case !uzerineYaz:
		return fmt.Errorf("%s: hedef %q zaten var; yerine koymak için %s doğru verin", is.fn, is.sonHam, adUzerineYaz.TR+":")
	case baglantiMi(v.Mode()):
		return fmt.Errorf("%s: hedef %q bir bağlantı; TR bağlantıların yerine yazmaz (önce %s(...) ile kaldırın)", is.fn, is.sonHam, kod.Ad(kod.G123))
	case v.IsDir() && !is.kaynakBilgi.IsDir():
		return fmt.Errorf("%s: hedef %q bir klasör, kaynak bir dosya; dosya bir klasörün yerine konmaz", is.fn, is.sonHam)
	case !v.IsDir() && is.kaynakBilgi.IsDir():
		return fmt.Errorf("%s: hedef %q bir dosya, kaynak bir klasör; klasör bir dosyanın yerine konmaz", is.fn, is.sonHam)
	case v.IsDir():
		if icindeMi(gk, gs) {
			return fmt.Errorf("%s: hedef %q kaynağı içeriyor; yerine konamaz", is.fn, is.sonHam)
		}
		if neden := silmeKorumasi(m, gs); neden != "" {
			return fmt.Errorf("%s: hedef %q (%s) %s; TR onun yerine bir şey koymaz", is.fn, is.sonHam, filepath.ToSlash(gs), neden)
		}
		is.varKlasor = true
	}
	return nil
}

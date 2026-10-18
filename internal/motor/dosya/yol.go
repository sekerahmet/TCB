package dosya

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ortak/baglama"
)

// YolTipi Yol değerlerinin tip adıdır (kod listesinde A250).
var YolTipi = yurut.TipAdi(kod.A250)

// Yol temizlenmiş bir dosya ya da klasör yoludur. Değiştirilemez; ayırıcıyı her platformda
// / ile saklar ve gösterir (Windows'ta \ de kabul edilir, işletim sistemine giderken
// çevrilir). Göreli bir yol çözülmeden saklanır: kullanıldığı anda, kullanan kodun klasörüne
// göre 13.1'in tek kuralıyla çözülür (yurut.YolDegerli).
type Yol struct{ yol string }

var (
	_ yurut.YolDegerli        = (*Yol)(nil)
	_ yurut.Esitlik           = (*Yol)(nil)
	_ yurut.GorevliUyeKancasi = (*Yol)(nil)
	_ yurut.DuzDegerli        = (*Yol)(nil)
)

// YeniYol bir yol metnini temizleyip Yol yapar.
func YeniYol(s string) *Yol { return &Yol{yol: temizle(s)} }

func (y *Yol) Type() yurut.ValueType { return YolTipi }
func (y *Yol) Inspect() string       { return y.yol }
func (y *Yol) IsTruthy() bool        { return true }

// DosyaYolu yolun kendisidir (çözülmemiş; bkz. yurut.YolDegerli).
func (y *Yol) DosyaYolu() string { return y.yol }

// Esit iki yolun temizlenmiş yazımları aynıysa doğrudur.
func (y *Yol) Esit(d yurut.Value) bool {
	b, ok := d.(*Yol)
	return ok && b.yol == y.yol
}

// DuzDeger yolun dış biçimlerdeki (JSON) karşılığıdır: yol metni.
func (y *Yol) DuzDeger() yurut.Value { return &yurut.StringVal{Val: y.yol} }

// temizle yolu tek yazıma indirir: Windows'ta \ → /, a/./b/../c → a/c, sondaki / atılır
// (kök dışında).
func temizle(s string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(s)))
}

// mutlakParcaMi parça kendi başına bir kökten başlıyor mu (/x, C:/x, C:x, \\sunucu)?
func mutlakParcaMi(p string) bool {
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return true
	}
	return runtime.GOOS == "windows" && (strings.HasPrefix(p, `\`) || filepath.VolumeName(p) != "")
}

// yolYerlesik yol(...) yerleşiğidir: parçaları birleştirip temizler.
func yolYerlesik(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if len(args) == 0 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G120, "en az 1", 0, "parça, ...")
	}
	fn := yurut.Cagri(kod.G120)
	parcalar := make([]string, len(args))
	for i, a := range args {
		p, ok, err := yurut.YolArgumani(fn, a)
		if !ok {
			return nil, yurut.ArgumanTipHatasi(kod.G120, i+1, a, kod.A202, kod.A250)
		}
		if err != nil {
			return nil, err
		}
		if p == "" {
			return nil, fmt.Errorf("%s %d. parça boş metin; bulunulan klasör için \".\" yazın", fn, i+1)
		}
		if i > 0 && mutlakParcaMi(p) {
			return nil, fmt.Errorf("%s %d. parça %q mutlak bir yol; yalnızca ilk parça mutlak olabilir (dışarıdan gelen bir ad taban klasörü değiştiremesin). Göreli bir ad verin", fn, i+1, p)
		}
		parcalar[i] = filepath.FromSlash(p)
	}
	return YeniYol(filepath.Join(parcalar...)), nil
}

// Yol üyeleri (adları kod listesinde, internal/dil/kod Y31x).
var yolUyeKodlari = []kod.Kod{kod.Y310, kod.Y311, kod.Y312, kod.Y313, kod.Y314, kod.Y315, kod.Y316, kod.Y317, kod.Y318}

// GetProperty üyeyi görevsiz okur (göreli yol yalnızca çalışma klasörüne göre çözülür); nokta
// erişimi GorevliUye'yi kullanır.
func (y *Yol) GetProperty(kd kod.Kod, ad string) (yurut.Value, error) {
	return y.GorevliUye(nil, kd, ad)
}

// GorevliUye yolun özelliğini verir; diske bakan özellikler yolu m'nin (çalışan kodun)
// klasörlerine göre çözer.
func (y *Yol) GorevliUye(m *yurut.VM, kd kod.Kod, ad string) (yurut.Value, error) {
	switch kd {
	case kod.Y310:
		return &yurut.StringVal{Val: y.dosyaAdi()}, nil
	case kod.Y311:
		return &yurut.StringVal{Val: uzanti(y.dosyaAdi())}, nil
	case kod.Y312:
		a := y.dosyaAdi()
		return &yurut.StringVal{Val: a[:len(a)-len(uzanti(a))]}, nil
	case kod.Y313:
		return &Yol{yol: filepath.ToSlash(filepath.Dir(filepath.FromSlash(y.yol)))}, nil
	case kod.Y314:
		return y.mutlak(m), nil
	case kod.Y315, kod.Y316, kod.Y317:
		return y.soru(m, kd), nil
	case kod.Y318:
		return y.bayt(m)
	}
	return nil, baglama.UyeHatasi(string(YolTipi), ad, kod.Adlar(yolUyeKodlari...))
}

// dosyaAdi son bileşendir; . ve kök için boştur.
func (y *Yol) dosyaAdi() string {
	if y.yol == "." || strings.HasSuffix(y.yol, "/") {
		return ""
	}
	return y.yol[strings.LastIndexByte(y.yol, '/')+1:]
}

// uzanti adın noktasıyla son uzantısıdır; nokta ile başlayan adda ve noktayla bitende boştur.
func uzanti(ad string) string {
	i := strings.LastIndexByte(ad, '.')
	if i <= 0 || i == len(ad)-1 {
		return ""
	}
	return ad[i:]
}

// coz yolu m'nin klasörlerine göre çözer (m nil ise yalnızca çalışma klasörü).
func (y *Yol) coz(m *yurut.VM) (string, []string) {
	return m.YolCoz(filepath.FromSlash(y.yol))
}

func (y *Yol) mutlak(m *yurut.VM) *Yol {
	c, _ := y.coz(m)
	if a, err := filepath.Abs(c); err == nil {
		c = a
	}
	return YeniYol(c)
}

// soru var_mı, dosya_mı ve klasör_mü'dür (bağlantı izlenir).
func (y *Yol) soru(m *yurut.VM, kd kod.Kod) yurut.Value {
	c, _ := y.coz(m)
	b, err := os.Stat(c)
	switch {
	case err != nil:
		return yurut.FALSE
	case kd == kod.Y316:
		return yurut.BoolToVal(b.Mode().IsRegular())
	case kd == kod.Y317:
		return yurut.BoolToVal(b.IsDir())
	}
	return yurut.TRUE
}

func (y *Yol) bayt(m *yurut.VM) (yurut.Value, error) {
	fn := kod.UyeAdi(kod.Y318)
	c, bakilanlar := y.coz(m)
	b, err := os.Stat(c)
	if err != nil {
		return nil, yurut.OkumaHatasi(fn, y.yol, "okunamadı", err, bakilanlar)
	}
	if b.IsDir() {
		return nil, fmt.Errorf("%s: %q bir klasör; klasörün boyutu yoktur (içindeki dosyaların boyutunu toplayın: %s(...))", fn, y.yol, kod.Ad(kod.G121))
	}
	return yurut.NewInt(b.Size()), nil
}

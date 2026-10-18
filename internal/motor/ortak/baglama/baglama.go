// Package baglama, Go paketlerini TR diline bağlayan motor paketlerinin (tensör,
// parçalayıcı, veri) ortak yardımcılarıdır: adlı argüman denetimi, göreli yol çözme,
// paniği hataya çevirme ve sayıların Türkçe kısa yazımı. Kullanıcıya görünen hiçbir ad
// tanımlamaz: adlı argümanların adları kod listesindedir (internal/dil/kod, x9xx kodları).
package baglama

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
)

// Ad bir adlı argümandır: sözlükteki kodu (x9xx), adı ve kabul edilen yazımları. Adlar
// kod listesinden gelir (internal/dil/kod); Go kodu yalnızca kodu bilir.
type Ad struct {
	Kod kod.Kod
	// TR adın kendisidir (hata mesajlarında ve önerilerde gösterilir).
	TR       string
	yazimlar []string
}

// KodAd sözlükteki bir adlı argüman kodunun Ad'ıdır. Paket düzeyinde (başlangıçta) bir
// kez çağrılır; kod bir adlı argüman kodu değilse panikler.
func KodAd(k kod.Kod) Ad {
	if k.Alan() != kod.AlanParametre {
		panic(fmt.Sprintf("baglama.KodAd: %s bir adlı argüman kodu (x9xx) değil", k))
	}
	return Ad{Kod: k, TR: kod.Ad(k), yazimlar: kod.Yazimlar(k)}
}

// Yazimlar adlı argümanın kabul edilen yazımlarıdır (önce adın kendisi). Dönen dilim
// değiştirilmemelidir.
func (a Ad) Yazimlar() []string { return a.yazimlar }

// Kabul verilen yazım bu adlı argümanın kabul edilen bir yazımı mı?
func (a Ad) Kabul(yazim string) bool { return slices.Contains(a.yazimlar, yazim) }

// yabanciAdlar başka dillerden alışılmış parametre adlarının TR karşılıklarıdır. Bu adlar
// KABUL EDİLMEZ; yalnızca hata mesajında doğru TR adı önerilir.
var yabanciAdlar = map[string]kod.Kod{
	"lr":            kod.T910, // öğrenme_oranı
	"learning_rate": kod.T910,
	"axis":          kod.T902, // eksen
	"dim":           kod.T902,
	"requires_grad": kod.T900, // türev_izle
	"weight_decay":  kod.T911, // ağırlık_çürümesi
	"eps":           kod.T905, // epsilon
	"seed":          kod.T903, // tohum
	"weight":        kod.T920, // ağırlık
	"bias":          kod.T921, // sapma
	"betas":         kod.T912, // beta1
}

// AdliDenetle verilen adlı argümanların bilinen adlardan olduğunu denetler; yazım
// hatasında (ya da Türkçe karaktersiz yazılmış, kaldırılmış ya da İngilizce alışılmış bir
// adda: ogrenme_orani, nerede, lr, axis...) en yakın TR adını önerir.
func AdliDenetle(fn string, named map[string]yurut.Value, adlar ...Ad) error {
	for verilen := range named {
		bilinen := false
		for _, a := range adlar {
			if a.Kabul(verilen) {
				bilinen = true
				break
			}
		}
		if bilinen {
			continue
		}
		var adaylar []string
		for _, a := range adlar {
			adaylar = append(adaylar, a.TR)
		}
		o := ""
		if k, ok := kod.Oneri(kod.AlanParametre, verilen, nil); ok && slices.Contains(adaylar, kod.Ad(k)) {
			o = kod.Ad(k)
		}
		if k, ok := yabanciAdlar[strings.ToLower(verilen)]; o == "" && ok && slices.Contains(adaylar, kod.Ad(k)) {
			o = kod.Ad(k)
		}
		if o == "" {
			o = turkce.EnYakin(verilen, adaylar)
		}
		oneri := ""
		if o != "" {
			oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
		} else if len(adaylar) > 0 {
			oneri = fmt.Sprintf(" (geçerli adlar: %s)", strings.Join(adaylar, ", "))
		} else {
			oneri = " (adlı argüman almaz)"
		}
		return fmt.Errorf("%s '%s' adında bir parametre almaz%s", fn, verilen, oneri)
	}
	return nil
}

// AdliAl adlı argümanı kabul edilen yazımlarıyla (önce adın kendisiyle) arar.
func AdliAl(named map[string]yurut.Value, a Ad) (yurut.Value, bool) {
	for _, y := range a.yazimlar {
		if v, ok := named[y]; ok {
			return v, true
		}
	}
	return nil, false
}

// TamSayi bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func TamSayi(v yurut.Value) (int64, bool) {
	switch n := v.(type) {
	case *yurut.IntVal:
		return n.Val, true
	case *yurut.FloatVal:
		if n.Val == float64(int64(n.Val)) {
			return int64(n.Val), true
		}
	}
	return 0, false
}

// Mantik bir TR değerinin doğru/yanlış olmasını bekler.
func Mantik(fn, ad string, v yurut.Value) (bool, error) {
	b, ok := v.(*yurut.BoolVal)
	if !ok {
		return false, fmt.Errorf("%s: %s doğru ya da yanlış olmalıdır, %s verildi", fn, ad, v.Type())
	}
	return b.Val, nil
}

// Kurtar fn'i çalıştırır; fn panik yaparsa programı çökertmek yerine paniği cevir ile
// TR'nin dene/yakala ile yakalanabilen bir hataya çevirir.
func Kurtar[T any](fn func() T, cevir func(r any) error) (sonuc T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var sifir T
			sonuc, err = sifir, cevir(r)
		}
	}()
	return fn(), nil
}

// Guvenli f'yi çalıştırır; Go paniğini "beklenmeyen iç hata" olarak bildirir.
func Guvenli(fn string, f func() (yurut.Value, error)) (yurut.Value, error) {
	var ferr error
	sonuc, err := Kurtar(func() yurut.Value {
		v, e := f()
		ferr = e
		return v
	}, func(r any) error { return fmt.Errorf("%s: beklenmeyen iç hata: %v", fn, r) })
	if err != nil {
		return nil, err
	}
	return sonuc, ferr
}

// YolCoz göreli bir yolu TR'nin tek yol kuralıyla çözer (bkz. vm.(*VM).YolCoz: önce
// çalışma klasörü, sonra programın klasörü). Hiçbir yerde yoksa hata mesajı kullanıcının
// yazdığı yolu gösterebilsin diye çalışma klasörüne göre yol döner.
func YolCoz(m *yurut.VM, yol string) string {
	cozulen, _ := YolCozBakilan(m, yol)
	return cozulen
}

// YolCozBakilan YolCoz gibidir; ayrıca hata mesajları için bakılan yerleri döndürür.
func YolCozBakilan(m *yurut.VM, yol string) (string, []string) {
	return m.YolCoz(yol) // m nil olabilir: o zaman yalnızca çalışma klasörüne bakılır
}

// YokMu hatanın "dosya ya da klasör yok" anlamına gelip gelmediğini söyler.
func YokMu(err error) bool { return errors.Is(err, os.ErrNotExist) }

// KisaSayi büyük sayıları Türkçe kısa biçimde yazar: 999 → "999", 12 345 → "12,3 bin",
// 56 900 000 000 → "56,9 milyar".
func KisaSayi(n int64) string {
	birimler := []struct {
		esik int64
		ad   string
	}{{1e12, "trilyon"}, {1e9, "milyar"}, {1e6, "milyon"}, {1e3, "bin"}}
	for _, b := range birimler {
		if n >= b.esik {
			x := float64(n) / float64(b.esik)
			s := strconv.FormatFloat(x, 'f', 1, 64)
			s = strings.TrimSuffix(s, ".0")
			return strings.Replace(s, ".", ",", 1) + " " + b.ad
		}
	}
	return strconv.FormatInt(n, 10)
}

// BaytMetni bayt sayısını okunur biçimde yazar (ör. "1,2 GB").
func BaytMetni(n int64) string {
	birimler := []struct {
		esik int64
		ad   string
	}{{1 << 40, "TB"}, {1 << 30, "GB"}, {1 << 20, "MB"}, {1 << 10, "KB"}}
	for _, b := range birimler {
		if n >= b.esik {
			s := strconv.FormatFloat(float64(n)/float64(b.esik), 'f', 1, 64)
			s = strings.TrimSuffix(s, ".0")
			return strings.Replace(s, ".", ",", 1) + " " + b.ad
		}
	}
	return strconv.FormatInt(n, 10) + " bayt"
}

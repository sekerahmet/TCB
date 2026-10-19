// Package bind, Go paketlerini TR diline bağlayan motor paketlerinin (tensör,
// parçalayıcı, veri) ortak yardımcılarıdır: adlı argüman denetimi, göreli yol çözme,
// paniği hataya çevirme ve sayıların Türkçe kısa yazımı. Kullanıcıya görünen hiçbir ad
// tanımlamaz: adlı argümanların adları kod listesindedir (internal/lang/codes, x9xx kodları).
//
// Katman: motor (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package bind

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Ad bir adlı argümandır: sözlükteki kodu (x9xx), adı ve kabul edilen yazımları. Adlar
// kod listesinden gelir (internal/lang/codes); Go kodu yalnızca kodu bilir.
type Ad struct {
	Kod codes.Kod
	// TR adın kendisidir (hata mesajlarında ve önerilerde gösterilir).
	TR       string
	yazimlar []string
}

// KodAd sözlükteki bir adlı argüman kodunun Ad'ıdır. Paket düzeyinde (başlangıçta) bir
// kez çağrılır; kod bir adlı argüman kodu değilse panikler.
func KodAd(k codes.Kod) Ad {
	if k.Alan() != codes.AlanParametre {
		panic(fmt.Sprintf("baglama.KodAd: %s bir adlı argüman kodu (x9xx) değil", k))
	}
	return Ad{Kod: k, TR: codes.Ad(k), yazimlar: codes.Yazimlar(k)}
}

// Yazimlar adlı argümanın kabul edilen yazımlarıdır (önce adın kendisi). Dönen dilim
// değiştirilmemelidir.
func (a Ad) Yazimlar() []string { return a.yazimlar }

// Kabul verilen yazım bu adlı argümanın kabul edilen bir yazımı mı?
func (a Ad) Kabul(yazim string) bool { return slices.Contains(a.yazimlar, yazim) }

// AdliDenetle verilen adlı argümanların bilinen adlardan olduğunu denetler; yazım
// hatasında (ya da Türkçe karaktersiz yazılmış, kaldırılmış ya da başka dillerden alışılmış
// bir adda: ogrenme_orani, nerede, lr, axis...) en yakın TR adını önerir. Kaldırılmış ve
// alışılmış adlar kod listesindedir (tr.json'da "eski" ve "yabancı").
func AdliDenetle(fn string, named map[string]interp.Value, adlar ...Ad) error {
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
		if k, ok := codes.Oneri(codes.AlanParametre, verilen, nil); ok && slices.Contains(adaylar, codes.Ad(k)) {
			o = codes.Ad(k)
		}
		if k, ok := codes.Oneri(codes.AlanParametre, strings.ToLower(verilen), nil); o == "" && ok && slices.Contains(adaylar, codes.Ad(k)) {
			o = codes.Ad(k) // LR, Seed: alışılmış adlar büyük harfle de tanınır
		}
		if o == "" {
			o = turkish.EnYakin(verilen, adaylar)
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

// UyeHatasi motor tiplerinde bulunamayan bir üye için hatadır: yazıma en yakın üyeyi, yakın
// bir üye yoksa tipin üyelerini önerir. ne tipin (ya da nesnenin) kullanıcıya görünen adıdır.
func UyeHatasi(ne, prop string, uyeler []string) error {
	oneri := ""
	if o := turkish.EnYakin(prop, uyeler); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else {
		oneri = fmt.Sprintf(" (üyeler: %s)", strings.Join(uyeler, ", "))
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", ne, prop, oneri)
}

// AdliAl adlı argümanı kabul edilen yazımlarıyla (önce adın kendisiyle) arar.
func AdliAl(named map[string]interp.Value, a Ad) (interp.Value, bool) {
	for _, y := range a.yazimlar {
		if v, ok := named[y]; ok {
			return v, true
		}
	}
	return nil, false
}

// TamSayi bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func TamSayi(v interp.Value) (int64, bool) {
	switch n := v.(type) {
	case *interp.IntVal:
		return n.Val, true
	case *interp.FloatVal:
		if n.Val == float64(int64(n.Val)) {
			return int64(n.Val), true
		}
	}
	return 0, false
}

// Mantik bir TR değerinin doğru/yanlış olmasını bekler.
func Mantik(fn, ad string, v interp.Value) (bool, error) {
	b, ok := v.(*interp.BoolVal)
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
			if interp.KesilmeMi(r) {
				panic(r) // kesilme (Ctrl+C) hata değildir; yığın çözülmeye devam eder
			}
			var sifir T
			sonuc, err = sifir, cevir(r)
		}
	}()
	return fn(), nil
}

// Guvenli f'yi çalıştırır; Go paniğini "beklenmeyen iç hata" olarak bildirir.
func Guvenli(fn string, f func() (interp.Value, error)) (interp.Value, error) {
	var ferr error
	sonuc, err := Kurtar(func() interp.Value {
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
func YolCoz(m *interp.VM, yol string) string {
	cozulen, _ := YolCozBakilan(m, yol)
	return cozulen
}

// YolCozBakilan YolCoz gibidir; ayrıca hata mesajları için bakılan yerleri döndürür.
func YolCozBakilan(m *interp.VM, yol string) (string, []string) {
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

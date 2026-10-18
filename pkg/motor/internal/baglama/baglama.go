// Package baglama, Go paketlerini TR diline bağlayan motor paketlerinin (parçalayıcı,
// veri) ortak yardımcılarıdır: adlı argüman denetimi, göreli yol çözme, paniği hataya
// çevirme ve sayıların Türkçe kısa yazımı. Kullanıcıya görünen hiçbir ad tanımlamaz.
package baglama

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Ad, bir adlı argümanın Türkçe adı ve (varsa) ASCII karşılığıdır.
type Ad struct {
	TR, ASCII string
}

// AdliDenetle verilen adlı argümanların bilinen adlardan olduğunu denetler; yazım
// hatasında en yakın Türkçe adı önerir.
func AdliDenetle(fn string, named map[string]vm.Value, adlar ...Ad) error {
	for verilen := range named {
		bilinen := false
		for _, a := range adlar {
			if verilen == a.TR || (a.ASCII != "" && verilen == a.ASCII) {
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
		oneri := ""
		if o := turkce.EnYakin(verilen, adaylar); o != "" {
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

// AdliAl adlı argümanı Türkçe adıyla ya da ASCII karşılığıyla arar.
func AdliAl(named map[string]vm.Value, a Ad) (vm.Value, bool) {
	if v, ok := named[a.TR]; ok {
		return v, true
	}
	if a.ASCII != "" {
		if v, ok := named[a.ASCII]; ok {
			return v, true
		}
	}
	return nil, false
}

// TamSayi bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func TamSayi(v vm.Value) (int64, bool) {
	switch n := v.(type) {
	case *vm.IntVal:
		return n.Val, true
	case *vm.FloatVal:
		if n.Val == float64(int64(n.Val)) {
			return int64(n.Val), true
		}
	}
	return 0, false
}

// Mantik bir TR değerinin doğru/yanlış olmasını bekler.
func Mantik(fn, ad string, v vm.Value) (bool, error) {
	b, ok := v.(*vm.BoolVal)
	if !ok {
		return false, fmt.Errorf("%s: %s doğru ya da yanlış olmalıdır, %s verildi", fn, ad, v.Type())
	}
	return b.Val, nil
}

// Guvenli f'yi çalıştırır; Go paniğini programı çökertmek yerine TR'nin dene/yakala
// ile yakalanabilen bir hataya çevirir.
func Guvenli(fn string, f func() (vm.Value, error)) (sonuc vm.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			sonuc = nil
			if e, ok := r.(error); ok {
				err = fmt.Errorf("%s: beklenmeyen iç hata: %v", fn, e)
			} else {
				err = fmt.Errorf("%s: beklenmeyen iç hata: %v", fn, r)
			}
		}
	}()
	return f()
}

// YolCoz göreli bir yolu önce çalışma klasörüne göre dener; orada yoksa ve program bir
// dosyadan çalışıyorsa o dosyanın klasörüne göre çözer. Hiçbiri yoksa yolu olduğu gibi
// döndürür (hata mesajı kullanıcının yazdığı yolu gösterir).
func YolCoz(m *vm.VM, yol string) string {
	if yol == "" || filepath.IsAbs(yol) {
		return yol
	}
	if _, err := os.Stat(yol); err == nil {
		return yol
	}
	if m != nil && m.SourceFile != "" && !strings.HasPrefix(m.SourceFile, "<") {
		aday := filepath.Join(filepath.Dir(m.SourceFile), yol)
		if _, err := os.Stat(aday); err == nil {
			return aday
		}
	}
	return yol
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

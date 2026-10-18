package yurut

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"tr_dili/internal/dil/kod"
)

// Taşınan kodlar: işi başka bir biçime taşınan kodların (fonksiyondan metoda ya da
// metottan fonksiyona; tanim.json'da "taşındı") eski adları kabul edilmez; yazılınca hata
// yeni biçimi gösterir: uzunluk(x) → x.uzunluk, liste.toplam() → toplam(liste). Kural:
// bir değeri dönüştüren ya da ona soru soran iş metottur, matematik ve tensör işi
// fonksiyondur (bkz. docs/dil/13_yerlesikler.md 13.6). Bu işlevler yalnızca hata yolunda çalışır.

// alici bir tipin hata mesajında alıcı olarak yazılan adıdır: Liste → liste.
func alici(tip kod.Kod) string {
	return strings.ToLowerSpecial(unicode.TurkishCase, kod.Ad(tip))
}

// tasinanGenelAdHatasi ad (genel adlarda) taşınmış bir kodun adı, eski adı ya da bunların
// ASCII biçimiyse yeni biçimi gösteren hata metnini verir; değilse boş metin.
// ör. "'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın".
func tasinanGenelAdHatasi(ad string) string {
	eski, ok := kod.OluBul(kod.AlanGenel, ad)
	if !ok {
		return ""
	}
	if kod.Kaldirildi(eski) {
		return kaldirildiHatasi(ad)
	}
	yeni, _ := kod.Tasindi(eski)
	if yeni.Alan() != kod.AlanUye {
		return ""
	}
	return fmt.Sprintf("'%s(...)' TR'de bir fonksiyon değil; '%s' yazın", ad, kod.UyeYazimi(yeni))
}

// tasinanUyeHatasi prop, obj'nin tipinde taşınmış bir üyenin adıysa (ve yeni fonksiyon bu
// tipi alıyorsa) yeni biçimi gösteren hata metnini verir; değilse boş metin.
// ör. "'liste.toplam()' TR'de yok; 'toplam(liste)' yazın".
func tasinanUyeHatasi(obj Value, prop string) string {
	eski, ok := kod.OluBul(kod.AlanUye, prop)
	if !ok {
		return ""
	}
	if kod.Kaldirildi(eski) {
		return kaldirildiHatasi(prop)
	}
	yeni, _ := kod.Tasindi(eski)
	tip, ok := kod.Bul(kod.AlanTip, string(obj.Type()))
	if !ok {
		return ""
	}
	t, _ := kod.Tanimi(yeni)
	if t.Tur != kod.TurFonksiyon || len(t.Imza.Param) == 0 || !slices.Contains(t.Imza.Param[0].Tipler, tip) {
		return ""
	}
	a := alici(tip)
	return fmt.Sprintf("'%s.%s()' TR'de yok; '%s(%s)' yazın", a, prop, kod.Ad(yeni), a)
}

// kaldirildiHatasi yerine geçen olmadan kaldırılmış bir adın hata metnidir.
func kaldirildiHatasi(ad string) string {
	return fmt.Sprintf("'%s' TR'den kaldırıldı", ad)
}

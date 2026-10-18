package yurut

import (
	"fmt"
	"slices"

	"tr_dili/internal/dil/ipucu"
	"tr_dili/internal/dil/kod"
)

// Taşınan kodlar: işi başka bir biçime taşınan kodların (fonksiyondan metoda ya da
// metottan fonksiyona; tanim.json'da "taşındı") eski adları kabul edilmez; yazılınca hata
// yeni biçimi gösterir: uzunluk(x) → x.uzunluk, liste.toplam() → toplam(liste). Kural:
// bir değeri dönüştüren ya da ona soru soran iş metottur, matematik ve tensör işi
// fonksiyondur (bkz. docs/dil/13_yerlesikler.md 13.6). Bu işlevler yalnızca hata yolunda çalışır.

// tasinanUyeHatasi prop, obj'nin tipinde taşınmış bir üyenin adıysa (ve yeni fonksiyon bu
// tipi alıyorsa) yeni biçimi gösteren hata metnini verir; değilse boş metin.
// ör. "'liste.toplam()' TR'de yok; 'toplam(liste)' yazın".
func tasinanUyeHatasi(obj Value, prop string) string {
	eski, ok := kod.OluBul(kod.AlanUye, prop)
	if !ok {
		return ""
	}
	if kod.Kaldirildi(eski) {
		return ipucu.KaldirildiHatasi(prop)
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
	a := kod.AliciAdi(tip)
	return fmt.Sprintf("'%s.%s()' TR'de yok; '%s(%s)' yazın", a, prop, kod.Ad(yeni), a)
}

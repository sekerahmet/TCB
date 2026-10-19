package interp

import (
	"fmt"
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Taşınan kodlar: işi başka bir biçime taşınan kodların (fonksiyondan metoda ya da
// metottan fonksiyona; tanim.json'da "taşındı") eski adları kabul edilmez; yazılınca hata
// yeni biçimi gösterir: uzunluk(x) → x.uzunluk, liste.toplam() → toplam(liste). Kural:
// bir değeri dönüştüren ya da ona soru soran iş metottur, matematik ve tensör işi
// fonksiyondur (bkz. docs/spec/13_yerlesikler.md 13.6). Bu işlevler yalnızca hata yolunda çalışır.

// tasinanUyeHatasi prop, obj'nin tipinde taşınmış bir üyenin adıysa (ve yeni fonksiyon bu
// tipi alıyorsa) yeni biçimi gösteren hata metnini verir; değilse boş metin.
// ör. "'liste.toplam()' TR'de yok; 'toplam(liste)' yazın".
func tasinanUyeHatasi(obj Value, prop string) string {
	eski, ok := codes.OluBul(codes.AlanUye, prop)
	if !ok {
		return ""
	}
	if codes.Kaldirildi(eski) {
		return hint.KaldirildiHatasi(prop)
	}
	yeni, _ := codes.Tasindi(eski)
	tip, ok := codes.Bul(codes.AlanTip, string(obj.Type()))
	if !ok {
		return ""
	}
	t, _ := codes.Tanimi(yeni)
	if t.Tur != codes.TurFonksiyon || len(t.Imza.Param) == 0 || !slices.Contains(t.Imza.Param[0].Tipler, tip) {
		return ""
	}
	a := codes.AliciAdi(tip)
	return fmt.Sprintf("'%s.%s()' TR'de yok; '%s(%s)' yazın", a, prop, codes.Ad(yeni), a)
}

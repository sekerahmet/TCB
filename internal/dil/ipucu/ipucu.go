// Package ipucu dilin adlarıyla ilgili hata ipuçlarının tek yazımıdır: kaldırılmış ya da
// taşınmış adlar, Türkçe karaktersiz (ASCII) yazılmış adlar ve tanımsız bir ada en yakın
// görünen ad. Aynı metinler sözcük çözümleyicide, ayrıştırıcıda, denetimde (çalışmadan
// önce) ve çalışma anında kullanılır. Bütün işlevler yalnızca hata yolunda çalışır; adlar
// kod listesinden (internal/dil/kod) gelir.
//
// Katman: dil çekirdeği (internal/dil); yalnızca turkce ve kod paketlerini içe aktarır.
package ipucu

import (
	"fmt"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
)

// KaldirildiHatasi yerine geçen olmadan kaldırılmış bir adın hata metnidir.
func KaldirildiHatasi(ad string) string {
	return fmt.Sprintf("'%s' TR'den kaldırıldı", ad)
}

// YokIpucu bir adın yerine asıl (Türkçe) adı öneren ipucudur.
func YokIpucu(ad, asil string) string {
	return fmt.Sprintf(" ('%s' TR'de yok; '%s' yazmak mı istediniz?)", ad, asil)
}

// TasinanGenelAdHatasi ad (genel adlarda) taşınmış bir kodun adı, eski adı ya da bunların
// ASCII biçimiyse yeni biçimi gösteren hata metnini verir; değilse boş metin.
// ör. "'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın".
func TasinanGenelAdHatasi(ad string) string {
	eski, ok := kod.OluBul(kod.AlanGenel, ad)
	if !ok {
		return ""
	}
	if kod.Kaldirildi(eski) {
		return KaldirildiHatasi(ad)
	}
	yeni, _ := kod.Tasindi(eski)
	if yeni.Alan() != kod.AlanUye {
		return ""
	}
	return fmt.Sprintf("'%s(...)' TR'de bir fonksiyon değil; '%s' yazın", ad, kod.UyeYazimi(yeni))
}

// KaldirilanYazimIpucu ad bir anahtar kelimenin Türkçe karaktersiz (ASCII) biçimiyse
// (degisken) ya da kaldırılmış bir anahtar kelimeyse (sözlükte bir A kodunun "eski" adı:
// işlev, islev) hata mesajına eklenecek ipucunu, değilse boş metni döndürür. Bu yazımlar
// sıradan adlardır (ör. değişken sınıf = "9-A"); yalnızca bir hata oluştuğunda, alışkanlıkla
// yazılmış olabilecekleri için asıl ad önerilir. Öneri her zaman Türkçe biçimdir.
func KaldirilanYazimIpucu(ad string) string {
	k, ok := kod.Oneri(kod.AlanGenel, ad, func(k kod.Kod) bool { return k.Harf() == 'A' })
	if !ok {
		return ""
	}
	return fmt.Sprintf(" ('%s' TR'de anahtar kelime değil; '%s' yazmak mı istediniz?)", ad, kod.Ad(k))
}

// KaldirilanYerlesikIpucu ad bir yerleşiğin Türkçe karaktersiz (ASCII) biçimiyse (yazdir)
// ya da kaldırılmış bir yerleşik adıysa (sözlükte bir G/T/V kodunun "eski" adı) hata
// mesajına eklenecek ipucunu, değilse boş metni döndürür. Bu yazımlar sıradan adlardır
// (ör. değişken boy = 180); yalnızca tanımsız ad hatasında, alışkanlıkla yazılmış
// olabilecekleri için asıl (Türkçe) ad önerilir.
func KaldirilanYerlesikIpucu(ad string) string {
	k, ok := kod.Oneri(kod.AlanGenel, ad, kod.Kod.Yerlesik)
	if !ok {
		return ""
	}
	return YokIpucu(ad, kod.Ad(k))
}

// AdOnerisi tanımsız bir ad için hata mesajına eklenecek öneriyi verir: kaldırılmış ya da
// Türkçe karaktersiz yazılmış bir anahtar kelime veya yerleşikse asıl adı, değilse görünen
// adlar arasında yazıma en yakın olanı ("Bunu mu demek istediniz?"); yoksa boş metin.
func AdOnerisi(ad string, gorunenler []string) string {
	if i := KaldirilanYazimIpucu(ad); i != "" {
		return i
	}
	if i := KaldirilanYerlesikIpucu(ad); i != "" {
		return i
	}
	if o := turkce.EnYakin(ad, gorunenler); o != "" {
		return fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return ""
}

// TanimsizAdMetni çalışma anında ve denetimde tanımsız bir ad için aynı mesajı verir
// (konum öneki olmadan).
func TanimsizAdMetni(ad string, gorunenler []string) string {
	if h := TasinanGenelAdHatasi(ad); h != "" {
		return h
	}
	return fmt.Sprintf("'%s' adında bir değişken ya da fonksiyon yok%s", ad, AdOnerisi(ad, gorunenler))
}

package denetci

import (
	"fmt"
	"sync"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/turkce"
)

// Tanımsız ad ipuçları. Aynı metinler hem denetimde (çalışmadan önce, tanimsiz_adlar.go) hem
// çalışma anında (yurut) kullanılır; tek yazımları buradadır. Yalnızca hata yolunda çalışırlar.

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

// KaldirilanYerlesikIpucu ad bir yerleşiğin Türkçe karaktersiz (ASCII) biçimiyse (yazdir)
// ya da kaldırılmış bir yerleşik adıysa (sözlükte bir G/T/V/W kodunun "eski" adı) hata
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
	if ipucu := sozcuk.KaldirilanYazimIpucu(ad); ipucu != "" {
		return ipucu
	}
	if ipucu := KaldirilanYerlesikIpucu(ad); ipucu != "" {
		return ipucu
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

// yerlesikOnbellegi etkin dilin yerleşik adlarını bir kez hesaplar.
var yerlesikOnbellegi struct {
	sync.Mutex
	dil   *kod.Dil
	adlar map[string]bool
}

// yerlesikAdlari sözlükteki yerleşiklerin (G/T/V/W) kabul edilen adlarıdır; denetimde bu
// adlar her kapsamda tanımlıdır (çalışma anında genel kapsama kaydedilirler).
func yerlesikAdlari() map[string]bool {
	o := &yerlesikOnbellegi
	o.Lock()
	defer o.Unlock()
	if o.dil != kod.Etkin() {
		m := map[string]bool{}
		for _, k := range kod.Kodlar() {
			if k.Yerlesik() && !kod.Kaldirildi(k) {
				for _, y := range kod.Yazimlar(k) {
					m[y] = true
				}
			}
		}
		o.dil, o.adlar = kod.Etkin(), m
	}
	return o.adlar
}

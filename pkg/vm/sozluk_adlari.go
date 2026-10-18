package vm

import (
	"fmt"

	"tr_dili/pkg/sozluk"
)

// Yorumlayıcının kaydettiği adlar sözlükten gelir (pkg/sozluk). Adlar kayıt sırasında
// (VM kurulurken) bir kez okunur; çalışma anında sözlüğe bakılmaz.

// RegisterBuiltinKod sözlükteki bir yerleşik kodunu (G/T/V/W) genel kapsama kaydeder.
// Ad ve kabul edilen öbür yazımlar (Türkçe karaktersiz biçim) sözlükten gelir; her işin
// tek adı vardır. Kod bir yerleşik kodu değilse ya da sözlükte yoksa bu bir geliştirici
// hatasıdır ve kayıt sırasında panikler.
func (v *VM) RegisterBuiltinKod(kod sozluk.Kod, fn BuiltinFunc) *BuiltinVal {
	return v.YerlesikTanimla(kod, &BuiltinVal{Name: sozluk.Ad(kod), Fn: fn})
}

// YerlesikTanimla hazır bir yerleşik değerini (adı kodun adı olmalı) kodun bütün
// yazımlarıyla genel kapsama bağlar (ör. tensör motorunun eski yerleşiği saran değeri).
// Eş yazımlar (sozluk.Girdi.Es) aynı fonksiyonu kendi adlarıyla taşıyan ayrı bir değere
// bağlanır; böylece <yerleşik veritabanı> gibi gösterimler eskisi gibi kalır.
func (v *VM) YerlesikTanimla(kod sozluk.Kod, b *BuiltinVal) *BuiltinVal {
	if !kod.Yerlesik() {
		panic(fmt.Sprintf("yerleşik kaydı: %s bir yerleşik kodu (G/T/V/W, x9xx dışı) değil", kod))
	}
	var esler []*BuiltinVal // eş yazımların kendi adlı kopyaları (çoğu kodda yok)
	for _, yazim := range sozluk.Yazimlar(kod) {
		d := b
		if sozluk.EsYazimi(kod, yazim) {
			ad, _ := sozluk.AsilAdAlanda(sozluk.AlanGenel, yazim)
			d = nil
			for _, e := range esler {
				if e.Name == ad {
					d = e
				}
			}
			if d == nil {
				kopya := *b
				kopya.Name = ad
				d = &kopya
				esler = append(esler, d)
			}
		}
		v.GlobalEnv.Define(yazim, d, true)
	}
	return b
}

// Yerlesik kodun genel kapsamda kayıtlı yerleşiğini verir; yoksa nil.
func (v *VM) Yerlesik(kod sozluk.Kod) *BuiltinVal {
	b, _ := v.GlobalEnv.yerelDeger(sozluk.Ad(kod)).(*BuiltinVal)
	return b
}

// KaldirilanYerlesikIpucu ad kaldırılmış bir yerleşik adıysa (sözlükte bir G/T/V/W
// kodunun "eski" adı) hata mesajına eklenecek ipucunu, değilse boş metni döndürür.
// Kaldırılan adlar artık sıradan adlardır (ör. değişken boy = 180); yalnızca tanımsız ad
// hatasında, eski alışkanlıkla yazılmış olabilecekleri için asıl ad önerilir.
func KaldirilanYerlesikIpucu(ad string) string {
	k, ascii, ok := sozluk.EskiBul(sozluk.AlanGenel, ad, sozluk.Kod.Yerlesik)
	if !ok {
		return ""
	}
	return yokIpucu(ad, asilYazim(k, ascii))
}

// kaldirilanUyeIpucu Metin ve Liste üzerinde kaldırılmış bir metot ya da özellik adı için
// (sözlükte bu tiplerin üye kodlarından birinin "eski" adı) ipucunu döndürür.
func kaldirilanUyeIpucu(ad string) string {
	k, ascii, ok := sozluk.EskiBul(sozluk.AlanUye, ad, cekirdekUyesi)
	if !ok {
		return ""
	}
	return yokIpucu(ad, asilYazim(k, ascii))
}

// asilYazim eski adın yerine önerilecek yazımdır: eski ad Türkçe karaktersiz yazılmışsa
// yeni adın Türkçe karaktersiz biçimi.
func asilYazim(k sozluk.Kod, ascii bool) string {
	if ascii {
		return sozluk.ASCIIBicim(k)
	}
	return sozluk.Ad(k)
}

func yokIpucu(ad, asil string) string {
	return fmt.Sprintf(" ('%s' TR'de yok; '%s' yazmak mı istediniz?)", ad, asil)
}

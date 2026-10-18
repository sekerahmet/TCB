package vm

import (
	"fmt"
	"strings"

	"tr_dili/pkg/sozluk"
)

// Yorumlayıcının kaydettiği adlar sözlükten gelir (pkg/sozluk). Adlar kayıt sırasında
// (VM kurulurken) bir kez okunur; çalışma anında sözlüğe bakılmaz.

// RegisterBuiltinKod sözlükteki bir yerleşik kodunu (G/T/V/W) genel kapsama kaydeder.
// Ad sözlükten gelir; her işin tek adı vardır. Kod bir yerleşik kodu değilse ya da
// sözlükte yoksa bu bir geliştirici hatasıdır ve kayıt sırasında panikler.
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

// KaldirilanYerlesikIpucu ad bir yerleşiğin Türkçe karaktersiz (ASCII) biçimiyse (yazdir)
// ya da kaldırılmış bir yerleşik adıysa (sözlükte bir G/T/V/W kodunun "eski" adı) hata
// mesajına eklenecek ipucunu, değilse boş metni döndürür. Bu yazımlar sıradan adlardır
// (ör. değişken boy = 180); yalnızca tanımsız ad hatasında, alışkanlıkla yazılmış
// olabilecekleri için asıl (Türkçe) ad önerilir.
func KaldirilanYerlesikIpucu(ad string) string {
	k, ok := sozluk.Oneri(sozluk.AlanGenel, ad, sozluk.Kod.Yerlesik)
	if !ok {
		return ""
	}
	return yokIpucu(ad, sozluk.Ad(k))
}

// kaldirilanUyeIpucu Metin, Liste ve Sözlük üyelerinden birinin ASCII biçimi ya da
// kaldırılmış adı için ipucunu döndürür.
func kaldirilanUyeIpucu(ad string) string {
	k, ok := sozluk.Oneri(sozluk.AlanUye, ad, cekirdekUyesi)
	if !ok {
		return ""
	}
	return yokIpucu(ad, sozluk.Ad(k))
}

func yokIpucu(ad, asil string) string {
	return fmt.Sprintf(" ('%s' TR'de yok; '%s' yazmak mı istediniz?)", ad, asil)
}

// uyeOneriHatasi motor tiplerinde (Tensör, Veritabanı, WebSunucu ...) bulunamayan bir üye
// adı bu tipin bir üyesinin ASCII biçimi ya da kaldırılmış adıysa, hatayı asıl adı öneren
// bir hatayla değiştirir; değilse hatayı olduğu gibi döndürür. Önerinin bu tipte gerçekten
// geçerli olduğu asıl adla yeniden okunarak doğrulanır (üye okumak yan etkisizdir:
// metotlar bağlı değer olarak döner, çağrılmaz). Yalnızca hata yolunda çalışır.
func uyeOneriHatasi(pg PropertyGetter, obj Value, prop string, err error) error {
	k, ok := sozluk.Oneri(sozluk.AlanUye, prop, nil)
	if !ok {
		return err
	}
	if strings.Contains(err.Error(), "'"+sozluk.Ad(k)) {
		return err // tipin kendi hata mesajı asıl adı zaten öneriyor (ör. 'devrik()')
	}
	if _, e := pg.GetProperty(k, sozluk.Ad(k)); e != nil {
		return err
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", obj.Type(), prop, yokIpucu(prop, sozluk.Ad(k)))
}

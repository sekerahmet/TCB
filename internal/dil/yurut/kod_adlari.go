package yurut

import (
	"fmt"
	"slices"
	"strings"

	"tr_dili/internal/dil/ipucu"
	"tr_dili/internal/dil/kod"
)

// Yorumlayıcının kaydettiği adlar kod listesinden gelir (internal/dil/kod). Adlar kayıt sırasında
// (VM kurulurken) bir kez okunur; çalışma anında sözlüğe bakılmaz.

// RegisterBuiltinKod sözlükteki bir yerleşik kodunu (G/T/V/W) genel kapsama kaydeder.
// Ad sözlükten gelir; her işin tek adı vardır. Kod bir yerleşik kodu değilse ya da
// sözlükte yoksa bu bir geliştirici hatasıdır ve kayıt sırasında panikler.
func (v *VM) RegisterBuiltinKod(kd kod.Kod, fn BuiltinFunc) *BuiltinVal {
	return v.YerlesikTanimla(kd, &BuiltinVal{Name: kod.Ad(kd), Fn: fn})
}

// AdlisizKaydet adlı argüman almayan bir yerleşiği kaydeder: adlı argümanla çağrı
// gövdeye girilmeden reddedilir (bkz. BuiltinVal.AdliAlmaz).
func (v *VM) AdlisizKaydet(kd kod.Kod, fn BuiltinFunc) *BuiltinVal {
	b := v.RegisterBuiltinKod(kd, fn)
	b.AdliAlmaz = true
	return b
}

// AdliAlmazHatasi adlı argüman almayan bir yerleşiğe ya da metoda adlı argüman verildiğini
// bildirir; biçim, adlı argüman alan yerleşiklerin bilinmeyen ad hatasıyla aynıdır.
func AdliAlmazHatasi(ad string, named map[string]Value) error {
	adlar := make([]string, 0, len(named))
	for a := range named {
		adlar = append(adlar, a)
	}
	slices.Sort(adlar)
	return fmt.Errorf("%s() '%s' adında bir parametre almaz (adlı argüman almaz)", ad, adlar[0])
}

// YerlesikTanimla hazır bir yerleşik değerini (adı kodun adı olmalı) kodun kabul edilen
// yazımıyla (kod.Yazimlar: yalnızca ad) genel kapsama bağlar (ör. tensör motorunun eski
// yerleşiği saran değeri).
func (v *VM) YerlesikTanimla(kd kod.Kod, b *BuiltinVal) *BuiltinVal {
	if !kd.Yerlesik() {
		panic(fmt.Sprintf("yerleşik kaydı: %s bir yerleşik kodu (G/T/V/W, x9xx dışı) değil", kd))
	}
	for _, yazim := range kod.Yazimlar(kd) {
		v.GlobalEnv.Define(yazim, b, true)
	}
	return b
}

// SabitTanimla bir yerleşik sabiti (tanim.json'da "sabit" türü; ör. pi) değeriyle genel
// kapsama bağlar. Sabit çağrılmaz, değer olarak okunur; yeniden atanamaz.
func (v *VM) SabitTanimla(kd kod.Kod, deger Value) {
	if !kd.Yerlesik() {
		panic(fmt.Sprintf("sabit kaydı: %s bir yerleşik kodu (G/T/V/W, x9xx dışı) değil", kd))
	}
	if v.yerlesikSabitler == nil {
		v.yerlesikSabitler = map[string]Value{}
	}
	for _, yazim := range kod.Yazimlar(kd) {
		v.GlobalEnv.Define(yazim, deger, true)
		v.yerlesikSabitler[yazim] = deger
	}
}

// YerlesikSabitMi ad kapsamda val değeriyle duran bir yerleşik sabit mi (kullanıcı aynı adla
// bir değişken tanımladıysa değeri farklıdır ve yanlış döner)?
func (v *VM) YerlesikSabitMi(ad string, val Value) bool {
	s, ok := v.yerlesikSabitler[ad]
	return ok && s == val
}

// Yerlesik kodun genel kapsamda kayıtlı yerleşiğini verir; yoksa nil.
func (v *VM) Yerlesik(kd kod.Kod) *BuiltinVal {
	b, _ := v.GlobalEnv.yerelDeger(kod.Ad(kd)).(*BuiltinVal)
	return b
}

// kaldirilanUyeIpucu Metin, Liste ve Sözlük üyelerinden birinin ASCII biçimi ya da
// kaldırılmış adı için ipucunu döndürür.
func kaldirilanUyeIpucu(ad string) string {
	k, ok := kod.Oneri(kod.AlanUye, ad, cekirdekUyesi)
	if !ok {
		return ""
	}
	return ipucu.YokIpucu(ad, kod.Ad(k))
}

// uyeOneriHatasi motor tiplerinde (Tensör, Parçalayıcı, VeriKümesi ...) bulunamayan bir üye
// adı bu tipin bir üyesinin ASCII biçimi ya da kaldırılmış adıysa, hatayı asıl adı öneren
// bir hatayla değiştirir; değilse hatayı olduğu gibi döndürür. Önerinin bu tipte gerçekten
// geçerli olduğu asıl adla yeniden okunarak doğrulanır (üye okumak yan etkisizdir:
// metotlar bağlı değer olarak döner, çağrılmaz). Yalnızca hata yolunda çalışır.
func uyeOneriHatasi(pg PropertyGetter, obj Value, prop string, err error) error {
	k, ok := kod.Oneri(kod.AlanUye, prop, nil)
	if !ok {
		return err
	}
	if strings.Contains(err.Error(), "'"+kod.Ad(k)) {
		return err // tipin kendi hata mesajı asıl adı zaten öneriyor (ör. 'devrik()')
	}
	if _, e := pg.GetProperty(k, kod.Ad(k)); e != nil {
		return err
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", obj.Type(), prop, ipucu.YokIpucu(prop, kod.Ad(k)))
}

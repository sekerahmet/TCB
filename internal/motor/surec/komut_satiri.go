package surec

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ortak/baglama"
)

// komut_satırı(seçenekler, açıklama: "", açıklamalar: {}) betiğin seçeneklerini
// varsayılanlarıyla tanımlar ve komut satırı argümanlarıyla doldurulmuş YENİ bir sözlük
// döndürür (verilen sözlük değişmez). Seçenek adları sözlüğün anahtarlarıdır (veri; kodlanmaz),
// tipi varsayılandan çıkar. Kurulum hataları (desteklenmeyen varsayılan, ayrılmış ad, ikinci
// çağrı) sıradan çalışma zamanı hatalarıdır; kullanıcının yazdığı argümandaki hatalar ve
// --yardım ise programı bitirir (yurut.CikisIstegi: 2 ya da 0 koduyla).

// tur bir seçeneğin değer tipidir.
type tur int

const (
	turTam tur = iota
	turOndalik
	turMetin
	turMantik
	turListe
	turYok // varsayılan yok: verilirse Metin
)

// secenek komut satırının bir seçeneğidir.
type secenek struct {
	ad         string
	tur        tur
	ogeTuru    tur // turListe'de öğelerin tipi
	varsayilan yurut.Value
	aciklama   string
	gizli      bool
}

// tipKodu seçenek tipinin (yardımda ve hatalarda gösterilen) tip kodudur.
func (t tur) tipKodu() kod.Kod {
	switch t {
	case turTam:
		return kod.A200
	case turOndalik:
		return kod.A201
	case turMantik:
		return kod.A203
	case turListe:
		return kod.A205
	}
	return kod.A202
}

// ayrilmisSecenekAdlari --yardım'ın adlarıdır; seçenek adı olamazlar.
var ayrilmisSecenekAdlari = []string{"yardım", "h"}

func (d *Durum) komutSatiri(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.G801)
	if err := baglama.AdliDenetle(fn, named, adAciklama, adAciklamalar); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G801, "1", len(args), "{seçenek: varsayılan, ...}")
	}
	sozluk, ok := args[0].(*yurut.DictVal)
	if !ok {
		return nil, yurut.ArgumanTipHatasi(kod.G801, 1, args[0], kod.A206)
	}
	// Kurulum önce denetlenir: tanımdaki hata, ikinci çağrıda da adıyla bildirilir.
	secenekler, err := secenekleriKur(m, fn, sozluk)
	if err != nil {
		return nil, err
	}
	aciklama, err := aciklamalariOku(fn, named, secenekler)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	cagrildi := d.cagrildi
	d.mu.Unlock()
	if cagrildi {
		return nil, fmt.Errorf("%s programda bir kez çağrılır; ilk çağrının döndürdüğü sözlüğü bir değişkende tutup kullanın", fn)
	}
	sonuc, err := argumanlariUygula(m, secenekler, aciklama)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.cagrildi, d.ayarlar = true, sonuc
	d.mu.Unlock()
	return sonuc, nil
}

// secenekleriKur sözlüğün anahtarlarını ve varsayılanlarını seçeneklere çevirir.
func secenekleriKur(m *yurut.VM, fn string, sozluk *yurut.DictVal) ([]*secenek, error) {
	var out []*secenek
	for _, k := range sozluk.Order {
		deger, ok := sozluk.Pairs[k]
		if !ok {
			continue
		}
		if anahtar := sozluk.KeyValue(k); anahtar != nil {
			if _, metin := anahtar.(*yurut.StringVal); !metin {
				return nil, fmt.Errorf("%s: seçenek adları %s olmalıdır, %s verildi", fn, kod.Ad(kod.A202), yurut.TipVeOzet(anahtar))
			}
		}
		if err := secenekAdiDenetle(fn, k); err != nil {
			return nil, err
		}
		s := &secenek{ad: k, varsayilan: deger, gizli: GizliAdMi(k)}
		var err error
		if s.tur, s.ogeTuru, err = varsayilanTuru(fn, k, deger); err != nil {
			return nil, err
		}
		if s.gizli && m != nil {
			gizliKaydet(m, deger)
		}
		out = append(out, s)
	}
	return out, nil
}

// secenekAdiDenetle seçenek adının komut satırında yazılabilir ve sonuç sözlüğünde
// okunabilir olduğunu denetler. Sözlüğün üye adları (uzunluk, anahtarlar ...) seçenek adı
// olamaz: ayar.uzunluk anahtarı değil eleman sayısını verirdi.
func secenekAdiDenetle(fn, ad string) error {
	if ad == "" || strings.HasPrefix(ad, "-") || strings.ContainsFunc(ad, func(r rune) bool { return r == '=' || unicode.IsSpace(r) }) {
		return fmt.Errorf("%s: %q seçenek adı olamaz (boş olamaz, - ile başlayamaz, boşluk ya da = içeremez)", fn, ad)
	}
	if slices.Contains(ayrilmisSecenekAdlari, ad) {
		return fmt.Errorf("%s: %q seçenek adı olamaz: --yardım ve -h seçenekleri listeler", fn, ad)
	}
	if k, ok := kod.Bul(kod.AlanUye, ad); ok {
		if tn, ok := kod.Tanimi(k); ok && slices.Contains(tn.Tipler(), kod.A206) {
			return fmt.Errorf("%s: %q seçenek adı olamaz: %s tipinin bir üyesidir, ayar.%s seçeneğin değerini değil üyeyi verirdi; başka bir ad seçin",
				fn, ad, kod.Ad(kod.A206), ad)
		}
	}
	return nil
}

// varsayilanTuru varsayılan değerden seçeneğin tipini çıkarır.
func varsayilanTuru(fn, ad string, v yurut.Value) (tur, tur, error) {
	if t, ok := duzTur(v); ok {
		return t, turMetin, nil
	}
	switch x := v.(type) {
	case *yurut.NullVal:
		return turYok, turMetin, nil
	case *yurut.ListVal:
		oge := turMetin
		for i, e := range x.Elements {
			t, ok := duzTur(e)
			if !ok {
				return 0, 0, fmt.Errorf("%s: '%s' seçeneğinin liste varsayılanı yalnızca aynı tipte %s öğeler içerebilir; %d. öğe %s",
					fn, ad, yurut.TipAdlari(kod.A200, kod.A201, kod.A202, kod.A203), i+1, yurut.TipVeOzet(e))
			}
			switch {
			case i == 0:
				oge = t
			case oge == t:
			case (oge == turTam && t == turOndalik) || (oge == turOndalik && t == turTam):
				oge = turOndalik
			default:
				return 0, 0, fmt.Errorf("%s: '%s' seçeneğinin liste varsayılanındaki öğeler aynı tipte olmalı (%d. öğe %s)", fn, ad, i+1, yurut.TipVeOzet(e))
			}
		}
		return turListe, oge, nil
	}
	return 0, 0, fmt.Errorf("%s: '%s' seçeneğinin varsayılanı %s, %s ya da %s olmalı; %s verildi",
		fn, ad, yurut.TipAdlari(kod.A200, kod.A201, kod.A202, kod.A203), kod.Ad(kod.A205), kod.Ad(kod.A042), yurut.TipVeOzet(v))
}

// duzTur Tam, Ondalık, Metin ya da Mantık değerin tipidir.
func duzTur(v yurut.Value) (tur, bool) {
	switch v.(type) {
	case *yurut.IntVal:
		return turTam, true
	case *yurut.FloatVal:
		return turOndalik, true
	case *yurut.StringVal:
		return turMetin, true
	case *yurut.BoolVal:
		return turMantik, true
	}
	return 0, false
}

// aciklamalariOku açıklama: ve açıklamalar: adlı argümanlarını okur; açıklamalar
// seçeneklere yazılır, programın açıklaması döner.
func aciklamalariOku(fn string, named map[string]yurut.Value, secenekler []*secenek) (string, error) {
	aciklama := ""
	if v, ok := baglama.AdliAl(named, adAciklama); ok {
		s, ok := v.(*yurut.StringVal)
		if !ok {
			return "", fmt.Errorf("%s: %s: %s olmalıdır, %s verildi", fn, adAciklama.TR, kod.Ad(kod.A202), yurut.TipVeOzet(v))
		}
		aciklama = s.Val
	}
	v, ok := baglama.AdliAl(named, adAciklamalar)
	if !ok {
		return aciklama, nil
	}
	sozluk, ok := v.(*yurut.DictVal)
	if !ok {
		return "", fmt.Errorf("%s: %s: %s olmalıdır (seçenek adı → açıklama), %s verildi", fn, adAciklamalar.TR, kod.Ad(kod.A206), yurut.TipVeOzet(v))
	}
	adlar := make([]string, len(secenekler))
	for i, s := range secenekler {
		adlar[i] = s.ad
	}
	for _, k := range sozluk.Order {
		deger, ok := sozluk.Pairs[k]
		if !ok {
			continue
		}
		i := slices.Index(adlar, k)
		if i < 0 {
			oneri := ""
			if o := turkce.EnYakin(k, adlar); o != "" {
				oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
			}
			return "", fmt.Errorf("%s: %s içindeki '%s' bir seçenek değil%s", fn, adAciklamalar.TR, sozluk.DisplayKey(k), oneri)
		}
		s, ok := deger.(*yurut.StringVal)
		if !ok {
			return "", fmt.Errorf("%s: %s içinde '%s' seçeneğinin açıklaması %s olmalıdır, %s verildi", fn, adAciklamalar.TR, k, kod.Ad(kod.A202), yurut.TipVeOzet(deger))
		}
		secenekler[i].aciklama = s.Val
	}
	return aciklama, nil
}

// gizliKaydet gizli bir seçeneğin (metin olan) değerini gizli değer süzgecine verir.
func gizliKaydet(m *yurut.VM, v yurut.Value) {
	if s, ok := v.(*yurut.StringVal); ok {
		gizliEkle(m, s.Val)
	}
}

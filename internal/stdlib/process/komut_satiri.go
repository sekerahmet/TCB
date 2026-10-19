package process

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
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
	varsayilan interp.Value
	aciklama   string
	gizli      bool
}

// tipKodu seçenek tipinin (yardımda ve hatalarda gösterilen) tip kodudur.
func (t tur) tipKodu() codes.Code {
	switch t {
	case turTam:
		return codes.A200
	case turOndalik:
		return codes.A201
	case turMantik:
		return codes.A203
	case turListe:
		return codes.A205
	}
	return codes.A202
}

// ayrilmisSecenekAdlari --yardım'ın adlarıdır; seçenek adı olamazlar.
var ayrilmisSecenekAdlari = []string{"yardım", "h"}

func (d *Durum) komutSatiri(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G801)
	if err := bind.CheckNamed(fn, named, adAciklama, adAciklamalar); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G801, "1", len(args), "{seçenek: varsayılan, ...}")
	}
	sozluk, ok := args[0].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.G801, 1, args[0], codes.A206)
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
func secenekleriKur(m *interp.VM, fn string, sozluk *interp.DictVal) ([]*secenek, error) {
	var out []*secenek
	for _, k := range sozluk.Order {
		deger, ok := sozluk.Pairs[k]
		if !ok {
			continue
		}
		if anahtar := sozluk.KeyValue(k); anahtar != nil {
			if _, metin := anahtar.(*interp.StringVal); !metin {
				return nil, fmt.Errorf("%s: seçenek adları %s olmalıdır, %s verildi", fn, codes.Name(codes.A202), interp.TypeAndSummary(anahtar))
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
	if k, ok := codes.Find(codes.NamespaceMember, ad); ok {
		if tn, ok := codes.DefinitionOf(k); ok && slices.Contains(tn.Types(), codes.A206) {
			return fmt.Errorf("%s: %q seçenek adı olamaz: %s tipinin bir üyesidir, ayar.%s seçeneğin değerini değil üyeyi verirdi; başka bir ad seçin",
				fn, ad, codes.Name(codes.A206), ad)
		}
	}
	return nil
}

// varsayilanTuru varsayılan değerden seçeneğin tipini çıkarır.
func varsayilanTuru(fn, ad string, v interp.Value) (tur, tur, error) {
	if t, ok := duzTur(v); ok {
		return t, turMetin, nil
	}
	switch x := v.(type) {
	case *interp.NullVal:
		return turYok, turMetin, nil
	case *interp.ListVal:
		oge := turMetin
		for i, e := range x.Elements {
			t, ok := duzTur(e)
			if !ok {
				return 0, 0, fmt.Errorf("%s: '%s' seçeneğinin liste varsayılanı yalnızca aynı tipte %s öğeler içerebilir; %d. öğe %s",
					fn, ad, interp.TypeNames(codes.A200, codes.A201, codes.A202, codes.A203), i+1, interp.TypeAndSummary(e))
			}
			switch {
			case i == 0:
				oge = t
			case oge == t:
			case (oge == turTam && t == turOndalik) || (oge == turOndalik && t == turTam):
				oge = turOndalik
			default:
				return 0, 0, fmt.Errorf("%s: '%s' seçeneğinin liste varsayılanındaki öğeler aynı tipte olmalı (%d. öğe %s)", fn, ad, i+1, interp.TypeAndSummary(e))
			}
		}
		return turListe, oge, nil
	}
	return 0, 0, fmt.Errorf("%s: '%s' seçeneğinin varsayılanı %s, %s ya da %s olmalı; %s verildi",
		fn, ad, interp.TypeNames(codes.A200, codes.A201, codes.A202, codes.A203), codes.Name(codes.A205), codes.Name(codes.A042), interp.TypeAndSummary(v))
}

// duzTur Tam, Ondalık, Metin ya da Mantık değerin tipidir.
func duzTur(v interp.Value) (tur, bool) {
	switch v.(type) {
	case *interp.IntVal:
		return turTam, true
	case *interp.FloatVal:
		return turOndalik, true
	case *interp.StringVal:
		return turMetin, true
	case *interp.BoolVal:
		return turMantik, true
	}
	return 0, false
}

// aciklamalariOku açıklama: ve açıklamalar: adlı argümanlarını okur; açıklamalar
// seçeneklere yazılır, programın açıklaması döner.
func aciklamalariOku(fn string, named map[string]interp.Value, secenekler []*secenek) (string, error) {
	aciklama := ""
	if v, ok := bind.GetNamed(named, adAciklama); ok {
		s, ok := v.(*interp.StringVal)
		if !ok {
			return "", fmt.Errorf("%s: %s: %s olmalıdır, %s verildi", fn, adAciklama.TR, codes.Name(codes.A202), interp.TypeAndSummary(v))
		}
		aciklama = s.Val
	}
	v, ok := bind.GetNamed(named, adAciklamalar)
	if !ok {
		return aciklama, nil
	}
	sozluk, ok := v.(*interp.DictVal)
	if !ok {
		return "", fmt.Errorf("%s: %s: %s olmalıdır (seçenek adı → açıklama), %s verildi", fn, adAciklamalar.TR, codes.Name(codes.A206), interp.TypeAndSummary(v))
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
			if o := turkish.Closest(k, adlar); o != "" {
				oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
			}
			return "", fmt.Errorf("%s: %s içindeki '%s' bir seçenek değil%s", fn, adAciklamalar.TR, sozluk.DisplayKey(k), oneri)
		}
		s, ok := deger.(*interp.StringVal)
		if !ok {
			return "", fmt.Errorf("%s: %s içinde '%s' seçeneğinin açıklaması %s olmalıdır, %s verildi", fn, adAciklamalar.TR, k, codes.Name(codes.A202), interp.TypeAndSummary(deger))
		}
		secenekler[i].aciklama = s.Val
	}
	return aciklama, nil
}

// gizliKaydet gizli bir seçeneğin (metin olan) değerini gizli değer süzgecine verir.
func gizliKaydet(m *interp.VM, v interp.Value) {
	if s, ok := v.(*interp.StringVal); ok {
		gizliEkle(m, s.Val)
	}
}

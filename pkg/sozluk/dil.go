package sozluk

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"tr_dili/pkg/turkce"
)

//go:embed tr.json
var trJSON []byte

// ASCIIKabul, adların Türkçe karakterleri ASCII'ye çevrilmiş biçimlerinin (yazdir,
// sonuc, degisken) adın kendisi gibi kabul edilip edilmediğini belirleyen TEK açma-kapama
// noktasıdır. Sözcük çözümleyici, yerleşik kaydı, üye çözümü, adlı argümanlar ve tip
// açıklamaları kabul edilen yazımları yalnızca Yazimlar/Bul üzerinden alır; ASCII
// biçimi başka hiçbir yerde türetilmez. Kapatıldığında ASCII biçimler sıradan (tanımsız)
// adlar olur; AsilAd ise ipucu için ASCII biçimden Türkçe biçimi bulmaya devam eder.
const ASCIIKabul = true

// Girdi dil dosyasındaki bir satırdır.
type Girdi struct {
	Kod Kod
	// Ad kodun bu dildeki adıdır.
	Ad string
	// Eski kaldırılmış adlardır: kabul edilmezler, yalnızca hata mesajında Ad önerilir.
	// Türkçe karakterli eski adların ASCII biçimleri de (işlev → islev) eski sayılır.
	Eski []string
	// Aile insanlar için kısa bir açıklamadır (grup, üyeler için tipler).
	Aile string
	// ASCII, türetilen ASCII biçimin bu ad için kabul edilip edilmediğidir (dil
	// dosyasında "ascii": false ile kapatılır; bugün yalnızca ASCII biçimi hiç kabul
	// edilmemiş birkaç adlı argüman için). ASCIIKabul kapalıysa etkisizdir.
	ASCII bool
}

// eskiEslesme bir eski adın hangi koda ait olduğu ve adın, eski bir adın türetilmiş
// ASCII biçimi olup olmadığıdır.
type eskiEslesme struct {
	kod   Kod
	ascii bool
}

// Dil bir dil dosyasından (tr.json) kurulan sözlüktür. Kurulduktan sonra değişmez;
// eşzamanlı okunabilir.
type Dil struct {
	// Kodu dilin ISO 639-1 kodudur ("tr"; gerekirse bölgeli: "pt-BR").
	Kodu string

	girdiler map[Kod]*Girdi
	sira     []Kod // dosyadaki sıra
	yazimlar map[Kod][]string
	adlar    [alanSayisi]map[string]Kod
	eskiler  [alanSayisi]map[string][]eskiEslesme
	asil     [alanSayisi]map[string]string
	asilTum  map[string]string
}

type girdiJSON struct {
	Ad    string   `json:"ad"`
	Eski  []string `json:"eski"`
	Aile  string   `json:"aile"`
	ASCII *bool    `json:"ascii"`
}

// Yukle bir dil dosyasını okur ve doğrular. Biçim hatası, yinelenen kod ya da bir ad
// alanında çakışan ad (Türkçe ya da ASCII biçimiyle, eski adlar dahil) varsa hepsini
// sayan bir hata döndürür.
func Yukle(kodu string, veri []byte) (*Dil, error) {
	d := &Dil{Kodu: kodu, girdiler: map[Kod]*Girdi{}, yazimlar: map[Kod][]string{}, asilTum: map[string]string{}}
	var hatalar []string
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Sprintf(bicim, a...)) }

	dec := json.NewDecoder(bytes.NewReader(veri))
	dec.DisallowUnknownFields()
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("sözlük %s: dosya bir JSON nesnesi ({ ... }) olmalı", kodu)
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("sözlük %s: %v", kodu, err)
		}
		anahtar, _ := t.(string)
		var g girdiJSON
		if err := dec.Decode(&g); err != nil {
			return nil, fmt.Errorf("sözlük %s: %s girdisi okunamadı: %v", kodu, anahtar, err)
		}
		k, err := KodCoz(anahtar)
		if err != nil {
			hata("%v", err)
			continue
		}
		if _, var_ := d.girdiler[k]; var_ {
			hata("%s iki kez tanımlı", k)
			continue
		}
		if k.Alan() == AlanYok {
			hata("%s: %c ailesi henüz kullanılmıyor", k, k.Harf())
			continue
		}
		if g.Ad == "" || strings.IndexFunc(g.Ad, unicode.IsSpace) >= 0 {
			hata("%s: ad boş olamaz ve boşluk içeremez (%q)", k, g.Ad)
			continue
		}
		gr := &Girdi{Kod: k, Ad: g.Ad, Eski: g.Eski, Aile: g.Aile, ASCII: g.ASCII == nil || *g.ASCII}
		for _, e := range g.Eski {
			if e == "" || strings.IndexFunc(e, unicode.IsSpace) >= 0 {
				hata("%s: eski ad boş olamaz ve boşluk içeremez (%q)", k, e)
			}
			if e == g.Ad || turkce.ASCII(e) == turkce.ASCII(g.Ad) {
				hata("%s: eski ad %q adın kendisi (ya da ASCII biçimi) olamaz", k, e)
			}
		}
		d.girdiler[k] = gr
		d.sira = append(d.sira, k)
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("sözlük %s: %v", kodu, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("sözlük %s: kapanan '}' sonrasında fazladan içerik var", kodu)
	}

	// Ad alanı tabloları. Kod sırasıyla kurulur ki hata mesajları kararlı olsun.
	for i := range d.adlar {
		d.adlar[i] = map[string]Kod{}
		d.eskiler[i] = map[string][]eskiEslesme{}
		d.asil[i] = map[string]string{}
	}
	sahibi := [alanSayisi]map[string]Kod{} // ad alanında ASCII biçime göre adın sahibi
	for i := range sahibi {
		sahibi[i] = map[string]Kod{}
	}
	for _, k := range d.Kodlar() {
		g := d.girdiler[k]
		al := k.Alan()
		ascii := turkce.ASCII(g.Ad)
		if onceki, ok := sahibi[al][ascii]; ok {
			hata("%s %q ile %s %q aynı ad alanında (%s) çakışıyor (ASCII biçimleri: %q)", onceki, d.girdiler[onceki].Ad, k, g.Ad, al, ascii)
			continue
		}
		sahibi[al][ascii] = k
		yaz := []string{g.Ad}
		if ASCIIKabul && g.ASCII && ascii != g.Ad {
			yaz = append(yaz, ascii)
		}
		d.yazimlar[k] = yaz
		for _, y := range yaz {
			d.adlar[al][y] = k
		}
		d.asil[al][g.Ad], d.asil[al][ascii] = g.Ad, g.Ad
		if _, ok := d.asilTum[ascii]; !ok {
			d.asilTum[ascii] = g.Ad
		}
		if _, ok := d.asilTum[g.Ad]; !ok {
			d.asilTum[g.Ad] = g.Ad
		}
	}
	for _, k := range d.Kodlar() {
		g := d.girdiler[k]
		al := k.Alan()
		for _, e := range g.Eski {
			yazimlar := []eskiEslesme{{kod: k}}
			if a := turkce.ASCII(e); a != e {
				yazimlar = append(yazimlar, eskiEslesme{kod: k, ascii: true})
			}
			for i, es := range yazimlar {
				ad := e
				if i == 1 {
					ad = turkce.ASCII(e)
				}
				if sahip, ok := sahibi[al][turkce.ASCII(ad)]; ok && (ad == d.girdiler[sahip].Ad || ad == turkce.ASCII(d.girdiler[sahip].Ad)) {
					hata("%s'in eski adı %q, %s %q adıyla (ya da ASCII biçimiyle) çakışıyor", k, ad, sahip, d.girdiler[sahip].Ad)
					continue
				}
				for _, o := range d.eskiler[al][ad] {
					if o.kod != k {
						hata("eski ad %q hem %s hem %s için kayıtlı; ipucu belirsiz olur", ad, o.kod, k)
					}
				}
				d.eskiler[al][ad] = append(d.eskiler[al][ad], es)
			}
		}
	}
	if len(hatalar) > 0 {
		return nil, errors.New("sözlük " + kodu + ":\n  " + strings.Join(hatalar, "\n  "))
	}
	return d, nil
}

// Kodlar dildeki bütün kodları artan sırayla verir.
func (d *Dil) Kodlar() []Kod {
	out := make([]Kod, 0, len(d.girdiler))
	for k := range d.girdiler {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Girdisi kodun dil dosyasındaki satırını verir.
func (d *Dil) Girdisi(k Kod) (Girdi, bool) {
	g, ok := d.girdiler[k]
	if !ok {
		return Girdi{}, false
	}
	return *g, true
}

// Ad kodun bu dildeki adıdır. Kod dil dosyasında yoksa bu bir geliştirici hatasıdır ve
// panikler (kodlar Go'da sabittir; dosyadan bir satır silinmişse program başlarken durur).
func (d *Dil) Ad(k Kod) string {
	g, ok := d.girdiler[k]
	if !ok {
		panic(fmt.Sprintf("sözlük %s: %s kodu dil dosyasında yok", d.Kodu, k))
	}
	return g.Ad
}

// Yazimlar kodun kabul edilen bütün yazımlarıdır: önce ad, ASCII biçimi kabul ediliyorsa
// (bkz. ASCIIKabul) ardından ASCII biçimi. Dönen dilim değiştirilmemelidir.
func (d *Dil) Yazimlar(k Kod) []string {
	y, ok := d.yazimlar[k]
	if !ok {
		panic(fmt.Sprintf("sözlük %s: %s kodu dil dosyasında yok", d.Kodu, k))
	}
	return y
}

// ASCIIBicim adın türetilmiş ASCII biçimidir (kabul edilip edilmediğinden bağımsız).
func (d *Dil) ASCIIBicim(k Kod) string { return turkce.ASCII(d.Ad(k)) }

// Bul bir yazımın (ad ya da kabul edilen ASCII biçimi) verilen ad alanındaki kodunu verir.
func (d *Dil) Bul(al Alan, yazim string) (Kod, bool) {
	if al >= alanSayisi {
		return Yok, false
	}
	k, ok := d.adlar[al][yazim]
	return k, ok
}

// EskiBul ad, verilen ad alanında kaldırılmış (eski) bir adsa onun yerine geçen kodu
// verir; uygun nil değilse yalnızca uygun(kod) doğru olan kodlar aranır. ascii, adın
// eski bir adın türetilmiş ASCII biçimi olduğunu söyler (ör. islev); ipucu bu durumda
// yeni adın ASCII biçimini önerebilir.
func (d *Dil) EskiBul(al Alan, ad string, uygun func(Kod) bool) (kod Kod, ascii bool, ok bool) {
	if al >= alanSayisi {
		return Yok, false, false
	}
	for _, e := range d.eskiler[al][ad] {
		if uygun == nil || uygun(e.kod) {
			return e.kod, e.ascii, true
		}
	}
	return Yok, false, false
}

// AsilAd bir adın (Türkçe ya da ASCII biçimiyle yazılmış) asıl Türkçe biçimini verir:
// AsilAd("yazdir") → "yazdır". Ad dilde yoksa olduğu gibi döner. Ad alanlarına bakmaz;
// bir ad alanına özgü sonuç için AsilAdAlanda kullanılır.
func (d *Dil) AsilAd(ad string) string {
	if a, ok := d.asilTum[ad]; ok {
		return a
	}
	return ad
}

// AsilAdAlanda AsilAd'ın yalnızca verilen ad alanına bakan biçimidir; ad o alanda yoksa
// ok yanlıştır.
func (d *Dil) AsilAdAlanda(al Alan, ad string) (string, bool) {
	if al >= alanSayisi {
		return "", false
	}
	a, ok := d.asil[al][ad]
	return a, ok
}

// etkin programın kullandığı dildir. Şimdilik yalnızca Türkçe vardır.
var etkin = func() *Dil {
	d, err := Yukle("tr", trJSON)
	if err != nil {
		panic(err)
	}
	return d
}()

// Etkin programın kullandığı dili verir.
func Etkin() *Dil { return etkin }

// TRVerisi gömülü Türkçe dil dosyasının içeriğidir (testler ve araçlar için).
func TRVerisi() []byte { return trJSON }

// Ad, Yazimlar, Bul, EskiBul, AsilAd ve Kodlar etkin dilin aynı adlı yöntemleridir.

func Ad(k Kod) string                   { return etkin.Ad(k) }
func Yazimlar(k Kod) []string           { return etkin.Yazimlar(k) }
func ASCIIBicim(k Kod) string           { return etkin.ASCIIBicim(k) }
func Bul(al Alan, y string) (Kod, bool) { return etkin.Bul(al, y) }
func Kodlar() []Kod                     { return etkin.Kodlar() }
func AsilAd(ad string) string           { return etkin.AsilAd(ad) }
func EskiBul(al Alan, ad string, uygun func(Kod) bool) (Kod, bool, bool) {
	return etkin.EskiBul(al, ad, uygun)
}

// UyeKodu bir metot ya da özellik adının (x.ad) kodudur; üye değilse Yok. Derleyici bunu
// derleme anında bir kez çağırır; çalışma anında yalnızca kod karşılaştırılır.
func UyeKodu(ad string) Kod {
	k, _ := etkin.adlar[AlanUye][ad]
	return k
}

// Adlar kodların adlarını aynı sırayla verir (öneri listeleri için).
func Adlar(kodlar ...Kod) []string {
	out := make([]string, len(kodlar))
	for i, k := range kodlar {
		out[i] = etkin.Ad(k)
	}
	return out
}

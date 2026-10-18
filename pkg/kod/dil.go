package kod

import (
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

//go:embed tr.json
var trJSON []byte

// ASCIIKabul, adların Türkçe karakterleri ASCII'ye çevrilmiş biçimlerinin (yazdir,
// sonuc, degisken) adın kendisi gibi kabul edilip edilmediğini belirleyen TEK açma-kapama
// noktasıdır ve KAPALIDIR: her işin tek yazımı vardır, dilin kelimeleri yalnızca Türkçe
// biçimleriyle geçerlidir. Sözcük çözümleyici, yerleşik kaydı, üye çözümü, adlı argümanlar
// ve tip açıklamaları kabul edilen yazımları yalnızca Yazimlar/Bul üzerinden alır; ASCII
// biçim başka hiçbir yerde kabul edilmez. ASCII biçimler sıradan (tanımsız) adlardır;
// Oneri, bir hata oluştuğunda ASCII biçimden asıl Türkçe adı önermek için onları tanır.
const ASCIIKabul = false

// Girdi dil dosyasındaki bir satırdır.
type Girdi struct {
	Kod Kod
	// Ad kodun bu dildeki adıdır.
	Ad string
	// Eski kaldırılmış adlardır: kabul edilmezler, yalnızca hata mesajında Ad önerilir. Her
	// işin tek adı vardır; eş anlamlı ya da eş yazımlı ikinci bir ad kabul edilmez.
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

// Dil bir dil dosyasından (tr.json) kurulan kod listesidir. Kurulduktan sonra değişmez;
// eşzamanlı okunabilir.
type Dil struct {
	// Kodu dilin ISO 639-1 kodudur ("tr"; gerekirse bölgeli: "pt-BR").
	Kodu string

	girdiler map[Kod]*Girdi
	// adDizi aile başına sayı → addır; Ad çalışma anında da harita araması yapmasın diye dizi.
	adDizi   [len(aileHarfleri)][]string
	kodlar   []Kod // artan sırada
	yazimlar map[Kod][]string
	adlar    [alanSayisi]map[string]Kod
	eskiler  [alanSayisi]map[string][]eskiEslesme
	asil     [alanSayisi]map[string]string // ad alanı başına ad ve ASCII biçimi → ad
}

// Yukle bir dil dosyasını okur ve doğrular. Biçim hatası, yinelenen kod ya da bir ad
// alanında çakışan ad (Türkçe ya da ASCII biçimiyle, eski adlar dahil) varsa hepsini
// sayan bir hata döndürür.
func Yukle(kodu string, veri []byte) (*Dil, error) {
	var hatalar []string
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Sprintf(bicim, a...)) }

	ham, err := dosyaCoz(veri)
	if err != nil {
		return nil, fmt.Errorf("kod listesi %s: dosya okunamadı (bir JSON nesnesi { \"G100\": { \"ad\": ... }, ... } olmalı): %v", kodu, err)
	}

	d := &Dil{Kodu: kodu, girdiler: make(map[Kod]*Girdi, len(ham)), yazimlar: make(map[Kod][]string, len(ham))}
	var alanBoyu [alanSayisi]int // ad alanı başına yazım sayısı (haritaların boyu için)
	var enBuyuk [len(aileHarfleri)]int
	for _, g := range ham {
		k, err := Coz(g.anahtar)
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
		if g.ad == "" || strings.IndexFunc(g.ad, unicode.IsSpace) >= 0 {
			hata("%s: ad boş olamaz ve boşluk içeremez (%q)", k, g.ad)
			continue
		}
		gr := &Girdi{Kod: k, Ad: g.ad, Eski: g.eski, Aile: g.aile, ASCII: g.ascii == nil || *g.ascii}
		for _, e := range g.eski {
			if e == "" || strings.IndexFunc(e, unicode.IsSpace) >= 0 {
				hata("%s: eski ad boş olamaz ve boşluk içeremez (%q)", k, e)
			}
			if e == g.ad || asciiBicim(e) == asciiBicim(g.ad) {
				hata("%s: eski ad %q adın kendisi (ya da ASCII biçimi) olamaz", k, e)
			}
		}
		d.girdiler[k] = gr
		d.kodlar = append(d.kodlar, k)
		alanBoyu[k.Alan()] += 2
		enBuyuk[int(k)/1000-1] = max(enBuyuk[int(k)/1000-1], k.Sayi())
	}
	sort.Slice(d.kodlar, func(i, j int) bool { return d.kodlar[i] < d.kodlar[j] })
	for i, n := range enBuyuk {
		if n > 0 {
			d.adDizi[i] = make([]string, n+1)
		}
	}

	// Ad alanı tabloları. Kod sırasıyla kurulur ki hata mesajları kararlı olsun.
	for i := range d.adlar {
		d.adlar[i] = make(map[string]Kod, alanBoyu[i])
		d.eskiler[i] = map[string][]eskiEslesme{}
		d.asil[i] = make(map[string]string, alanBoyu[i])
	}
	// sahibi: ad alanında ASCII biçime göre adın sahibi kod ve Türkçe biçimi.
	type sahip struct {
		kod Kod
		ad  string
	}
	sahibi := [alanSayisi]map[string]sahip{}
	for i := range sahibi {
		sahibi[i] = make(map[string]sahip, alanBoyu[i]/2)
	}
	for _, k := range d.kodlar {
		g := d.girdiler[k]
		al := k.Alan()
		ad := g.Ad
		ascii := asciiBicim(ad)
		if o, ok := sahibi[al][ascii]; ok {
			hata("%s %q ile %s %q aynı ad alanında (%s) çakışıyor (ASCII biçimleri: %q)", o.kod, o.ad, k, ad, al, ascii)
			continue
		}
		sahibi[al][ascii] = sahip{k, ad}
		yaz := []string{ad}
		if ASCIIKabul && g.ASCII && ascii != ad {
			yaz = append(yaz, ascii)
		}
		d.asil[al][ad], d.asil[al][ascii] = ad, ad
		d.yazimlar[k] = yaz
		d.adDizi[int(k)/1000-1][k.Sayi()] = g.Ad
		for _, y := range yaz {
			d.adlar[al][y] = k
		}
	}
	for _, k := range d.kodlar {
		g := d.girdiler[k]
		al := k.Alan()
		for _, e := range g.Eski {
			yazimlar := []string{e}
			if a := asciiBicim(e); a != e {
				yazimlar = append(yazimlar, a)
			}
			for i, ad := range yazimlar {
				if o, ok := sahibi[al][asciiBicim(ad)]; ok && (ad == o.ad || ad == asciiBicim(o.ad)) {
					hata("%s'in eski adı %q, %s %q adıyla (ya da ASCII biçimiyle) çakışıyor", k, ad, o.kod, o.ad)
					continue
				}
				for _, o := range d.eskiler[al][ad] {
					if o.kod != k {
						hata("eski ad %q hem %s hem %s için kayıtlı; ipucu belirsiz olur", ad, o.kod, k)
					}
				}
				d.eskiler[al][ad] = append(d.eskiler[al][ad], eskiEslesme{kod: k, ascii: i == 1})
			}
		}
	}
	if len(hatalar) > 0 {
		return nil, errors.New("kod listesi " + kodu + ":\n  " + strings.Join(hatalar, "\n  "))
	}
	return d, nil
}

// Kodlar dildeki bütün kodları artan sırayla verir.
func (d *Dil) Kodlar() []Kod { return append([]Kod(nil), d.kodlar...) }

// Girdisi kodun dil dosyasındaki satırını verir.
func (d *Dil) Girdisi(k Kod) (Girdi, bool) {
	g, ok := d.girdiler[k]
	if !ok {
		return Girdi{}, false
	}
	return *g, true
}

// Ad kodun bu dildeki adıdır (dizi indekslemesi; harita araması yok). Kod dil dosyasında yoksa bu bir geliştirici hatasıdır ve
// panikler (kodlar Go'da sabittir; dosyadan bir satır silinmişse program başlarken durur).
func (d *Dil) Ad(k Kod) string {
	if f, n := int(k)/1000-1, k.Sayi(); f >= 0 && f < len(d.adDizi) && n < len(d.adDizi[f]) {
		if a := d.adDizi[f][n]; a != "" {
			return a
		}
	}
	panic(fmt.Sprintf("kod listesi %s: %s kodu dil dosyasında yok", d.Kodu, k))
}

// Yazimlar kodun kabul edilen bütün yazımlarıdır. Her işin tek yazımı olduğundan bu
// yalnızca addır (ASCIIKabul açılırsa ardından ASCII biçimi gelir). Dönen dilim
// değiştirilmemelidir.
func (d *Dil) Yazimlar(k Kod) []string {
	y, ok := d.yazimlar[k]
	if !ok {
		panic(fmt.Sprintf("kod listesi %s: %s kodu dil dosyasında yok", d.Kodu, k))
	}
	return y
}

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

// AsilAdAlanda bir adın (Türkçe ya da ASCII biçimiyle yazılmış) verilen ad alanındaki
// asıl Türkçe biçimini verir: AsilAdAlanda(AlanGenel, "yazdir") → "yazdır". Ad o alanda
// yoksa ok yanlıştır. Aynı ASCII yazım farklı ad alanlarında farklı adlara ait olabilir
// (genel adlarda "tur" G201 tip'in eski adıdır, üyelerde Y402 tür'ün ASCII biçimidir);
// bu yüzden her zaman ad alanıyla sorulur.
func (d *Dil) AsilAdAlanda(al Alan, ad string) (string, bool) {
	if al >= alanSayisi {
		return "", false
	}
	a, ok := d.asil[al][ad]
	return a, ok
}

// Oneri, verilen ad alanında geçerli olmayan bir yazım için önerilecek kodu verir: yazım
// orada bir adın Türkçe karakterleri ASCII'ye çevrilmiş biçimiyse (yazdir → yazdır) ya da
// kaldırılmış (eski) bir adsa ya da onun ASCII biçimiyse (işlev, islev → fonksiyon).
// uygun nil değilse yalnızca uygun(kod) doğru olan kodlar önerilir. Önerilen her zaman
// kodun adıdır (Türkçe biçimi); ASCII biçim hiçbir zaman önerilmez. Yazım ad alanında
// geçerli bir adsa öneri yoktur.
func (d *Dil) Oneri(al Alan, yazim string, uygun func(Kod) bool) (Kod, bool) {
	if al >= alanSayisi {
		return Yok, false
	}
	if _, gecerli := d.adlar[al][yazim]; gecerli {
		return Yok, false
	}
	if ad, ok := d.asil[al][yazim]; ok && ad != yazim {
		if k, ok := d.adlar[al][ad]; ok && (uygun == nil || uygun(k)) {
			return k, true
		}
	}
	if k, _, ok := d.EskiBul(al, yazim, uygun); ok {
		return k, true
	}
	return Yok, false
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

// Ad, Yazimlar, Bul, EskiBul, Oneri, AsilAdAlanda ve Kodlar etkin dilin aynı adlı
// yöntemleridir.

func Ad(k Kod) string                   { return etkin.Ad(k) }
func Yazimlar(k Kod) []string           { return etkin.Yazimlar(k) }
func Bul(al Alan, y string) (Kod, bool) { return etkin.Bul(al, y) }
func Kodlar() []Kod                     { return etkin.Kodlar() }
func AsilAdAlanda(al Alan, ad string) (string, bool) {
	return etkin.AsilAdAlanda(al, ad)
}
func EskiBul(al Alan, ad string, uygun func(Kod) bool) (Kod, bool, bool) {
	return etkin.EskiBul(al, ad, uygun)
}
func Oneri(al Alan, yazim string, uygun func(Kod) bool) (Kod, bool) {
	return etkin.Oneri(al, yazim, uygun)
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

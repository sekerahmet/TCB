package kod

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Dilden bağımsız tanımlar (tanim.json): her kodun türü (anahtar kelime, tip, fonksiyon,
// metot ...), metot ve özelliklerde geçerli olduğu tipler, imzası (konumlu parametrelerin
// sırası ve tipi, adlı argümanların kodu, tipi ve varsayılanı), dönüş tipi, ilişkili kodlar
// ve kaldırılan kodlarda işin taşındığı yeni kod. Adlar burada geçmez: imzalar tip ve kod
// listesidir, adları dil dosyası (tr.json) verir. docs/KODLAR.md bu iki dosyadan üretilir.
//
// Dosyanın biçimi (her girdi tek satır ya da kısa bir blok, kodlar artan sırada):
//
//	"G305": { "tür": "fonksiyon", "param": ["A200", "A200?"], "dönüş": "A205" },
//	"G400": { "tür": "fonksiyon", "param": ["A205|A212"], "adlı": [["T902", "A200"], ["T906", "A203", false]], "dönüş": "A200|A201|A212" },
//	"Y001": { "tür": "özellik", "tipler": ["A202", "A205", "A206"], "dönüş": "A200" },
//	"Y103": { "tür": "metot", "imzalar": { "A202": { "param": ["A202?"], "dönüş": "A205" }, "A212": { ... } } },
//	"G200": { "tür": "fonksiyon", "taşındı": "Y001" },
//
// Tip ifadesi tip kodlarının "|" ile birleşimidir (A200|A201: Tam ya da Ondalık); konumlu
// parametrede sonek "?" isteğe bağlı, "*" sıfır ya da çok, "+" bir ya da çok demektir. Tip
// kodları A2xx'tir; tip adı bir oluşturucunun adı olan nesneler (AdamW, SGD) için
// oluşturucunun kodu (T400, T401) kullanılır. Adlı argüman [kod, tip, varsayılan]
// üçlüsüdür; varsayılan bir JSON değeridir (null: yok) ve tek bir varsayılan yoksa
// yazılmaz. "taşındı", kod kaldırılıp işi başka bir biçime taşındığında yeni kodu verir;
// eski kodun numarası bir daha kullanılmaz ve adı yalnızca ipucu için dil dosyasında kalır.

//go:embed tanim.json
var tanimJSON []byte

// TanimVerisi gömülü tanim.json'un içeriğidir (testler ve araçlar için).
func TanimVerisi() []byte { return tanimJSON }

// Tur bir kodun türüdür.
type Tur string

const (
	TurAnahtarKelime Tur = "anahtar_kelime"
	TurBaglamsal     Tur = "bağlamsal"
	TurTip           Tur = "tip"
	TurFonksiyon     Tur = "fonksiyon"
	TurMetot         Tur = "metot"
	TurOzellik       Tur = "özellik"
	TurAdliArguman   Tur = "adlı_argüman"
)

// BeklenenTur kodun ailesinden ve grubundan çıkan türdür (Y üyelerinde metot ya da
// özellik olabileceği için TurMetot döner; ikisi de geçerlidir).
func (k Kod) BeklenenTur() Tur {
	switch {
	case k.Harf() == 'A' && k.Grup() == 0:
		return TurAnahtarKelime
	case k.Harf() == 'A' && k.Grup() == 1:
		return TurBaglamsal
	case k.Harf() == 'A':
		return TurTip
	case k.Harf() == 'Y':
		return TurMetot
	case k.Alan() == AlanParametre:
		return TurAdliArguman
	}
	return TurFonksiyon
}

// TipIfadesi bir parametrenin ya da dönüşün tipidir: tiplerden biri (boşsa belirtilmemiş).
type TipIfadesi struct {
	Tipler []Kod
	// Istege isteğe bağlı (?), Cok sıfır ya da çok (*), EnAzBir bir ya da çok (+) argümandır.
	Istege, Cok, EnAzBir bool
}

// Bos tip ifadesinin belirtilmemiş olup olmadığıdır.
func (t TipIfadesi) Bos() bool { return len(t.Tipler) == 0 }

// Adli bir adlı argümandır.
type Adli struct {
	Kod Kod
	Tip TipIfadesi
	// Varsayilan JSON değeridir (bool, float64, string ya da nil = yok); VarsayilanVar
	// yanlışsa tek bir varsayılan yoktur.
	Varsayilan    any
	VarsayilanVar bool
}

// Imza konumlu parametreler, adlı argümanlar ve dönüş tipidir.
type Imza struct {
	Param []TipIfadesi
	Adli  []Adli
	Donus TipIfadesi
}

// TipImzasi bir üyenin bir tipteki imzasıdır.
type TipImzasi struct {
	Tip  Kod
	Imza Imza
}

// Tanim tanim.json'daki bir girdidir.
type Tanim struct {
	Kod Kod
	Tur Tur
	// Imza fonksiyonların ve tipten bağımsız imzalı üyelerin imzasıdır.
	Imza Imza
	// Uyeler metot ve özelliklerin geçerli olduğu tipler ve her tipteki imzasıdır (tip
	// sırasıyla).
	Uyeler   []TipImzasi
	Iliskili []Kod
	// Tasindi, kod kaldırılıp işi taşındıysa yeni koddur.
	Tasindi Kod
}

// Tipler üyenin geçerli olduğu tiplerdir.
func (t Tanim) Tipler() []Kod {
	out := make([]Kod, len(t.Uyeler))
	for i, u := range t.Uyeler {
		out[i] = u.Tip
	}
	return out
}

// TipeOzel üyenin imzası tipe göre değişiyor mu ("imzalar" ile yazılmış mı)?
func (t Tanim) TipeOzel() bool {
	return len(t.Uyeler) > 1 && !imzaEsit(t.Uyeler[0].Imza, t.Uyeler[1:]...)
}

func imzaEsit(a Imza, digerleri ...TipImzasi) bool {
	for _, d := range digerleri {
		if fmt.Sprint(a) != fmt.Sprint(d.Imza) {
			return false
		}
	}
	return true
}

var (
	tanimOnce   sync.Once
	tanimlar    map[Kod]Tanim
	tanimHatasi error
	tanimSirasi []Kod
)

// Tanimi kodun tanımını verir. Tanım dosyası ilk çağrıda okunur (yorumlayıcı çalışırken
// gerekmez; araçlar, testler ve hata yolu kullanır).
func Tanimi(k Kod) (Tanim, bool) {
	tanimYukle()
	t, ok := tanimlar[k]
	return t, ok
}

// TanimHatasi tanim.json okunurken oluşan biçim hatasıdır (yoksa nil).
func TanimHatasi() error {
	tanimYukle()
	return tanimHatasi
}

// TanimKodlari tanım dosyasındaki bütün kodlardır (artan sırada).
func TanimKodlari() []Kod {
	tanimYukle()
	return append([]Kod(nil), tanimSirasi...)
}

func tanimYukle() {
	tanimOnce.Do(func() {
		tanimlar, tanimSirasi, tanimHatasi = TanimCoz(tanimJSON)
	})
}

type hamImza struct {
	Param []string            `json:"param"`
	Adli  [][]json.RawMessage `json:"adlı"`
	Donus string              `json:"dönüş"`
}

type hamTanim struct {
	Tur      string              `json:"tür"`
	Tipler   []string            `json:"tipler"`
	Param    []string            `json:"param"`
	Adli     [][]json.RawMessage `json:"adlı"`
	Donus    string              `json:"dönüş"`
	Imzalar  map[string]hamImza  `json:"imzalar"`
	Iliskili []string            `json:"ilişkili"`
	Tasindi  string              `json:"taşındı"`
}

// TanimCoz bir tanım dosyasını çözer; biçim hatalarının hepsini tek hatada toplar.
func TanimCoz(veri []byte) (map[Kod]Tanim, []Kod, error) {
	var ham map[string]json.RawMessage
	if err := json.Unmarshal(veri, &ham); err != nil {
		return nil, nil, fmt.Errorf("tanim.json: %v", err)
	}
	var hatalar []string
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Sprintf(bicim, a...)) }
	out := make(map[Kod]Tanim, len(ham))
	var sira []Kod
	for anahtar, satir := range ham {
		k, err := Coz(anahtar)
		if err != nil {
			hata("%v", err)
			continue
		}
		var h hamTanim
		dec := json.NewDecoder(bytes.NewReader(satir))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&h); err != nil {
			hata("%s: %v", k, err)
			continue
		}
		t := Tanim{Kod: k, Tur: Tur(h.Tur)}
		kodlar := func(alan string, ss []string) []Kod {
			var ks []Kod
			for _, s := range ss {
				x, err := Coz(s)
				if err != nil {
					hata("%s %s: %v", k, alan, err)
					continue
				}
				ks = append(ks, x)
			}
			return ks
		}
		imza := func(param []string, adli [][]json.RawMessage, donus string) Imza {
			var im Imza
			for _, p := range param {
				ti, err := TipCoz(p, true)
				if err != nil {
					hata("%s param: %v", k, err)
				}
				im.Param = append(im.Param, ti)
			}
			for _, a := range adli {
				ad, err := adliCoz(a)
				if err != nil {
					hata("%s adlı: %v", k, err)
					continue
				}
				im.Adli = append(im.Adli, ad)
			}
			if donus != "" {
				ti, err := TipCoz(donus, false)
				if err != nil {
					hata("%s dönüş: %v", k, err)
				}
				im.Donus = ti
			}
			return im
		}
		t.Imza = imza(h.Param, h.Adli, h.Donus)
		for _, tip := range kodlar("tipler", h.Tipler) {
			t.Uyeler = append(t.Uyeler, TipImzasi{Tip: tip, Imza: t.Imza})
		}
		if len(h.Imzalar) > 0 {
			if len(h.Tipler) > 0 || len(h.Param) > 0 || len(h.Adli) > 0 || h.Donus != "" {
				hata("%s: \"imzalar\" varken \"tipler\", \"param\", \"adlı\" ve \"dönüş\" yazılmaz", k)
			}
			for s, hi := range h.Imzalar {
				tip, err := Coz(s)
				if err != nil {
					hata("%s imzalar: %v", k, err)
					continue
				}
				t.Uyeler = append(t.Uyeler, TipImzasi{Tip: tip, Imza: imza(hi.Param, hi.Adli, hi.Donus)})
			}
			sort.Slice(t.Uyeler, func(i, j int) bool { return t.Uyeler[i].Tip < t.Uyeler[j].Tip })
		}
		t.Iliskili = kodlar("ilişkili", h.Iliskili)
		if h.Tasindi != "" {
			if y := kodlar("taşındı", []string{h.Tasindi}); len(y) == 1 {
				t.Tasindi = y[0]
			}
		}
		out[k] = t
		sira = append(sira, k)
	}
	sort.Slice(sira, func(i, j int) bool { return sira[i] < sira[j] })
	if len(hatalar) > 0 {
		sort.Strings(hatalar)
		return out, sira, fmt.Errorf("tanim.json:\n  %s", strings.Join(hatalar, "\n  "))
	}
	return out, sira, nil
}

// TipCoz "A200|A201?" biçimindeki bir tip ifadesini çözer; sonekler yalnızca konumlu
// parametrelerde (parametre doğruyken) geçerlidir.
func TipCoz(s string, parametre bool) (TipIfadesi, error) {
	var t TipIfadesi
	if parametre && s != "" {
		switch s[len(s)-1] {
		case '?':
			t.Istege, s = true, s[:len(s)-1]
		case '*':
			t.Cok, s = true, s[:len(s)-1]
		case '+':
			t.EnAzBir, s = true, s[:len(s)-1]
		}
	}
	for _, p := range strings.Split(s, "|") {
		k, err := Coz(p)
		if err != nil {
			return t, fmt.Errorf("tip ifadesi %q: %v", s, err)
		}
		t.Tipler = append(t.Tipler, k)
	}
	return t, nil
}

func adliCoz(a []json.RawMessage) (Adli, error) {
	var ad Adli
	if len(a) < 2 || len(a) > 3 {
		return ad, fmt.Errorf("adlı argüman [kod, tip] ya da [kod, tip, varsayılan] olmalı")
	}
	var ks, ts string
	if err := json.Unmarshal(a[0], &ks); err != nil {
		return ad, err
	}
	if err := json.Unmarshal(a[1], &ts); err != nil {
		return ad, err
	}
	k, err := Coz(ks)
	if err != nil {
		return ad, err
	}
	ti, err := TipCoz(ts, false)
	if err != nil {
		return ad, err
	}
	ad.Kod, ad.Tip = k, ti
	if len(a) == 3 {
		if err := json.Unmarshal(a[2], &ad.Varsayilan); err != nil {
			return ad, err
		}
		ad.VarsayilanVar = true
	}
	return ad, nil
}

// UyeYazimi bir üyenin yazımını verir: x.uzunluk, liste.filtrele(...), liste.sırala().
// Üye tek bir tipteyse alıcı o tipin küçük harfli adıdır (Liste → liste), değilse x.
// Hata mesajları ve belgeler taşınan kodların yeni biçimini bununla gösterir.
func UyeYazimi(y Kod) string {
	t, _ := Tanimi(y)
	a := "x"
	if len(t.Uyeler) == 1 {
		a = strings.ToLowerSpecial(unicode.TurkishCase, Ad(t.Uyeler[0].Tip))
	}
	s := a + "." + Ad(y)
	if t.Tur != TurMetot {
		return s
	}
	for _, u := range t.Uyeler {
		if len(u.Imza.Param) > 0 {
			return s + "(...)"
		}
	}
	return s + "()"
}

// TanimDenetle tanımların dil dosyasıyla tutarlılığını denetler: her kodun (taşınmışlar
// dahil) tanımı var, tanımdaki her kod dil dosyasında var, tür kodun ailesine uygun, tip
// ifadeleri tip kodlarından (A2xx ya da tip adı bir oluşturucunun adı olan T400/T401)
// oluşur, adlı argüman kodları x9xx'tir, ilişkili ve taşındı kodları vardır, taşınan kod
// canlı ve başka bir ad alanındadır, dil dosyasındaki taşınma bilgisi tanımla aynıdır.
func TanimDenetle(d *Dil) []error {
	var hatalar []error
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Errorf(bicim, a...)) }
	if err := TanimHatasi(); err != nil {
		hata("%v", err)
	}
	dilde := func(k Kod) bool { _, ok := d.girdiler[k]; return ok }
	tumu := append(d.Kodlar(), d.Tasinanlar()...)
	for _, k := range tumu {
		if _, ok := Tanimi(k); !ok {
			hata("%s (%s) dil dosyasında var, tanim.json'da tanımı yok", k, d.Ad(k))
		}
	}
	tipKodu := func(k Kod) bool {
		if k == T400 || k == T401 {
			return true
		}
		g, ok := d.girdiler[k]
		return ok && k.Harf() == 'A' && k.Grup() >= 2 && g.Tasindi == Yok
	}
	tipDenetle := func(k Kod, yer string, t TipIfadesi) {
		for _, x := range t.Tipler {
			if !tipKodu(x) {
				hata("%s %s: %s bir tip kodu değil", k, yer, x)
			}
		}
	}
	imzaDenetle := func(k Kod, yer string, im Imza) {
		for i, p := range im.Param {
			tipDenetle(k, fmt.Sprintf("%s%d. parametre", yer, i+1), p)
			if (p.Cok || p.EnAzBir) && i != len(im.Param)-1 {
				hata("%s %s: '*' ya da '+' yalnızca son parametrede olabilir", k, yer)
			}
		}
		for _, a := range im.Adli {
			if a.Kod.Alan() != AlanParametre || !dilde(a.Kod) {
				hata("%s %sadlı argüman %s bir adlı argüman kodu (x9xx) değil", k, yer, a.Kod)
			}
			tipDenetle(k, yer+"adlı argüman", a.Tip)
		}
		if !im.Donus.Bos() {
			tipDenetle(k, yer+"dönüş", im.Donus)
		}
	}
	for _, k := range TanimKodlari() {
		t, _ := Tanimi(k)
		g, ok := d.girdiler[k]
		if !ok {
			hata("%s tanim.json'da var, dil dosyasında yok", k)
			continue
		}
		bek := k.BeklenenTur()
		switch {
		case bek == TurMetot && t.Tur != TurMetot && t.Tur != TurOzellik:
			hata("%s: tür %q olamaz; üyeler (Y) metot ya da özelliktir", k, t.Tur)
		case bek != TurMetot && t.Tur != bek:
			hata("%s: tür %q, ama kodun ailesi %q gerektirir", k, t.Tur, bek)
		}
		if g.Tasindi != t.Tasindi {
			hata("%s: dil dosyasının taşınma bilgisi (%s) tanımla (%s) aynı değil; go run ./araclar/kod_sabitleri", k, g.Tasindi, t.Tasindi)
		}
		if t.Tasindi != Yok {
			if len(t.Uyeler) > 0 || len(t.Imza.Param) > 0 || len(t.Imza.Adli) > 0 || !t.Imza.Donus.Bos() {
				hata("%s taşındı; imza yazılmaz (imza yeni kodda, %s)", k, t.Tasindi)
			}
			y, ok := d.girdiler[t.Tasindi]
			if !ok || y.Tasindi != Yok {
				hata("%s %s'e taşındı, ama %s canlı bir kod değil", k, t.Tasindi, t.Tasindi)
			} else if t.Tasindi.Alan() == k.Alan() {
				hata("%s %s'e taşındı, ama ikisi aynı ad alanında (%s); taşıma biçim değiştirir (fonksiyon ↔ üye)", k, t.Tasindi, k.Alan())
			}
			continue
		}
		switch t.Tur {
		case TurMetot, TurOzellik:
			if len(t.Uyeler) == 0 {
				hata("%s: üyenin geçerli olduğu tipler (tipler ya da imzalar) yazılmalı", k)
			}
			for _, u := range t.Uyeler {
				if !tipKodu(u.Tip) {
					hata("%s: %s bir tip kodu değil", k, u.Tip)
				}
				imzaDenetle(k, u.Tip.String()+" ", u.Imza)
				if t.Tur == TurOzellik && (len(u.Imza.Param) > 0 || len(u.Imza.Adli) > 0) {
					hata("%s: özellik parametre almaz", k)
				}
			}
		case TurFonksiyon:
			imzaDenetle(k, "", t.Imza)
		default:
			if len(t.Imza.Param) > 0 || len(t.Imza.Adli) > 0 || !t.Imza.Donus.Bos() || len(t.Uyeler) > 0 {
				hata("%s: %s imza almaz", k, t.Tur)
			}
		}
		for _, x := range t.Iliskili {
			if !dilde(x) {
				hata("%s: ilişkili kod %s dil dosyasında yok", k, x)
			} else if x == k {
				hata("%s kendisiyle ilişkili yazılmış", k)
			}
		}
	}
	return hatalar
}

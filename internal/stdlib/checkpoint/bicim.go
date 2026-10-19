package checkpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Kontrol noktası dosya biçimi (sürüm 2; sürüm 1 de okunur: o sürümde modellerin yalnızca
// türev izleyen tensörleri kaydedilirdi, sürüm 2'de türev izlemeyenler de kaydedilir; dosya
// düzeni aynıdır). Dosyalar güvenilmeyen girdidir (başkasından
// gelen ya da yarım kopyalanmış bir koşu klasörü): her dosyanın boyu sınırlıdır, JSON katı
// çözülür (bilinmeyen alan, fazladan veri hata), sürüm, adım ve dosya adları birbiriyle
// karşılaştırılır, her dosyanın SHA-256 özeti tutmalıdır. Biçimde yalnızca veri (tensörler,
// sayılar, metinler) vardır; kod çalıştıran ya da yol çözen hiçbir alan yoktur.

const (
	bicimSurumu  = 2 // yazılan sürüm
	enEskiSurumu = 1 // okunabilen en eski sürüm

	klasorAdi     = "kontrol_noktalari" // <koşu>/kontrol_noktalari/
	sonDosyasi    = "son.json"          // en son kaydedilen noktayı gösterir (araçlar için)
	ozetDosyasi   = "ozet.json"         // noktanın dosyalarının bayt sayısı ve SHA-256 özeti (en son yazılır)
	durumDosyasi  = "durum.json"        // adım, program, ayarlar ve nesnelerin kaydı
	tensorDosyasi = "tensorler.trt"     // parametreler ve iyileştirici durumları (tensör_kaydet biçimi)
	noktaOneki    = "adim_"             // adim_0000001500
	geciciEki     = ".tmp-"             // yazılmakta olan nokta: adim_0000001500.tmp-<süreç>
	eskiEki       = ".eski-"            // yerine yenisi konan aynı adımlı nokta: adim_...eski-<süreç>

	// Boy sınırları (SECURITY.md): JSON dosyaları belleğe okunmadan önce sınanır.
	enFazlaKucukJSON = 64 << 10 // son.json, ozet.json
	enFazlaDurumJSON = 64 << 20 // durum.json (düz değerler ve okuyucu durumları)
	enFazlaNesne     = 1000     // kontrol_noktası sözlüğündeki nesne sayısı
	enFazlaNesneAdi  = 200      // nesne adının bayt uzunluğu
)

// noktaKalibi bir noktanın klasör adıdır: en az 10 basamaklı adım (sıralama sözlük sırasıyla
// da doğru olsun diye sıfırla doldurulur).
var noktaKalibi = regexp.MustCompile(`^adim_([0-9]{10,19})$`)

// noktaAdi adımın klasör adıdır.
func noktaAdi(adim int64) string { return fmt.Sprintf("%s%010d", noktaOneki, adim) }

// noktaAdimi klasör adından adımı okur; ad bir nokta adı değilse ok yanlış.
func noktaAdimi(ad string) (int64, bool) {
	e := noktaKalibi.FindStringSubmatch(ad)
	if e == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(e[1], 10, 64)
	return n, err == nil && noktaAdi(n) == ad
}

// sonKaydi son.json'dur.
type sonKaydi struct {
	Surum  int    `json:"sürüm"`
	Adim   int64  `json:"adım"`
	Klasor string `json:"klasör"`
}

// ozetKaydi ozet.json'dur: noktadaki her dosyanın boyu ve özeti.
type ozetKaydi struct {
	Surum    int                   `json:"sürüm"`
	Adim     int64                 `json:"adım"`
	Dosyalar map[string]dosyaOzeti `json:"dosyalar"`
}

type dosyaOzeti struct {
	Bayt   int64  `json:"bayt"`
	SHA256 string `json:"sha256"`
}

// durumKaydi durum.json'dur.
type durumKaydi struct {
	Surum   int    `json:"sürüm"`
	Adim    int64  `json:"adım"`
	Zaman   string `json:"zaman"`
	TR      string `json:"tr"`      // TR sürümü (bilgi için)
	Program string `json:"program"` // programın dosya adı (bilgi için)
	Isci    int    `json:"işçi"`    // GOMAXPROCS (bilgi için)
	// Ayarlar programın komut_satırı sonucudur (gizli değerler maskeli); sürdürmede
	// farklıysa uyarı verilir.
	Ayarlar  json.RawMessage `json:"ayarlar,omitempty"`
	Nesneler []nesneKaydi    `json:"nesneler"`
}

// nesneKaydi kontrol_noktası sözlüğündeki bir nesnenin kaydıdır. Tensörleri (parametreler,
// iyileştirici durumu) tensorler.trt'de "<ad>/<tensör adı>" adıyla durur.
type nesneKaydi struct {
	Ad              string          `json:"ad"`
	Tur             string          `json:"tür"`
	Iyilestirici    string          `json:"iyileştirici,omitempty"`     // AdamW, SGD
	ParametreSayisi int             `json:"parametre_sayısı,omitempty"` // iyileştiricinin
	Uzunluk         int64           `json:"uzunluk,omitempty"`          // günlüğün bayt uzunluğu
	Deger           json.RawMessage `json:"değer,omitempty"`            // düz değer ya da okuyucu durumu (duz.go)
}

// Nesne türlerinin dosyadaki adları (biçimin parçası; çevrilmez).
var turAdlari = strings.Fields("parametreler iyileştirici durumlu günlük değerler")

const (
	turModel nesneTuru = iota
	turIyilestirici
	turDurumlu
	turGunluk
	turDuz
)

type nesneTuru int

func (t nesneTuru) String() string { return turAdlari[t] }

func turCoz(ad string) (nesneTuru, bool) {
	for i, a := range turAdlari {
		if a == ad {
			return nesneTuru(i), true
		}
	}
	return 0, false
}

// jsonCoz b'yi katı çözer: bilinmeyen alan ve değerden sonra fazladan veri hatadır.
func jsonCoz(b []byte, hedef any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(hedef); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON değerinden sonra fazladan veri var")
	}
	return nil
}

// surumDenetle dosyanın biçim sürümünü denetler.
func surumDenetle(dosya string, surum int) error {
	switch {
	case surum >= enEskiSurumu && surum <= bicimSurumu:
		return nil
	case surum > bicimSurumu:
		return fmt.Errorf("%s biçim sürümü %d: daha yeni bir TR sürümüyle yazılmış; bu TR %d ile %d arası sürümleri okur (TR'yi güncelleyin)", dosya, surum, enEskiSurumu, bicimSurumu)
	}
	return fmt.Errorf("%s biçim sürümü %d geçersiz", dosya, surum)
}

// ozetCoz ozet.json'u çözer ve noktanın adımıyla karşılaştırır.
func ozetCoz(b []byte, adim int64) (*ozetKaydi, error) {
	var o ozetKaydi
	if err := jsonCoz(b, &o); err != nil {
		return nil, fmt.Errorf("%s çözülemedi: %v", ozetDosyasi, err)
	}
	if err := surumDenetle(ozetDosyasi, o.Surum); err != nil {
		return nil, err
	}
	if o.Adim != adim {
		return nil, fmt.Errorf("%s adım %d diyor, klasör adı %d", ozetDosyasi, o.Adim, adim)
	}
	if len(o.Dosyalar) != 2 {
		return nil, fmt.Errorf("%s %d dosya listeliyor, 2 olmalı (%s, %s)", ozetDosyasi, len(o.Dosyalar), durumDosyasi, tensorDosyasi)
	}
	for _, ad := range []string{durumDosyasi, tensorDosyasi} {
		d, ok := o.Dosyalar[ad]
		if !ok {
			return nil, fmt.Errorf("%s %s dosyasını listelemiyor", ozetDosyasi, ad)
		}
		if d.Bayt < 0 || len(d.SHA256) != 64 || strings.Trim(d.SHA256, "0123456789abcdef") != "" {
			return nil, fmt.Errorf("%s içinde %s kaydı geçersiz", ozetDosyasi, ad)
		}
	}
	return &o, nil
}

// durumCoz durum.json'u çözer ve noktanın adımıyla karşılaştırır.
func durumCoz(b []byte, adim int64) (*durumKaydi, error) {
	var d durumKaydi
	if err := jsonCoz(b, &d); err != nil {
		return nil, fmt.Errorf("%s çözülemedi: %v", durumDosyasi, err)
	}
	if err := surumDenetle(durumDosyasi, d.Surum); err != nil {
		return nil, err
	}
	if d.Adim != adim {
		return nil, fmt.Errorf("%s adım %d diyor, klasör adı %d", durumDosyasi, d.Adim, adim)
	}
	if len(d.Nesneler) == 0 || len(d.Nesneler) > enFazlaNesne {
		return nil, fmt.Errorf("%s %d nesne listeliyor (1 ile %d arası olmalı)", durumDosyasi, len(d.Nesneler), enFazlaNesne)
	}
	gorulen := map[string]bool{}
	for _, n := range d.Nesneler {
		if err := nesneAdiDenetle(n.Ad); err != nil {
			return nil, fmt.Errorf("%s: %v", durumDosyasi, err)
		}
		if gorulen[n.Ad] {
			return nil, fmt.Errorf("%s: %q nesnesi iki kez geçiyor", durumDosyasi, n.Ad)
		}
		gorulen[n.Ad] = true
		if _, ok := turCoz(n.Tur); !ok {
			return nil, fmt.Errorf("%s: %q nesnesinin türü %q bilinmiyor", durumDosyasi, n.Ad, n.Tur)
		}
		if n.Uzunluk < 0 || n.ParametreSayisi < 0 {
			return nil, fmt.Errorf("%s: %q nesnesinin kaydı geçersiz", durumDosyasi, n.Ad)
		}
	}
	return &d, nil
}

// nesneAdiDenetle bir nesne adının (sözlük anahtarı) dosya biçimine uygun olduğunu
// denetler: boş değil, en çok enFazlaNesneAdi bayt, '/' ve denetim karakteri içermez
// (tensör adları "<ad>/<tensör adı>" biçimindedir).
func nesneAdiDenetle(ad string) error {
	if ad == "" || len(ad) > enFazlaNesneAdi {
		return fmt.Errorf("nesne adı boş ya da çok uzun (en çok %d bayt)", enFazlaNesneAdi)
	}
	for _, r := range ad {
		if r == '/' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("%q nesne adı '/' ya da denetim karakteri içeremez", ad)
		}
	}
	return nil
}

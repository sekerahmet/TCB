package veri

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Meta, veri kökündeki meta.json dosyasıdır. Yalnızca yükleyicinin
// kullandığı alanlar türlüdür; geri kalanı ham tutulur.
type Meta struct {
	Tokenizer json.RawMessage            `json:"tokenizer"`
	Files     map[string]json.RawMessage `json:"files"`
	Parts     map[string]MetaParca       `json:"parts"`
	Valid     *MetaDogrulama             `json:"valid"`
}

// MetaParca, meta.json içindeki bir parça kaydıdır.
type MetaParca struct {
	Tokens int64 `json:"tokens"`
}

// MetaDogrulama, meta.json içindeki doğrulama kümesi kaydıdır.
type MetaDogrulama struct {
	Tokens int64 `json:"tokens"`
	Docs   int64 `json:"docs"`
}

// TokenizerYolu, 'tokenizer' alanı bir metinse onu döner.
func (m *Meta) TokenizerYolu() string {
	var s string
	if m == nil || json.Unmarshal(m.Tokenizer, &s) != nil {
		return ""
	}
	return s
}

// MetaOku, verilen meta.json dosyasını okur.
func MetaOku(yol string) (*Meta, error) {
	b, err := os.ReadFile(yol)
	if err != nil {
		return nil, fmt.Errorf("veri: meta okunamadı: %w", err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("veri: %s: meta.json çözümlenemedi: %w", yol, err)
	}
	return &m, nil
}

// ParcaBilgisi, kümedeki bir parçanın adı, yolu ve token sayısıdır.
type ParcaBilgisi struct {
	Ad    string
	Yol   string
	Token int64
	Bayt  int64 // dosya boyutu (dizin listesinden)
}

// KumeSecenekleri, KumeAc'ın davranışını belirler.
type KumeSecenekleri struct {
	// Desen, parça dosyalarının adı için kalıptır (varsayılan "*.npy").
	Desen string
	// MetaYok true ise meta.json aranmaz; tüm parçaların başlığı okunur.
	MetaYok bool
	// Parca, parçaların açılış seçenekleridir.
	Parca ParcaSecenekleri
	// Onbellek verilirse parçalar açılmadan önce yerel önbelleğe alınır;
	// yerel kopya alınamazsa parça açma hata verir (kaynağa geri dönülmez).
	// Varsayılan (nil): önbellek kapalı, parçalar doğrudan kaynaktan
	// okunur. UYARI: kaynak yavaşsa (ağ ya da akışlı bulut sürücüsü)
	// rastgele pencere okuması eğitimi kat kat yavaşlatır; böyle
	// kaynaklarda önbellek kullanın.
	Onbellek *Onbellek
}

// Kume, bir dizindeki .npy parçalarının sıralı listesidir. Parçalar ancak
// okunacakları zaman açılır.
type Kume struct {
	dizin    string
	parcalar []ParcaBilgisi
	toplam   int64
	meta     *Meta
	sec      KumeSecenekleri
}

// KumeAc, dizindeki parçaları tanır. dizin tek bir .npy dosyası da olabilir.
// Dizinde ya da üst dizinde meta.json varsa, parça token sayıları oradan
// alınır ve dosya başlıkları açılışta okunmaz (yavaş kaynaklarda önemli);
// sayılar parça açıldığında başlıkla karşılaştırılır.
func KumeAc(dizin string, s KumeSecenekleri) (*Kume, error) {
	if s.Desen == "" {
		s.Desen = "*.npy"
	}
	st, err := os.Stat(dizin)
	if err != nil {
		return nil, fmt.Errorf("veri: küme açılamadı: %w", err)
	}
	var yollar []string
	baytlar := map[string]int64{}
	metaDizinleri := []string{dizin, filepath.Dir(dizin)}
	if st.IsDir() {
		if _, err := filepath.Match(s.Desen, ""); err != nil {
			return nil, fmt.Errorf("veri: geçersiz desen %q: %w", s.Desen, err)
		}
		// Dizin listesi boyutları da verir; parça başına ayrı sorgu gerekmez.
		girdiler, err := os.ReadDir(dizin)
		if err != nil {
			return nil, fmt.Errorf("veri: küme dizini okunamadı: %w", err)
		}
		for _, g := range girdiler {
			if g.IsDir() {
				continue
			}
			if ok, _ := filepath.Match(s.Desen, g.Name()); !ok {
				continue
			}
			bilgi, err := g.Info()
			if err != nil {
				return nil, fmt.Errorf("veri: %s: %w", g.Name(), err)
			}
			y := filepath.Join(dizin, g.Name())
			yollar = append(yollar, y)
			baytlar[y] = bilgi.Size()
		}
	} else {
		yollar = []string{dizin}
		baytlar[dizin] = st.Size()
		metaDizinleri = []string{filepath.Dir(dizin), filepath.Dir(filepath.Dir(dizin))}
	}
	sort.Strings(yollar)
	if len(yollar) == 0 {
		return nil, fmt.Errorf("veri: %s içinde %q desenine uyan veri dosyası yok", dizin, s.Desen)
	}

	k := &Kume{dizin: dizin, sec: s}
	if !s.MetaYok {
		for _, d := range metaDizinleri {
			my := filepath.Join(d, "meta.json")
			if _, err := os.Stat(my); err == nil {
				m, err := MetaOku(my)
				if err != nil {
					return nil, err
				}
				k.meta = m
				break
			}
		}
	}
	for _, y := range yollar {
		ad := filepath.Base(y)
		pb := ParcaBilgisi{Ad: ad, Yol: y, Token: -1, Bayt: baytlar[y]}
		if k.meta != nil {
			if mp, ok := k.meta.Parts[ad]; ok {
				pb.Token = mp.Tokens
			}
		}
		if pb.Token < 0 {
			n, err := baslikUzunlugu(y)
			if err != nil {
				return nil, err
			}
			pb.Token = n
		}
		k.parcalar = append(k.parcalar, pb)
		k.toplam += pb.Token
	}
	return k, nil
}

func baslikUzunlugu(yol string) (int64, error) {
	f, err := os.Open(yol)
	if err != nil {
		return 0, fmt.Errorf("veri: veri dosyası açılamadı: %w", err)
	}
	defer f.Close()
	bs, err := BaslikOku(f)
	if err != nil {
		var bh *BicimHatasi
		if errors.As(err, &bh) {
			bh.Yol = yol
		}
		return 0, err
	}
	return bs.Uzunluk, nil
}

// Dizin, kümenin açıldığı yoldur.
func (k *Kume) Dizin() string { return k.dizin }

// Meta, bulunduysa meta.json içeriğidir (yoksa nil).
func (k *Kume) Meta() *Meta { return k.meta }

// Parcalar, parçaların ada göre sıralı listesidir (kopya).
func (k *Kume) Parcalar() []ParcaBilgisi { return append([]ParcaBilgisi(nil), k.parcalar...) }

// ParcaSayisi, parça sayısıdır.
func (k *Kume) ParcaSayisi() int { return len(k.parcalar) }

// ToplamToken, tüm parçalardaki token sayısıdır.
func (k *Kume) ToplamToken() int64 { return k.toplam }

// ParcaAc, i'inci parçayı açar. Önbellek etkinse parça önce yerel
// önbelleğe alınır (gerekirse kopyalanmasını bekler) ve Kapat'a kadar
// önbellekten silinmez.
func (k *Kume) ParcaAc(ctx context.Context, i int) (*Parca, error) {
	if i < 0 || i >= len(k.parcalar) {
		return nil, fmt.Errorf("veri: veri dosyası sırası %d geçersiz (0..%d)", i, len(k.parcalar)-1)
	}
	pb := k.parcalar[i]
	yol, birak := pb.Yol, func() {}
	if k.sec.Onbellek != nil {
		y, b, err := k.sec.Onbellek.Getir(ctx, pb.Yol)
		if err != nil {
			return nil, err
		}
		yol, birak = y, b
	}
	p, err := ParcaAc(yol, k.sec.Parca)
	if err != nil {
		birak()
		return nil, err
	}
	p.birak = birak
	if p.Uzunluk() != pb.Token {
		_ = p.Kapat()
		return nil, fmt.Errorf("veri: %s: başlık %d token bildiriyor, meta.json %d diyor", pb.Yol, p.Uzunluk(), pb.Token)
	}
	return p, nil
}

// onbellekKotaDenetle, önbellek etkinse kotanın aynı anda gereken parça
// sayısına (açık parçalar + ileri bakış) en büyük parça boyutuyla yettiğini
// denetler. enKisa tokendan kısa parçalar hiç açılmadığı için sayılmaz.
func (k *Kume) onbellekKotaDenetle(adet int, enKisa int64) error {
	ob := k.sec.Onbellek
	if ob == nil {
		return nil
	}
	var enBuyuk int64
	uygun := 0
	for _, p := range k.parcalar {
		if p.Token >= enKisa {
			uygun++
			enBuyuk = max(enBuyuk, p.Bayt)
		}
	}
	adet = min(adet, uygun)
	gereken := int64(adet) * enBuyuk
	if ob.Kota() < gereken {
		return fmt.Errorf("%w: kota %d bayt, en az %d veri dosyası × en büyük veri dosyası %d bayt = %d bayt gerekli "+
			"(kotayı büyütün ya da ileri bakışı/grup boyunu küçültün)",
			ErrOnbellekYetersiz, ob.Kota(), adet, enBuyuk, gereken)
	}
	// Klasörü paylaşan diğer kümelerin ayırdığı yerle birlikte de sığmalı.
	return ob.Ayir(gereken)
}

// onbellekHazirla, önbellek etkinse verilen parçaların arka planda
// kopyalanmasını ister.
func (k *Kume) onbellekHazirla(siralar []int) {
	if k.sec.Onbellek == nil {
		return
	}
	yollar := make([]string, len(siralar))
	for i, s := range siralar {
		yollar[i] = k.parcalar[s].Yol
	}
	k.sec.Onbellek.Hazirla(yollar)
}

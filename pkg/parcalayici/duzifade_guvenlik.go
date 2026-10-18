package parcalayici

import (
	"fmt"
	"sync/atomic"
)

// ============================================================================
// Düzenli ifade motorunun güvenlik sınırları
// ============================================================================
//
// Geri izlemeli bir motor, iç içe niceleyicili ya da çakışan seçenekli bir
// desende kötü niyetli bir girdiyle üstel zamana düşebilir (ReDoS). Desenler
// parçalayıcı dosyasından geldiği için (yarı güvenilir) iki önlem alınır:
//
//  1. Adım bütçesi: bir çağrının bütün aramaları (bir metnin kodlanmasında
//     normalleştirici ve ön bölücülerdeki bütün desenler) toplam
//     AdimTabani + AdimCarpani·(işlenen bayt) adımı paylaşır. Böylece her
//     araması kendi bütçesinde kalan ama toplamda karesel büyüyen desenler
//     de durdurulur. Aşılırsa AdimSiniriHatasi oluşur; sessizce yanlış bölme
//     yapılmaz. Geri izleme noktalarının belleği de GeriIzlemeSiniri ile
//     sınırlıdır.
//  2. Derleme anında risk çözümlemesi: iç içe sınırsız niceleyiciler (ör.
//     "(a+)+", "(\w+\s*)*") varsayılan olarak reddedilir; sınırsız bir
//     yinelemenin içinde aynı karakterle başlayan seçenekler (ör. "(a|ab)*")
//     uyarı olarak kaydedilir.

// DuzIfadeAyarlari motorun güvenlik ayarlarıdır.
type DuzIfadeAyarlari struct {
	// AdimTabani ve AdimCarpani bir çağrının (ör. bir metnin kodlanması)
	// bütün aramalarının paylaştığı adım bütçesini belirler:
	// AdimTabani + AdimCarpani * (işlenen bayt sayısı).
	AdimTabani  int64
	AdimCarpani int64
	// RiskliDesenIzni doğruysa iç içe sınırsız niceleyicili desenler
	// reddedilmez (adım bütçesi yine geçerlidir).
	RiskliDesenIzni bool
	// GeriIzlemeSiniri bir aramanın aynı anda tutabileceği en çok geri
	// izleme noktası sayısıdır (bellek sınırı; kayıt başına ~24 bayt).
	GeriIzlemeSiniri int
}

// VarsayilanDuzIfadeAyarlari varsayılan güvenlik ayarlarıdır.
var VarsayilanDuzIfadeAyarlari = DuzIfadeAyarlari{AdimTabani: 1_000_000, AdimCarpani: 100, GeriIzlemeSiniri: 1 << 22}

var duzIfadeAyar atomic.Pointer[DuzIfadeAyarlari]

func init() {
	a := VarsayilanDuzIfadeAyarlari
	duzIfadeAyar.Store(&a)
}

// DuzIfadeAyarla motorun güvenlik ayarlarını değiştirir (bütün desenler için,
// eşzamanlı kullanımda güvenlidir). Sıfır ya da negatif bütçe değerleri
// varsayılana çevrilir.
func DuzIfadeAyarla(a DuzIfadeAyarlari) {
	if a.AdimTabani <= 0 {
		a.AdimTabani = VarsayilanDuzIfadeAyarlari.AdimTabani
	}
	if a.AdimCarpani <= 0 {
		a.AdimCarpani = VarsayilanDuzIfadeAyarlari.AdimCarpani
	}
	if a.GeriIzlemeSiniri <= 0 {
		a.GeriIzlemeSiniri = VarsayilanDuzIfadeAyarlari.GeriIzlemeSiniri
	}
	duzIfadeAyar.Store(&a)
}

// DuzIfadeAyarlariniAl geçerli güvenlik ayarlarını verir.
func DuzIfadeAyarlariniAl() DuzIfadeAyarlari { return *duzIfadeAyar.Load() }

// AdimSiniriHatasi bir çağrının düzenli ifade adım bütçesini ya da geri
// izleme belleği sınırını aştığını bildirir.
type AdimSiniriHatasi struct {
	Desen  string // desenin kaynağı
	Konum  int    // sınırın aşıldığı aramanın başladığı bayt konumu
	Butce  int64  // aşılan adım bütçesi
	Bellek bool   // aşılan, geri izleme belleği sınırıysa (GeriIzlemeSiniri) doğru
}

func (e *AdimSiniriHatasi) Error() string {
	if e.Bellek {
		return fmt.Sprintf("düzenli ifade %q, girdinin %d. bayt konumundan başlayan aramada geri izleme belleği sınırını (%d kayıt) aştı: "+
			"desen bu girdide çok uzun bir yinelemeyi geri izlenebilir tutuyor; parçalama durduruldu",
			dfKisalt(e.Desen), e.Konum, DuzIfadeAyarlariniAl().GeriIzlemeSiniri)
	}
	return fmt.Sprintf("düzenli ifade %q, girdinin %d. bayt konumundan başlayan aramada adım sınırını (%d) aştı: "+
		"desen bu girdide üstel ya da aşırı geri izleme yapıyor; parçalama durduruldu", dfKisalt(e.Desen), e.Konum, e.Butce)
}

// adimHatasiniYakala f'yi çalıştırır; adım sınırı aşılırsa hatayı döndürür
// (diğer panikler olduğu gibi sürer).
func adimHatasiniYakala(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if h, ok := r.(*AdimSiniriHatasi); ok {
				err = h
				return
			}
			panic(r)
		}
	}()
	f()
	return nil
}

// ---------------------------------------------------------------------------
// Derleme anında risk çözümlemesi
// ---------------------------------------------------------------------------

// dfDoyma uzunluk hesaplarının doyduğu (taşmadığı) üst sınırdır.
const dfDoyma = 1 << 40

func dfDoymaliCarp(a, b int) int {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > dfDoyma/b {
		return dfDoyma
	}
	return a * b
}

func dfDoymaliTop(a, b int) int {
	if a+b > dfDoyma {
		return dfDoyma
	}
	return a + b
}

// sinirsiz niceleyicinin "sınırsıza yakın" sayıldığı üst sınır.
const riskliUstSinir = 100

func dfSinirsiz(t *dfTekrar) bool { return (t.ust < 0 || t.ust >= riskliUstSinir) && !t.sahiplenici }

// dfEnKisa düğümün eşleyebileceği en az karakter sayısı.
func dfEnKisa(d dfDugum) int {
	switch x := d.(type) {
	case *dfKarakter, *dfSinif, *dfNokta:
		return 1
	case *dfBirlesim:
		t := 0
		for _, o := range x.ogeler {
			t = dfDoymaliTop(t, dfEnKisa(o))
		}
		return t
	case *dfSecenek:
		m := -1
		for _, o := range x.secenekler {
			if n := dfEnKisa(o); m < 0 || n < m {
				m = n
			}
		}
		if m < 0 {
			return 0
		}
		return m
	case *dfTekrar:
		return dfDoymaliCarp(x.en, dfEnKisa(x.alt))
	case *dfAtomik:
		return dfEnKisa(x.alt)
	}
	return 0
}

// dfSerbestYineleme: düğümün bir yinelemesi, kendisi de sınırsız olan bir iç
// niceleyiciyle (geri kalanı boş eşleşebilecek biçimde) tamamen
// karşılanabiliyorsa doğru. Bu durumda dış yineleme, girdiyi üstel sayıda
// farklı biçimde bölebilir.
func dfSerbestYineleme(d dfDugum) bool {
	switch x := d.(type) {
	case *dfTekrar:
		return dfSinirsiz(x)
	case *dfBirlesim:
		for i, o := range x.ogeler {
			if !dfSerbestYineleme(o) {
				continue
			}
			bos := true
			for j, p := range x.ogeler {
				if j != i && dfEnKisa(p) > 0 {
					bos = false
					break
				}
			}
			if bos {
				return true
			}
		}
		// Arka arkaya (araları boş eşleşebilen) iki sınırsız niceleyici aynı
		// karakterleri paylaşıyorsa (ör. "x+x+") girdiyi bölmenin yolu çoktur.
		for i, o := range x.ogeler {
			ti, ok := o.(*dfTekrar)
			if !ok || !dfSinirsiz(ti) {
				continue
			}
			for j := i + 1; j < len(x.ogeler); j++ {
				if tj, ok := x.ogeler[j].(*dfTekrar); ok && dfSinirsiz(tj) && dfCakisir(ti.alt, tj.alt) {
					return true
				}
				if dfEnKisa(x.ogeler[j]) > 0 {
					break
				}
			}
		}
	case *dfSecenek:
		for _, o := range x.secenekler {
			if dfSerbestYineleme(o) {
				return true
			}
		}
	}
	return false
}

// dfIlkKarakterler düğümün ilk karakteri için olası düz karakterleri ve
// sınıf koşullarını verir (yaklaşık; çakışma uyarısı için).
func dfIlkKarakterler(d dfDugum) ([]rune, []func(rune) bool) {
	switch x := d.(type) {
	case *dfKarakter:
		return append([]rune{x.r}, x.katla...), nil
	case *dfSinif:
		return nil, []func(rune) bool{x.f}
	case *dfNokta:
		return nil, []func(rune) bool{func(r rune) bool { return r != '\n' || x.satirSonuDahil }}
	case *dfBirlesim:
		var rs []rune
		var fs []func(rune) bool
		for _, o := range x.ogeler {
			r, f := dfIlkKarakterler(o)
			rs, fs = append(rs, r...), append(fs, f...)
			if dfEnKisa(o) > 0 {
				break
			}
		}
		return rs, fs
	case *dfSecenek:
		var rs []rune
		var fs []func(rune) bool
		for _, o := range x.secenekler {
			r, f := dfIlkKarakterler(o)
			rs, fs = append(rs, r...), append(fs, f...)
		}
		return rs, fs
	case *dfTekrar:
		return dfIlkKarakterler(x.alt)
	case *dfAtomik:
		return dfIlkKarakterler(x.alt)
	}
	return nil, nil
}

func dfCakisir(a, b dfDugum) bool {
	ra, fa := dfIlkKarakterler(a)
	rb, fb := dfIlkKarakterler(b)
	for _, x := range ra {
		for _, y := range rb {
			if x == y {
				return true
			}
		}
		for _, f := range fb {
			if f(x) {
				return true
			}
		}
	}
	for _, y := range rb {
		for _, f := range fa {
			if f(y) {
				return true
			}
		}
	}
	return false
}

// dfRiskCozumle desen ağacını gezer; reddedilecek yapı için hata iletisi,
// ayrıca uyarılar döndürür.
func dfRiskCozumle(d dfDugum) (hata string, uyarilar []string) {
	var gez func(d dfDugum)
	gez = func(d dfDugum) {
		switch x := d.(type) {
		case *dfBirlesim:
			for _, o := range x.ogeler {
				gez(o)
			}
		case *dfSecenek:
			for _, o := range x.secenekler {
				gez(o)
			}
		case *dfAtomik:
			gez(x.alt)
		case *dfBakma:
			gez(x.alt)
		case *dfTekrar:
			if dfSinirsiz(x) {
				if hata == "" && dfSerbestYineleme(x.alt) {
					hata = "iç içe sınırsız niceleyici (ör. \"(a+)+\" ya da \"(\\w+\\s*)*\") üstel geri izlemeye yol açabilir"
				}
				if s, ok := x.alt.(*dfSecenek); ok {
					for i := 0; i < len(s.secenekler); i++ {
						for j := i + 1; j < len(s.secenekler); j++ {
							if dfCakisir(s.secenekler[i], s.secenekler[j]) {
								uyarilar = append(uyarilar, fmt.Sprintf(
									"sınırsız yineleme içinde %d. ve %d. seçenek aynı karakterle başlayabiliyor; uzun girdilerde aşırı geri izleme olabilir", i+1, j+1))
							}
						}
					}
				}
			}
			gez(x.alt)
		}
	}
	gez(d)
	return hata, uyarilar
}

// Uyarilar desenin derlenirken bulunan risk uyarılarını verir.
func (d *DuzIfade) Uyarilar() []string { return append([]string(nil), d.uyarilar...) }

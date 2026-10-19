package gpu

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// GPU profili: dışa açık Go arayüzü ve özet
// ============================================================================
//
// İzleme araçları (canlı profil) şu üç işi kullanır: ProfilAc ile ölçümü açıp kapatır,
// CekirdekOlcumleri ile şimdiye kadarki çekirdek başına toplamları (anlık görüntü; arka ucu
// beklemez, saniyede birkaç kez okunabilir) ve BellekOlcumleri ile GPU belleğinin durumunu
// alır. TR_GPU_PROFIL=1 ile çalışan bir program sonunda (Kapat) özet standart hataya yazılır.

// CekirdekOlcumu bir çekirdeğin (ya da sürücü bellek işinin) toplamları.
type CekirdekOlcumu struct {
	Ad        string // üretilen çekirdeğin adı (ör. "carpim_128_128_8_...")
	Aile      string // adın ailesi (ör. "carpim")
	Cagri     int64
	GPUSuresi time.Duration
}

// BellekOlcumu bir GPU'nun bellek havuzunun durumu (bayt).
type BellekOlcumu struct {
	Cihaz        int
	Ayrilmis     uint64 // sürücüden ayrılmış toplam
	Havuzda      uint64 // ayrılmış ama boşta (yeniden kullanılmayı bekleyen)
	Canli        uint64 // tensörlere verilmiş (çöp toplayıcının henüz bırakmadıkları dahil)
	TepeCanli    uint64
	TepeAyrilmis uint64
	CopToplama   int64 // bellek için başlatılan çöp toplama sayısı
	SurucuAyirma int64 // sürücüden yeni blok ayırma sayısı
}

var kayitli struct {
	mu sync.Mutex
	uc []*arkaUc
}

// kaydet arka ucu profil ve bellek ölçümlerine ekler (cihaz olarak kaydedilenler).
func kaydet(g *arkaUc) *arkaUc {
	kayitli.mu.Lock()
	kayitli.uc = append(kayitli.uc, g)
	kayitli.mu.Unlock()
	return g
}

func kayitlilar() []*arkaUc {
	kayitli.mu.Lock()
	defer kayitli.mu.Unlock()
	return append([]*arkaUc(nil), kayitli.uc...)
}

// ProfilAc GPU çekirdek ölçümünü açar ya da kapatır (TR_GPU_PROFIL=1 ile başlangıçta
// açıktır). Kapalıyken maliyet başlatma başına tek bir atomik okumadır.
func ProfilAc(acik bool) { profilAcik.Store(acik) }

// ProfilAcik ölçümün açık olup olmadığı.
func ProfilAcik() bool { return profilAcik.Load() }

// CekirdekOlcumleri bütün GPU'larda şimdiye kadar çözülmüş ölçümlerin çekirdek başına
// toplamlarını GPU süresine göre azalan sırada verir. Son ~250 ms'nin ölçümleri bir sonraki
// görüntüde görünür.
func CekirdekOlcumleri() []CekirdekOlcumu {
	topla := map[string]*CekirdekOlcumu{}
	for _, g := range kayitlilar() {
		for _, o := range g.prof.sirali() {
			if t := topla[o.Ad]; t != nil {
				t.Cagri += o.Cagri
				t.GPUSuresi += o.GPUSuresi
				continue
			}
			c := o
			topla[o.Ad] = &c
		}
	}
	r := make([]CekirdekOlcumu, 0, len(topla))
	for _, o := range topla {
		r = append(r, *o)
	}
	sort.Slice(r, func(i, j int) bool { return r[i].GPUSuresi > r[j].GPUSuresi })
	return r
}

// BellekOlcumleri her GPU'nun bellek havuzunun durumunu verir.
func BellekOlcumleri() []BellekOlcumu {
	var r []BellekOlcumu
	for _, g := range kayitlilar() {
		b := g.havuz.olcum()
		b.Cihaz = g.dev.Index
		r = append(r, b)
	}
	return r
}

// gpuKapandi dış profilleyici için C çıkış işleyicileri çalıştırıldıktan sonra doğrudur: o
// andan sonra sürücüye hiçbir çağrı yapılmaz (gir açık hata verir).
var gpuKapandi atomic.Bool

var cikisBirKez sync.Once

// Kapat program sonunda çağrılır: ölçüm açıksa özeti standart hataya yazar, matris çarpımı
// kitaplığının tanımlarını yok eder ve bağlı dış profilleyicilere topladıkları veriyi
// yazdırır. Birden çok kez çağrılabilir; özet yalnızca son yazımdan sonra yeni ölçüm varsa
// yeniden yazılır.
//
// Dış profilleyici bağlıyken (CUDA_INJECTION64_PATH) bütün GPU'lar kapandıktan sonra, süreç
// genelinde bir kez C çıkış işleyicileri çalıştırılır (__cxa_finalize(NULL)): Go programı
// bunları çalıştırmadan biter, araçlar ise topladıklarını orada yazar. Bu işleyiciler
// sürücünün ve araçların C++ durumunu da yıkar; bu yüzden ardından GPU kullanılamaz
// (gpuKapandi) ve Kapat'ın sonraki çağrıları hiçbir şey yapmaz. Araç bağlı değilse hiç
// çalıştırılmaz.
func Kapat() {
	if gpuKapandi.Load() {
		return
	}
	for _, g := range kayitlilar() {
		g.kapat()
	}
	if aracBagli() && len(kayitlilar()) > 0 {
		cikisBirKez.Do(func() {
			for _, g := range kayitlilar() {
				g.mu.Lock() // başka bir gorutin GPU'yu kullanıyorsa bitmesini bekle
			}
			gpuKapandi.Store(true)
			cikisIsleyicileriniCalistir()
			for _, g := range kayitlilar() {
				g.mu.Unlock()
			}
		})
	}
}

// kapat bir GPU'nun program sonu işleri; sürücü hatası (ör. yapışkan bağlam hatası) programın
// çıkışını bozmasın diye yutulur (hata zaten bildirilmiştir).
func (g *arkaUc) kapat() {
	defer func() { _ = recover() }()
	g.gir()
	defer g.cik()
	if profilAcik.Load() && g.prof.destek > 0 {
		g.esitle()
		g.adimIsaretle() // son adımın duvar süresi
		if g.prof.cagri != g.prof.yazilan {
			g.prof.yazilan = g.prof.cagri
			g.ozetYaz(os.Stderr)
		}
	}
	g.ltKapat()
	g.sr.araclariBosalt()
}

// ProfilOzetiYaz bütün GPU'ların ölçüm özetini w'ye yazar (sürücü hatası yutulur).
func ProfilOzetiYaz(w io.Writer) {
	for _, g := range kayitlilar() {
		func() {
			defer func() { _ = recover() }()
			g.gir()
			defer g.cik()
			g.esitle()
			g.ozetYaz(w)
		}()
	}
}

// ozetYaz bu GPU'nun özetini yazar (kilit tutulurken).
func (g *arkaUc) ozetYaz(w io.Writer) {
	p := &g.prof
	fmt.Fprintf(w, "== GPU profili (%s: %s; matris çarpımı: %s) ==\n", g.dev, g.sr.kartAdi(), g.carpimYolu())
	p.mu.Lock()
	var top adimIstat
	sayi := 0
	// İlk iki adım (derleme ve ısınma) ve işaretten sonra yarım kalan son adım dışarıda.
	isaretli := len(p.adimlar)
	for i, a := range p.adimlar {
		if i < 2 || !a.bitti {
			continue
		}
		top.duvar += a.duvar
		top.konak += a.konak
		top.esit += a.esit
		top.gpuMs += a.gpuMs
		top.boslukMs += a.boslukMs
		top.baslatma += a.baslatma
		top.cop += a.cop
		top.copSure += a.copSure
		sayi++
	}
	p.mu.Unlock()
	if sayi > 0 {
		n := float64(sayi)
		ms := func(d time.Duration) float64 { return d.Seconds() * 1e3 / n }
		fmt.Fprintf(w, "adım başına (%d adım): duvar %.2f ms · GPU çekirdek %.2f ms · GPU boşluğu %.2f ms · başlatma %.0f · "+
			"konakta arka uç %.2f ms · eşzamanlama beklemesi %.2f ms · çöp toplama %.2f kez, %.2f ms\n",
			sayi, ms(top.duvar), top.gpuMs/n, top.boslukMs/n, float64(top.baslatma)/n, ms(top.konak), ms(top.esit),
			float64(top.cop)/n, ms(top.copSure))
	} else {
		fmt.Fprintf(w, "adım başına: adım bilgisi yok (%d adım işaretlendi; ilk iki adım ısınmadır, en az üç tamamlanmış adım gerekir; adımlar iyileştiricinin adımıyla işaretlenir)\n", isaretli)
	}
	olcumler := p.sirali()
	var toplam time.Duration
	aile := map[string]*CekirdekOlcumu{}
	for _, o := range olcumler {
		toplam += o.GPUSuresi
		a := aile[o.Aile]
		if a == nil {
			a = &CekirdekOlcumu{Ad: o.Aile}
			aile[o.Aile] = a
		}
		a.Cagri += o.Cagri
		a.GPUSuresi += o.GPUSuresi
	}
	var aileler []CekirdekOlcumu
	for _, a := range aile {
		aileler = append(aileler, *a)
	}
	sort.Slice(aileler, func(i, j int) bool { return aileler[i].GPUSuresi > aileler[j].GPUSuresi })
	tablo := func(baslik string, r []CekirdekOlcumu, enCok int) {
		fmt.Fprintf(w, "%-44s %9s %12s %10s %6s\n", baslik, "çağrı", "toplam ms", "ort µs", "%")
		for i, o := range r {
			if i >= enCok {
				break
			}
			yuzde := 0.0
			if toplam > 0 {
				yuzde = 100 * float64(o.GPUSuresi) / float64(toplam)
			}
			fmt.Fprintf(w, "%-44s %9d %12.2f %10.1f %6.1f\n", kisalt(o.Ad, 44), o.Cagri, o.GPUSuresi.Seconds()*1e3,
				o.GPUSuresi.Seconds()*1e6/float64(max(o.Cagri, 1)), yuzde)
		}
	}
	tablo("aile", aileler, 30)
	tablo("çekirdek (ilk 25)", olcumler, 25)
	b := g.havuz.olcum()
	fmt.Fprintf(w, "bellek: ayrılmış %s · havuzda %s · canlı %s · tepe canlı %s · tepe ayrılmış %s · sürücü ayırma %d · çöp toplama %d\n",
		baytYazi(b.Ayrilmis), baytYazi(b.Havuzda), baytYazi(b.Canli), baytYazi(b.TepeCanli), baytYazi(b.TepeAyrilmis),
		b.SurucuAyirma, b.CopToplama)
}

func kisalt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

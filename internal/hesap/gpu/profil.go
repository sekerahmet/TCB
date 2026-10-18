package gpu

import (
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// GPU profili: çekirdek başına GPU süresi, başlatma arası boşluk, adım özeti
// ============================================================================
//
// Ölçüm açıkken (TR_GPU_PROFIL=1 ya da ProfilAc) her çekirdek başlatması ve sürücünün
// bellek işi (doldurma, kopya, yükleme) iki zamanlama olayı arasına alınır. Olaylar
// beklenmeden sıraya konur; geçilmiş olanlar arada bir (ve her eşzamanlamada) çözülür:
// çekirdeğin GPU süresi iki olayı arasındaki süredir, boşluk ise önceki işin bitişiyle
// bunun başlangıcı arasındaki süredir (GPU'nun boş beklediği zaman). Konak tarafında arka
// uçta geçen süre ve eşzamanlamada beklenen süre de toplanır. Adımlar iyileştiricinin
// adımıyla işaretlenir.
//
// Kapalıyken maliyet her başlatmada tek bir atomik okumadır. Ölçüm koşarken kapatılırsa
// bekleyen olaylar bir sonraki eşzamanlamada ya da yeniden açılınca çözülür; olaylar yok
// edilmez, havuzda yeniden kullanılır (en çok ~32 bin olay).
// Bellek için başlatılan çöp toplamalar adım başına sayılır (copKaydet).

// profilAcik süreç genelinde ölçüm bayrağı.
var profilAcik atomic.Bool

func init() {
	if os.Getenv("TR_GPU_PROFIL") == "1" {
		profilAcik.Store(true)
	}
}

// profKayit çözülmeyi bekleyen bir ölçüm.
type profKayit struct {
	ad       string
	bas, son uintptr
	adim     int32
}

// cekIstat bir çekirdeğin toplamları.
type cekIstat struct {
	sayi int64
	ms   float64
}

// adimIstat bir adımın toplamları.
type adimIstat struct {
	duvar, konak, esit time.Duration
	gpuMs, boslukMs    float64
	baslatma           int64
	cop                int64         // bellek için başlatılan çöp toplama
	copSure            time.Duration // çöp toplamada beklenen süre
}

// profil bir arka ucun ölçüm durumu. Olay işleri arka ucun kilidi altında yapılır;
// toplamlar kendi kilidiyle (mu) korunur, böylece anlık görüntü arka ucu beklemez.
type profil struct {
	destek    int8 // 0 denenmedi, 1 var, -1 yok (sürücü olayları vermiyor)
	bekleyen  []profKayit
	bosOlay   []uintptr
	oncekiSon uintptr
	sonCozum  time.Time
	girisAni  time.Time
	esitBas   time.Time

	mu      sync.Mutex
	istat   map[string]*cekIstat
	adimlar []adimIstat
	adimBas time.Time
	cagri   int64 // başlatma sayısı (bellek işleri dahil)
	yazilan int64 // son özet yazıldığındaki cagri
}

// adimNo şimdiki adımın sırası (işaretlenmemişse 0).
func (p *profil) adimNo() int32 { return int32(len(p.adimlar)) }

// olcBasla ölçüm açıksa başlangıç olayını sıraya koyar; kapalıysa 0 döner.
func (g *arkaUc) olcBasla() uintptr {
	if !profilAcik.Load() {
		return 0
	}
	p := &g.prof
	if p.destek == 0 {
		p.destek = -1
		if ev, s := g.sr.olayOlustur(); s == basarili {
			p.destek = 1
			p.bosOlay = append(p.bosOlay, ev)
			p.mu.Lock()
			p.istat = map[string]*cekIstat{}
			p.mu.Unlock()
		}
	}
	if p.destek < 0 {
		return 0
	}
	ev := g.olayAl()
	if ev == 0 || g.sr.olayKaydet(ev) != basarili {
		return 0
	}
	return ev
}

// olcBitir bitiş olayını koyar ve ölçümü bekleyenlere ekler.
func (g *arkaUc) olcBitir(ad string, bas uintptr) {
	if bas == 0 {
		return
	}
	p := &g.prof
	son := g.olayAl()
	if son == 0 || g.sr.olayKaydet(son) != basarili {
		p.bosOlay = append(p.bosOlay, bas)
		return
	}
	p.bekleyen = append(p.bekleyen, profKayit{ad: ad, bas: bas, son: son, adim: p.adimNo()})
	switch {
	case len(p.bekleyen) >= 16384:
		g.esitle() // olaylar tükenmesin (yalnızca ölçüm kipinde)
	case len(p.bekleyen) >= 512 || time.Since(p.sonCozum) > 250*time.Millisecond:
		g.olcCoz()
	}
}

func (g *arkaUc) olayAl() uintptr {
	p := &g.prof
	if n := len(p.bosOlay); n > 0 {
		ev := p.bosOlay[n-1]
		p.bosOlay = p.bosOlay[:n-1]
		return ev
	}
	ev, s := g.sr.olayOlustur()
	if s != basarili {
		return 0
	}
	return ev
}

// olcCoz geçilmiş ölçümleri (sırayla, ilk geçilmemişe kadar) toplamlara ekler.
func (g *arkaUc) olcCoz() {
	p := &g.prof
	p.sonCozum = time.Now()
	n := 0
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.bekleyen {
		if !g.sr.olayHazir(k.son) {
			break
		}
		ms, _ := g.sr.olaySure(k.bas, k.son)
		ist := p.istat[k.ad]
		if ist == nil {
			ist = &cekIstat{}
			p.istat[k.ad] = ist
		}
		ist.sayi++
		ist.ms += float64(ms)
		a := p.adimAl(k.adim)
		a.gpuMs += float64(ms)
		a.baslatma++
		if p.oncekiSon != 0 {
			if b, s := g.sr.olaySure(p.oncekiSon, k.bas); s == basarili && b > 0 {
				a.boslukMs += float64(b)
			}
			p.bosOlay = append(p.bosOlay, p.oncekiSon)
		}
		p.bosOlay = append(p.bosOlay, k.bas)
		p.oncekiSon = k.son
		p.cagri++
		n++
	}
	p.bekleyen = append(p.bekleyen[:0], p.bekleyen[n:]...)
}

// adimAl i. adımın toplamlarını verir (0: ilk işaretten önce).
func (p *profil) adimAl(i int32) *adimIstat {
	if i == 0 || int(i) > len(p.adimlar) {
		return &adimIstat{} // işaretsiz bölge: yalnızca çekirdek toplamlarına girer
	}
	return &p.adimlar[i-1]
}

// adimIsaretle yeni bir eğitim adımını başlatır (iyileştiricinin adımı çağırır).
func (g *arkaUc) adimIsaretle() {
	if !profilAcik.Load() || g.prof.destek < 0 {
		return
	}
	p := &g.prof
	simdi := time.Now()
	p.mu.Lock()
	if n := len(p.adimlar); n > 0 {
		p.adimlar[n-1].duvar = simdi.Sub(p.adimBas)
	}
	p.adimlar = append(p.adimlar, adimIstat{})
	p.adimBas = simdi
	p.mu.Unlock()
}

// copKaydet bellek için başlatılan bir çöp toplamayı şimdiki adıma yazar.
func (p *profil) copKaydet(d time.Duration) {
	if !profilAcik.Load() {
		return
	}
	p.mu.Lock()
	if n := len(p.adimlar); n > 0 {
		p.adimlar[n-1].cop++
		p.adimlar[n-1].copSure += d
	}
	p.mu.Unlock()
}

// konakSuresi arka uç girişinden çıkışa geçen süreyi şimdiki adıma ekler (gir/cik).
func (g *arkaUc) konakBasla() {
	if profilAcik.Load() {
		g.prof.girisAni = time.Now()
	}
}

func (g *arkaUc) konakBitir() {
	p := &g.prof
	if p.girisAni.IsZero() {
		return
	}
	d := time.Since(p.girisAni)
	p.girisAni = time.Time{}
	if n := len(p.adimlar); n > 0 && profilAcik.Load() {
		p.mu.Lock()
		p.adimlar[n-1].konak += d
		p.mu.Unlock()
	}
}

// esitSuresi bir eşzamanlamada beklenen süreyi kaydeder ve bekleyen ölçümleri çözer.
func (g *arkaUc) esitBasla() {
	if profilAcik.Load() {
		g.prof.esitBas = time.Now()
	}
}

func (g *arkaUc) esitBitir() {
	p := &g.prof
	if p.esitBas.IsZero() {
		return
	}
	d := time.Since(p.esitBas)
	p.esitBas = time.Time{}
	if n := len(p.adimlar); n > 0 {
		p.mu.Lock()
		p.adimlar[n-1].esit += d
		p.mu.Unlock()
	}
	if len(p.bekleyen) > 0 {
		g.olcCoz()
	}
}

// cekirdekAilesi çekirdek adının ailesi: ilk "_<rakam>" ya da "_true/_false" ekinden önceki
// kısım (ör. "carpim_128_128_8" → "carpim", "tekli_turev_3_" → "tekli_turev").
func cekirdekAilesi(ad string) string {
	parca := strings.Split(ad, "_")
	for i, p := range parca {
		if i > 0 && (p == "" || p[0] >= '0' && p[0] <= '9' || p == "true" || p == "false") {
			return strings.Join(parca[:i], "_")
		}
	}
	return ad
}

// sirali çekirdek toplamlarını GPU süresine göre azalan sırada verir.
func (p *profil) sirali() []CekirdekOlcumu {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := make([]CekirdekOlcumu, 0, len(p.istat))
	for ad, s := range p.istat {
		r = append(r, CekirdekOlcumu{Ad: ad, Aile: cekirdekAilesi(ad), Cagri: s.sayi,
			GPUSuresi: time.Duration(s.ms * float64(time.Millisecond))})
	}
	sort.Slice(r, func(i, j int) bool {
		if r[i].GPUSuresi != r[j].GPUSuresi {
			return r[i].GPUSuresi > r[j].GPUSuresi
		}
		return r[i].Ad < r[j].Ad
	})
	return r
}

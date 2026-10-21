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

// profileOn süreç genelinde ölçüm bayrağı.
var profileOn atomic.Bool

func init() {
	if os.Getenv("TR_GPU_PROFIL") == "1" {
		profileOn.Store(true)
	}
}

// profRecord çözülmeyi bekleyen bir ölçüm.
type profRecord struct {
	ad          string
	begin, last uintptr
	step        int32
}

// pullStats bir çekirdeğin toplamları.
type pullStats struct {
	num int64
	ms  float64
}

// stepStats bir adımın toplamları.
type stepStats struct {
	wall, host, equal time.Duration
	gpuMS, idleMs     float64
	launching         int64
	gc                int64         // bellek için başlatılan çöp toplama
	gcTime            time.Duration // çöp toplamada beklenen süre
	// done adım bir sonraki işaretle kapandı (duvar ölçüldü). Tamamlanmış adım süreye göre
	// değil buna göre seçilir: Windows'ta konak saati kabadır (0,5-15,6 ms), kısa bir adımın
	// duvar süresi 0 ölçülebilir.
	done bool
}

// profile bir arka ucun ölçüm durumu. Olay işleri arka ucun kilidi altında yapılır;
// toplamlar kendi kilidiyle (mu) korunur, böylece anlık görüntü arka ucu beklemez.
type profile struct {
	support      int8 // 0 denenmedi, 1 var, -1 yok (sürücü olayları vermiyor)
	pending      []profRecord
	emptyEvent   []uintptr
	prevEnd      uintptr
	lastResolved time.Time
	entryTime    time.Time
	equalStart   time.Time

	mu        sync.Mutex
	stats     map[string]*pullStats
	steps     []stepStats
	stepStart time.Time
	callN     int64 // başlatma sayısı (bellek işleri dahil)
	written   int64 // son özet yazıldığındaki cagri
}

// stepNo şimdiki adımın sırası (işaretlenmemişse 0).
func (p *profile) stepNo() int32 { return int32(len(p.steps)) }

// measureStart ölçüm açıksa başlangıç olayını sıraya koyar; kapalıysa 0 döner.
func (g *backend) measureStart() uintptr {
	if !profileOn.Load() {
		return 0
	}
	p := &g.profiler
	if p.support == 0 {
		p.support = -1
		if ev, s := g.sr.createEvent(); s == succeeded {
			p.support = 1
			p.emptyEvent = append(p.emptyEvent, ev)
			p.mu.Lock()
			p.stats = map[string]*pullStats{}
			p.mu.Unlock()
		}
	}
	if p.support < 0 {
		return 0
	}
	ev := g.takeEvent()
	if ev == 0 || g.sr.recordEvent(ev) != succeeded {
		return 0
	}
	return ev
}

// measureEnd bitiş olayını koyar ve ölçümü bekleyenlere ekler.
func (g *backend) measureEnd(ad string, begin uintptr) {
	if begin == 0 {
		return
	}
	p := &g.profiler
	last := g.takeEvent()
	if last == 0 || g.sr.recordEvent(last) != succeeded {
		p.emptyEvent = append(p.emptyEvent, begin)
		return
	}
	p.pending = append(p.pending, profRecord{ad: ad, begin: begin, last: last, step: p.stepNo()})
	switch {
	case len(p.pending) >= 16384:
		g.equalize() // olaylar tükenmesin (yalnızca ölçüm kipinde)
	case len(p.pending) >= 512 || time.Since(p.lastResolved) > 250*time.Millisecond:
		g.measureParse()
	}
}

func (g *backend) takeEvent() uintptr {
	p := &g.profiler
	if n := len(p.emptyEvent); n > 0 {
		ev := p.emptyEvent[n-1]
		p.emptyEvent = p.emptyEvent[:n-1]
		return ev
	}
	ev, s := g.sr.createEvent()
	if s != succeeded {
		return 0
	}
	return ev
}

// measureParse geçilmiş ölçümleri (sırayla, ilk geçilmemişe kadar) toplamlara ekler.
func (g *backend) measureParse() {
	p := &g.profiler
	p.lastResolved = time.Now()
	n := 0
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.pending {
		if !g.sr.eventReady(k.last) {
			break
		}
		ms, _ := g.sr.eventTime(k.begin, k.last)
		st := p.stats[k.ad]
		if st == nil {
			st = &pullStats{}
			p.stats[k.ad] = st
		}
		st.num++
		st.ms += float64(ms)
		a := p.takeStep(k.step)
		a.gpuMS += float64(ms)
		a.launching++
		if p.prevEnd != 0 {
			if b, s := g.sr.eventTime(p.prevEnd, k.begin); s == succeeded && b > 0 {
				a.idleMs += float64(b)
			}
			p.emptyEvent = append(p.emptyEvent, p.prevEnd)
		}
		p.emptyEvent = append(p.emptyEvent, k.begin)
		p.prevEnd = k.last
		p.callN++
		n++
	}
	p.pending = append(p.pending[:0], p.pending[n:]...)
}

// takeStep i. adımın toplamlarını verir (0: ilk işaretten önce).
func (p *profile) takeStep(i int32) *stepStats {
	if i == 0 || int(i) > len(p.steps) {
		return &stepStats{} // işaretsiz bölge: yalnızca çekirdek toplamlarına girer
	}
	return &p.steps[i-1]
}

// markStep yeni bir eğitim adımını başlatır (iyileştiricinin adımı çağırır).
func (g *backend) markStep() {
	if !profileOn.Load() || g.profiler.support < 0 {
		return
	}
	p := &g.profiler
	now := time.Now()
	p.mu.Lock()
	if n := len(p.steps); n > 0 {
		p.steps[n-1].wall = now.Sub(p.stepStart)
		p.steps[n-1].done = true
	}
	p.steps = append(p.steps, stepStats{})
	p.stepStart = now
	p.mu.Unlock()
}

// recordGC bellek için başlatılan bir çöp toplamayı şimdiki adıma yazar.
func (p *profile) recordGC(d time.Duration) {
	if !profileOn.Load() {
		return
	}
	p.mu.Lock()
	if n := len(p.steps); n > 0 {
		p.steps[n-1].gc++
		p.steps[n-1].gcTime += d
	}
	p.mu.Unlock()
}

// konakSuresi arka uç girişinden çıkışa geçen süreyi şimdiki adıma ekler (gir/cik).
func (g *backend) hostStart() {
	if profileOn.Load() {
		g.profiler.entryTime = time.Now()
	}
}

func (g *backend) hostEnd() {
	p := &g.profiler
	if p.entryTime.IsZero() {
		return
	}
	d := time.Since(p.entryTime)
	p.entryTime = time.Time{}
	if n := len(p.steps); n > 0 && profileOn.Load() {
		p.mu.Lock()
		p.steps[n-1].host += d
		p.mu.Unlock()
	}
}

// esitSuresi bir eşzamanlamada beklenen süreyi kaydeder ve bekleyen ölçümleri çözer.
func (g *backend) equalBegin() {
	if profileOn.Load() {
		g.profiler.equalStart = time.Now()
	}
}

func (g *backend) equalFinish() {
	p := &g.profiler
	if p.equalStart.IsZero() {
		return
	}
	d := time.Since(p.equalStart)
	p.equalStart = time.Time{}
	if n := len(p.steps); n > 0 {
		p.mu.Lock()
		p.steps[n-1].equal += d
		p.mu.Unlock()
	}
	if len(p.pending) > 0 {
		g.measureParse()
	}
}

// kernelFamily çekirdek adının ailesi: ilk "_<rakam>" ya da "_true/_false" ekinden önceki
// kısım (ör. "carpim_128_128_8" → "carpim", "tekli_turev_3_" → "tekli_turev"). Kitaplığın
// matris çarpımları ve tek satırlık çarpımlar (gemv) da "carpim" ailesindedir (çekirdek
// tablosunda adlarıyla ayrılırlar).
func kernelFamily(ad string) string {
	if strings.HasPrefix(ad, "cublaslt_carpim") || strings.HasPrefix(ad, "gemv_") {
		return "carpim"
	}
	part := strings.Split(ad, "_")
	for i, p := range part {
		if i > 0 && (p == "" || p[0] >= '0' && p[0] <= '9' || p == "true" || p == "false") {
			return strings.Join(part[:i], "_")
		}
	}
	return ad
}

// sorted çekirdek toplamlarını GPU süresine göre azalan sırada verir.
func (p *profile) sorted() []KernelSample {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := make([]KernelSample, 0, len(p.stats))
	for ad, s := range p.stats {
		r = append(r, KernelSample{Name: ad, Family: kernelFamily(ad), Calls: s.num,
			GPUTime: time.Duration(s.ms * float64(time.Millisecond))})
	}
	sort.Slice(r, func(i, j int) bool {
		if r[i].GPUTime != r[j].GPUTime {
			return r[i].GPUTime > r[j].GPUTime
		}
		return r[i].Name < r[j].Name
	})
	return r
}

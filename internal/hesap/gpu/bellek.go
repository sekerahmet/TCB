package gpu

import (
	"fmt"
	"math/bits"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Cihaz belleği: depolama, boyut sınıflı havuz, konak hazırlık halkası
// ============================================================================
//
// Havuz: ayırmalar boyut sınıflarına yuvarlanır (256 bayt .. 64 MB ikinin kuvvetleri,
// üstü 2 MB'nin katları) ve serbest kalan bloklar sürücüye geri verilmeden sınıfın
// listesinde bekler. Bir depolama iki yoldan havuza döner: otomatik türevin işi biten
// ara gradyanları açıkça iade etmesi (Release) ya da Go çöp toplayıcısının ulaşılamaz
// depolamayı temizlemesi (runtime.AddCleanup). Bütün işler tek akışta sırayla
// yürüdüğünden, havuza dönen bir bloğu kullanan eski çekirdekler, bloğu yeniden kullanan
// yeni işlerden önce biter; bu yüzden iadede eşzamanlama gerekmez.
//
// Go'nun çöp toplayıcısı cihaz belleğini görmez (küçük Go nesneleri büyük cihaz
// bloklarını tutar). Bu yüzden son toplamadan bu yana ayrılan cihaz belleği eşiği
// aşınca (toplam belleğin 1/8'i) toplama açıkça başlatılır; ayırma başarısız olursa
// önce toplama ve havuzun sürücüye boşaltılması denenir, yine olmazsa Türkçe ve
// yakalanabilir bir hata verilir.
//
// Konak gölgesi: konaktan yüklenen küçük float32 depolamalar (≤ 1M eleman; ör. veri
// kümesinden gelen indeks toplu işleri) konaktaki kopyalarını da tutar. Depolama cihazda
// değiştirilmediği sürece konağa okuma (indeksler, .değer) eşzamanlama gerektirmez.
// Gölgenin silinmesi merkezîdir: cihaz belleğine yazan yollar yalnızca ikidir — çekirdek
// başlatma (baslatici.go: göstericiler depolamayla ve okuma/yazma bildirimiyle verilir,
// bildirim çekirdeğin imzasıyla denetlenir, yazılan depolamaların gölgesi başlatmada
// silinir) ve bu dosyadaki sürücü yazma sarmalayıcıları (dKopyala, sifirla, doldur32,
// yukleKonak); hepsi hedef depolamayı alır ve gölgesini siler. Sürücünün yazma işlevleri
// başka yerden çağrılmaz.

// depo bir GPU depolaması.
type depo struct {
	tensor.VersionCounter
	g     *arkaUc
	adr   uint64
	n     int
	tur   tensor.DType
	sinif uint64
	konak []float32
	iade  bool
	temiz runtime.Cleanup
}

func (d *depo) Device() tensor.Device { return d.g.dev }
func (d *depo) Len() int              { return d.n }
func (d *depo) DType() tensor.DType   { return d.tur }

// golgeSiniri bundan büyük depolamalar konak gölgesi tutmaz.
const golgeSiniri = 1 << 20

// depoOf depolamanın bu arka uca ait GPU depolaması olduğunu denetler.
func (g *arkaUc) depoOf(s tensor.Storage) *depo {
	d, ok := s.(*depo)
	if !ok || d.g != g {
		panic(fmt.Sprintf("tensor: %s arka ucuna %s cihazından depolama verildi; işlemden önce tensörleri aynı cihaza taşıyın", g.dev, s.Device()))
	}
	if d.iade {
		panic("tensor: GPU: havuza iade edilmiş bir depolama kullanıldı (iç hata)")
	}
	return d
}

// adresi görünümün ilk elemanının cihaz adresi.
func (g *arkaUc) adresi(v tensor.View) uint64 {
	d := g.depoOf(v.S)
	return d.adr + uint64(v.Offset*turBoyu(d.tur))
}

// havuz boyut sınıflı serbest blok listeleri.
type havuz struct {
	mu   sync.Mutex
	bos  map[uint64][]uint64
	yeni uint64 // son çöp toplamadan bu yana sürücüden ayrılan bayt
}

func sinifBoyu(bayt uint64) uint64 {
	switch {
	case bayt <= 256:
		return 256
	case bayt <= 64<<20:
		return 1 << bits.Len64(bayt-1)
	}
	return (bayt + (2<<20 - 1)) &^ (2<<20 - 1)
}

// iade bloğu sınıfının listesine koyar (çöp toplayıcının temizlik iş parçacığından da
// çağrılır; yalnızca havuzun kilidini alır).
func (h *havuz) iade(adr, sinif uint64) {
	h.mu.Lock()
	h.bos[sinif] = append(h.bos[sinif], adr)
	h.mu.Unlock()
}

func (h *havuz) al(sinif uint64) (uint64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := h.bos[sinif]
	if len(l) == 0 {
		return 0, false
	}
	adr := l[len(l)-1]
	h.bos[sinif] = l[:len(l)-1]
	return adr, true
}

type temizlikBilgisi struct {
	h          *havuz
	adr, sinif uint64
}

// ayir n elemanlı dt türünde depolama verir; sifirla doğruysa içerik sıfırdır.
func (g *arkaUc) ayir(n int, dt tensor.DType, sifirla bool) *depo {
	if dt != tensor.Float32 && dt != tensor.BFloat16 && dt != tensor.Float16 {
		yok(dt.String() + " türünde depolama")
	}
	if n < 0 {
		panic(fmt.Sprintf("tensor: GPU: negatif eleman sayısı %d", n))
	}
	bayt := uint64(n) * uint64(turBoyu(dt))
	if bayt > g.sr.toplamBellek() {
		panic(tensor.SizeError(n, dt, int64(g.sr.toplamBellek()), "bu GPU'da"))
	}
	d := &depo{g: g, n: n, tur: dt}
	if n == 0 {
		return d
	}
	d.sinif = sinifBoyu(bayt)
	adr, ok := g.havuz.al(d.sinif)
	if !ok {
		adr = g.suruculenAyir(d.sinif, bayt)
	}
	d.adr = adr
	d.temiz = runtime.AddCleanup(d, func(t temizlikBilgisi) { t.h.iade(t.adr, t.sinif) }, temizlikBilgisi{&g.havuz, adr, d.sinif})
	if sifirla {
		g.sifirla(d)
	}
	return d
}

// sifirla depolamanın bütün baytlarını sıfırlar.
func (g *arkaUc) sifirla(d *depo) {
	if d.n == 0 {
		return
	}
	d.konak = nil
	if d.tur == tensor.Float32 {
		g.dene("cuMemsetD32Async", g.sr.doldur32(d.adr, 0, uint64(d.n)))
	} else {
		g.dene("cuMemsetD16Async", g.sr.doldur16(d.adr, 0, uint64(d.n)))
	}
}

// doldur32 float32 depolamanın bütün elemanlarını verilen bitlerle doldurur.
func (g *arkaUc) doldur32(d *depo, bitler uint32) {
	if d.n == 0 {
		return
	}
	d.konak = nil
	g.dene("cuMemsetD32Async", g.sr.doldur32(d.adr, bitler, uint64(d.n)))
}

// dKopyala cihaz içi kopya: hedefin hOfs'undan başlayarak kaynağın kOfs'undaki n eleman
// (aynı tür).
func (g *arkaUc) dKopyala(hedef *depo, hOfs int, kaynak *depo, kOfs, n int) {
	if n == 0 {
		return
	}
	hedef.konak = nil
	b := turBoyu(hedef.tur)
	g.dene("cuMemcpyDtoDAsync", g.sr.kopyala(hedef.adr+uint64(hOfs*b), kaynak.adr+uint64(kOfs*b), uint64(n*b)))
}

// suruculenAyir sürücüden yeni blok ayırır; gerekirse çöp toplamayı başlatır, bellek
// yetmezse havuzu boşaltıp yeniden dener.
func (g *arkaUc) suruculenAyir(sinif, istenen uint64) uint64 {
	esik := max(g.sr.toplamBellek()/8, 64<<20)
	if g.havuz.yeni+sinif > esik {
		g.havuz.yeni = 0
		runtime.GC()
		if adr, ok := g.havuz.al(sinif); ok {
			return adr
		}
	}
	adr, s := g.sr.ayir(sinif)
	if s == hataBellekYok {
		adr, s = g.bellekKurtar(sinif)
	}
	if s == hataBellekYok {
		bos, toplam, _ := g.sr.bosBellek()
		panic(fmt.Sprintf("tensor: GPU belleği yetmedi: %s istendi; GPU'da %s boş, %s toplam (%s). Toplu iş boyutunu, "+
			"bağlam uzunluğunu ya da modeli küçültün", baytYazi(istenen), baytYazi(bos), baytYazi(toplam), g.sr.hataAdi(s)))
	}
	g.dene("cuMemAlloc", s)
	g.havuz.yeni += sinif
	return adr
}

// bellekKurtar çöp toplayıcıyı çalıştırır, temizliklerin havuza dönmesini kısa süre
// bekler, havuzdaki bütün blokları sürücüye geri verir ve ayırmayı yeniden dener.
func (g *arkaUc) bellekKurtar(sinif uint64) (uint64, sonuc) {
	runtime.GC()
	for i := 0; i < 20; i++ {
		if adr, ok := g.havuz.al(sinif); ok {
			return adr, basarili
		}
		time.Sleep(5 * time.Millisecond)
		runtime.Gosched()
	}
	g.esitle()
	g.havuz.mu.Lock()
	bos := g.havuz.bos
	g.havuz.bos = map[uint64][]uint64{}
	g.havuz.mu.Unlock()
	for _, l := range bos {
		for _, adr := range l {
			g.dene("cuMemFree", g.sr.birak(adr))
		}
	}
	return g.sr.ayir(sinif)
}

// birak depolamayı havuza iade eder (Release).
func (g *arkaUc) birak(d *depo) {
	if d.n == 0 || d.iade {
		return
	}
	d.iade = true
	d.temiz.Stop()
	d.konak = nil
	g.havuz.iade(d.adr, d.sinif)
}

// ----------------------------------------------------------------------------
// Konak hazırlık halkası (sayfası kilitli bellek; eşzamansız yükleme)
// ----------------------------------------------------------------------------

// hazirlik eşzamansız yüklemeler için sayfası kilitli konak belleği. Yüklenecek veri
// halkaya kopyalanır ve eşzamansız kopya sıraya eklenir; halka dolunca bir kez
// eşzamanlanıp baştan kullanılır. Her eşzamanlamada ofset sıfırlanır (önceki kopyalar
// bitmiştir).
type hazirlik struct {
	taban  unsafe.Pointer
	boy    int
	ofset  int
	denedi bool
}

const (
	hazirlikBoyu  = 64 << 20
	hazirlikEnCok = 16 << 20 // bundan büyük yüklemeler eşzamanlı yapılır
)

// yukleKonak konak verisini depolamanın başına kopyalar (küçükse halka üzerinden
// eşzamansız).
func (g *arkaUc) yukleKonak(d *depo, veri unsafe.Pointer, bayt int) {
	if bayt == 0 {
		return
	}
	d.konak = nil
	hedef := d.adr
	h := &g.hazirlik
	if !h.denedi {
		h.denedi = true
		if p, s := g.sr.konakAyir(hazirlikBoyu); s == basarili {
			h.taban, h.boy = p, hazirlikBoyu
		}
	}
	if h.taban == nil || bayt > hazirlikEnCok {
		// Eşzamanlı kopya: kaynak Go belleği olabilir, çağrı dönene kadar canlı tutulur.
		g.dene("cuMemcpyHtoD", g.sr.yukle(hedef, veri, uint64(bayt), false))
		runtime.KeepAlive(veri)
		h.ofset = 0 // eşzamanlı kopya önceki işleri bekler
		return
	}
	if h.ofset+bayt > h.boy {
		g.esitle()
	}
	p := unsafe.Add(h.taban, h.ofset)
	// Eşzamansız kopyanın kaynağı her zaman halkadır (sayfası kilitli, Go dışı bellek):
	// veri halkaya kopyalandıktan sonra çağıranın dilimi serbesttir.
	copy(unsafe.Slice((*byte)(p), bayt), unsafe.Slice((*byte)(veri), bayt))
	runtime.KeepAlive(veri)
	h.ofset = (h.ofset + bayt + 255) &^ 255
	g.dene("cuMemcpyHtoDAsync", g.sr.yukle(hedef, p, uint64(bayt), true))
}

// yukleTam 32 bitlik tam sayıları yeni bir cihaz tamponuna yükler (indeksler, hedefler).
func (g *arkaUc) yukleTam(v []int32) *depo {
	d := g.ayir(len(v), tensor.Float32, false)
	if len(v) > 0 {
		g.yukleKonak(d, unsafe.Pointer(&v[0]), 4*len(v))
	}
	return d
}

// indir cihazdaki bayt baytlık bölgeyi konağa kopyalar (eşzamanlar).
func (g *arkaUc) indir(hedef unsafe.Pointer, kaynak uint64, bayt int) {
	if bayt == 0 {
		return
	}
	g.dene("cuMemcpyDtoH", g.sr.indir(hedef, kaynak, uint64(bayt)))
	g.hazirlik.ofset = 0
}

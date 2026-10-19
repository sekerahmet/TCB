package gpu

import (
	"fmt"
	"math/bits"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Cihaz belleği: depolama, boyut sınıflı havuz, konak hazırlık halkası
// ============================================================================
//
// Havuz: ayırmalar boyut sınıflarına yuvarlanır (bkz. sinifBoyu: 1 MB'ye kadar ikinin
// kuvvetleri, 64 MB'ye kadar dörtte bir adımlar, üstü 2 MB'nin katları) ve serbest kalan bloklar sürücüye geri verilmeden sınıfın
// listesinde bekler. Bir depolama iki yoldan havuza döner: otomatik türevin işi biten
// ara gradyanları açıkça iade etmesi (Release) ya da Go çöp toplayıcısının ulaşılamaz
// depolamayı temizlemesi (runtime.AddCleanup). Bütün işler tek akışta sırayla
// yürüdüğünden, havuza dönen bir bloğu kullanan eski çekirdekler, bloğu yeniden kullanan
// yeni işlerden önce biter; bu yüzden iadede eşzamanlama gerekmez.
//
// Go'nun çöp toplayıcısı cihaz belleğini görmez (küçük Go nesneleri büyük cihaz
// bloklarını tutar). Bu yüzden toplama açıkça başlatılır: son toplamadan bu yana verilen
// depolama copEsigi'ni aşınca (kart belleğine göre 256 MB–2 GB; havuzdan verilenler dahil,
// böylece havuz bir adımın çöpüyle şişmez), her geri yayılımın sonunda (BackwardDone) ve
// son toplamadan bu yana sürücüden ayrılan cihaz belleği toplamın 1/32'sini aşınca; ayırma başarısız olursa
// önce toplama ve havuzun sürücüye boşaltılması denenir, yine olmazsa Türkçe ve
// yakalanabilir bir hata verilir.
//
// Konak gölgesi: konaktan yüklenen küçük float32 depolamalar (≤ 1M eleman; ör. veri
// kümesinden gelen indeks toplu işleri) konaktaki kopyalarını da tutar. Depolama cihazda
// değiştirilmediği sürece konağa okuma (indeksler, .değer) eşzamanlama gerektirmez.
// Gölgenin silinmesi merkezîdir: cihaz belleğine yazan yollar yalnızca üçtür — çekirdek
// başlatma (baslatici.go: göstericiler depolamayla ve okuma/yazma bildirimiyle verilir,
// bildirim çekirdeğin imzasıyla denetlenir, yazılan depolamaların gölgesi başlatmada
// silinir), matris çarpımı kitaplığının çağrısı (gemm hedefin gölgesini siler) ve bu
// dosyadaki sürücü yazma sarmalayıcıları (dKopyala, sifirla, doldur32,
// yukleKonak); hepsi hedef depolamayı alır ve gölgesini siler. Sürücünün yazma işlevleri
// başka yerden çağrılmaz.

// depo bir GPU depolaması.
type storage struct {
	tensor.VersionCounter
	g        *backend
	addr     uint64
	n        int
	dtype    tensor.DType
	class    uint64
	host     []float32
	release2 bool
	clean    runtime.Cleanup
}

func (d *storage) Device() tensor.Device { return d.g.dev }
func (d *storage) Len() int              { return d.n }
func (d *storage) DType() tensor.DType   { return d.dtype }

// golgeSiniri bundan büyük depolamalar konak gölgesi tutmaz.
const shadowLimit = 1 << 20

// depoOf depolamanın bu arka uca ait GPU depolaması olduğunu denetler.
func (g *backend) storageOf(s tensor.Storage) *storage {
	d, ok := s.(*storage)
	if !ok || d.g != g {
		panic(fmt.Sprintf("tensor: %s arka ucuna %s cihazından depolama verildi; işlemden önce tensörleri aynı cihaza taşıyın", g.dev, s.Device()))
	}
	if d.release2 {
		panic("tensor: GPU: havuza iade edilmiş bir depolama kullanıldı (iç hata)")
	}
	return d
}

// adresi görünümün ilk elemanının cihaz adresi.
func (g *backend) addressOf(v tensor.View) uint64 {
	d := g.storageOf(v.S)
	return d.addr + uint64(v.Offset*dtypeSize(d.dtype))
}

// havuz boyut sınıflı serbest blok listeleri ve bellek sayaçları (bayt).
type pool struct {
	mu     sync.Mutex
	empty  map[uint64][]uint64
	fresh2 uint64 // son çöp toplamadan bu yana sürücüden ayrılan bayt
	// Sayaçlar: sürücüden ayrılmış, havuzda boşta, tensörlere verilmiş (canlı).
	reserved, pooled, live uint64
	peakLive, peakReserved uint64
	gcRuns, driverAllocs   int64
}

// sinifBoyu ayırmanın yuvarlandığı boy: 256 bayta kadar 256; 1 MB'ye kadar ikinin
// kuvvetleri; 1 MB–64 MB arası her ikinin kuvveti aralığı dört eşit sınıfa bölünür (en çok
// %25 boşluk); üstü 2 MB'nin katları.
func classSize(byteN uint64) uint64 {
	switch {
	case byteN <= 256:
		return 256
	case byteN <= 1<<20:
		return 1 << bits.Len64(byteN-1)
	case byteN <= 64<<20:
		step := uint64(1) << (bits.Len64(byteN-1) - 3) // aralığın dörtte biri
		return (byteN + step - 1) &^ (step - 1)
	}
	return (byteN + (2<<20 - 1)) &^ (2<<20 - 1)
}

// iade bloğu sınıfının listesine koyar (çöp toplayıcının temizlik iş parçacığından da
// çağrılır; yalnızca havuzun kilidini alır).
func (h *pool) release2(addr, class uint64) {
	h.mu.Lock()
	h.empty[class] = append(h.empty[class], addr)
	h.pooled += class
	h.live -= class
	h.mu.Unlock()
}

func (h *pool) al(class uint64) (uint64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := h.empty[class]
	if len(l) == 0 {
		return 0, false
	}
	addr := l[len(l)-1]
	h.empty[class] = l[:len(l)-1]
	h.pooled -= class
	h.addLive(class)
	return addr, true
}

// canliEkle canlı sayaçlarını artırır (kilit tutulurken).
func (h *pool) addLive(class uint64) {
	h.live += class
	h.peakLive = max(h.peakLive, h.live)
}

// yeniBlok sürücüden yeni ayrılan bloğu sayar.
func (h *pool) newBlock(class uint64) {
	h.mu.Lock()
	h.reserved += class
	h.peakReserved = max(h.peakReserved, h.reserved)
	h.driverAllocs++
	h.addLive(class)
	h.mu.Unlock()
}

// olcum sayaçların anlık görüntüsü.
func (h *pool) measurement() MemorySample {
	h.mu.Lock()
	defer h.mu.Unlock()
	return MemorySample{Reserved: h.reserved, Pooled: h.pooled, Live: h.live, PeakLive: h.peakLive,
		PeakReserved: h.peakReserved, GCRuns: h.gcRuns, DriverAllocs: h.driverAllocs}
}

type cleanupInfo struct {
	h           *pool
	addr, class uint64
}

// ayir n elemanlı dt türünde depolama verir; sifirla doğruysa içerik sıfırdır.
func (g *backend) allocate(n int, dt tensor.DType, reset bool) *storage {
	if dt != tensor.Float32 && dt != tensor.BFloat16 && dt != tensor.Float16 {
		none(dt.String() + " türünde depolama")
	}
	if n < 0 {
		panic(fmt.Sprintf("tensor: GPU: negatif eleman sayısı %d", n))
	}
	byteN := uint64(n) * uint64(dtypeSize(dt))
	if byteN > g.sr.totalMemory() {
		panic(tensor.SizeError(n, dt, int64(g.sr.totalMemory()), "bu GPU'da"))
	}
	d := &storage{g: g, n: n, dtype: dt}
	if n == 0 {
		return d
	}
	d.class = classSize(byteN)
	if g.gcThreshold > 0 && g.volume > g.gcThreshold {
		g.collectGarbage(0)
	}
	g.volume += d.class
	addr, ok := g.pool.al(d.class)
	if !ok {
		addr = g.driverAlloc(d.class, byteN)
	}
	d.addr = addr
	d.clean = runtime.AddCleanup(d, func(t cleanupInfo) { t.h.release2(t.addr, t.class) }, cleanupInfo{&g.pool, addr, d.class})
	if reset {
		g.reset(d)
	}
	return d
}

// sifirla depolamanın bütün baytlarını sıfırlar.
func (g *backend) reset(d *storage) {
	if d.n == 0 {
		return
	}
	d.host = nil
	ev := g.measureStart()
	if d.dtype == tensor.Float32 {
		g.try_("cuMemsetD32Async", g.sr.fill32(d.addr, 0, uint64(d.n)))
	} else {
		g.try_("cuMemsetD16Async", g.sr.fill16(d.addr, 0, uint64(d.n)))
	}
	g.measureEnd("surucu_sifirla", ev)
}

// doldur32 float32 depolamanın bütün elemanlarını verilen bitlerle doldurur.
func (g *backend) fill32(d *storage, bitsList uint32) {
	if d.n == 0 {
		return
	}
	d.host = nil
	ev := g.measureStart()
	g.try_("cuMemsetD32Async", g.sr.fill32(d.addr, bitsList, uint64(d.n)))
	g.measureEnd("surucu_doldur", ev)
}

// dKopyala cihaz içi kopya: hedefin hOfs'undan başlayarak kaynağın kOfs'undaki n eleman
// (aynı tür).
func (g *backend) dCopy(target *storage, hOfs int, source *storage, kOfs, n int) {
	if n == 0 {
		return
	}
	target.host = nil
	b := dtypeSize(target.dtype)
	ev := g.measureStart()
	g.try_("cuMemcpyDtoDAsync", g.sr.copyFn(target.addr+uint64(hOfs*b), source.addr+uint64(kOfs*b), uint64(n*b)))
	g.measureEnd("surucu_kopya", ev)
}

// suruculenAyir sürücüden yeni blok ayırır; gerekirse çöp toplamayı başlatır, bellek
// yetmezse havuzu boşaltıp yeniden dener.
func (g *backend) driverAlloc(class, requested uint64) uint64 {
	if g.pool.fresh2+class > max(g.sr.totalMemory()/32, 256<<20) {
		if addr, ok := g.collectGarbage(class); ok {
			return addr
		}
	}
	addr, s := g.sr.allocate(class)
	if s == errNoMemory {
		addr, s = g.reclaimMemory(class)
	}
	if s == errNoMemory {
		empty, total, _ := g.sr.freeMemory()
		panic(fmt.Sprintf("tensor: GPU belleği yetmedi: %s istendi; GPU'da %s boş, %s toplam (%s). Toplu iş boyutunu, "+
			"bağlam uzunluğunu ya da modeli küçültün", bytesText(requested), bytesText(empty), bytesText(total), g.sr.errName(s)))
	}
	g.try_("cuMemAlloc", s)
	g.pool.fresh2 += class
	g.pool.newBlock(class)
	return addr
}

// copTopla Go çöp toplayıcısını çalıştırır: ulaşılamayan depolamaların temizlikleri
// blokları havuza döndürür. Temizlikler ayrı bir gorutinde koştuğundan sınıfın bloğu kısa
// süre beklenir; gelirse verilir.
func (g *backend) collectGarbage(class uint64) (uint64, bool) {
	g.pool.fresh2, g.volume = 0, 0
	begin := time.Now()
	runtime.GC()
	g.profiler.recordGC(time.Since(begin))
	g.pool.mu.Lock()
	g.pool.gcRuns++
	g.pool.mu.Unlock()
	if class == 0 {
		return 0, false
	}
	for i := 0; i < 64; i++ {
		if addr, ok := g.pool.al(class); ok {
			return addr, true
		}
		runtime.Gosched()
	}
	return 0, false
}

// BackwardDone (tensor.BackwardFinisher) bir geri yayılım bittiğinde çağrılır: ileri
// yönün ara tensörleri artık ulaşılamazdır. Son toplamadan bu yana verilen bellek yeterince
// büyükse çöp toplama başlatılır; böylece bloklar bir sonraki ileri yönden önce havuza
// döner ve tepe bellek bir adımın ara tensörleriyle sınırlı kalır.
func (g *backend) BackwardDone() {
	g.enter()
	defer g.exit()
	if g.volume > max(256<<20, g.pool.measurement().Reserved/4) {
		g.collectGarbage(0)
	}
}

// bellekKurtar çöp toplayıcıyı çalıştırır, temizliklerin havuza dönmesini kısa süre
// bekler, havuzdaki bütün blokları sürücüye geri verir ve ayırmayı yeniden dener.
func (g *backend) reclaimMemory(class uint64) (uint64, result) {
	runtime.GC()
	for i := 0; i < 20; i++ {
		if addr, ok := g.pool.al(class); ok {
			return addr, succeeded
		}
		time.Sleep(5 * time.Millisecond)
		runtime.Gosched()
	}
	g.equalize()
	g.pool.mu.Lock()
	empty := g.pool.empty
	g.pool.empty = map[uint64][]uint64{}
	g.pool.mu.Unlock()
	for class, l := range empty {
		for _, addr := range l {
			g.try_("cuMemFree", g.sr.release(addr))
			g.pool.mu.Lock()
			g.pool.reserved -= class
			g.pool.pooled -= class
			g.pool.mu.Unlock()
		}
	}
	return g.sr.allocate(class)
}

// birak depolamayı havuza iade eder (Release).
func (g *backend) release(d *storage) {
	if d.n == 0 || d.release2 {
		return
	}
	d.release2 = true
	d.clean.Stop()
	d.host = nil
	g.pool.release2(d.addr, d.class)
}

// ----------------------------------------------------------------------------
// Konak hazırlık halkası (sayfası kilitli bellek; eşzamansız yükleme)
// ----------------------------------------------------------------------------

// hazirlik eşzamansız yüklemeler için sayfası kilitli konak belleği. Yüklenecek veri
// halkaya kopyalanır ve eşzamansız kopya sıraya eklenir; halka dolunca bir kez
// eşzamanlanıp baştan kullanılır. Her eşzamanlamada ofset sıfırlanır (önceki kopyalar
// bitmiştir).
type setup struct {
	base2  unsafe.Pointer
	size   int
	offset int
	tried  bool
}

const (
	setupSize = 64 << 20
	setupMax  = 16 << 20 // bundan büyük yüklemeler eşzamanlı yapılır
)

// yukleKonak konak verisini depolamanın başına kopyalar (küçükse halka üzerinden
// eşzamansız).
func (g *backend) loadHost(d *storage, data unsafe.Pointer, byteN int) {
	if byteN == 0 {
		return
	}
	d.host = nil
	target := d.addr
	h := &g.setup
	if !h.tried {
		h.tried = true
		if p, s := g.sr.hostAlloc(setupSize); s == succeeded {
			h.base2, h.size = p, setupSize
		}
	}
	if h.base2 == nil || byteN > setupMax {
		// Eşzamanlı kopya: kaynak Go belleği olabilir, çağrı dönene kadar canlı tutulur.
		g.equalBegin()
		g.try_("cuMemcpyHtoD", g.sr.load(target, data, uint64(byteN), false))
		g.equalFinish()
		runtime.KeepAlive(data)
		h.offset = 0 // eşzamanlı kopya önceki işleri bekler
		return
	}
	if h.offset+byteN > h.size {
		g.equalize()
	}
	p := unsafe.Add(h.base2, h.offset)
	// Eşzamansız kopyanın kaynağı her zaman halkadır (sayfası kilitli, Go dışı bellek):
	// veri halkaya kopyalandıktan sonra çağıranın dilimi serbesttir.
	copy(unsafe.Slice((*byte)(p), byteN), unsafe.Slice((*byte)(data), byteN))
	runtime.KeepAlive(data)
	h.offset = (h.offset + byteN + 255) &^ 255
	ev := g.measureStart()
	g.try_("cuMemcpyHtoDAsync", g.sr.load(target, p, uint64(byteN), true))
	g.measureEnd("konaktan_yukle", ev)
}

// yukleTam 32 bitlik tam sayıları yeni bir cihaz tamponuna yükler (indeksler, hedefler).
func (g *backend) loadInt(v []int32) *storage {
	d := g.allocate(len(v), tensor.Float32, false)
	if len(v) > 0 {
		g.loadHost(d, unsafe.Pointer(&v[0]), 4*len(v))
	}
	return d
}

// indir cihazdaki bayt baytlık bölgeyi konağa kopyalar (eşzamanlar).
func (g *backend) download(target unsafe.Pointer, source uint64, byteN int) {
	if byteN == 0 {
		return
	}
	g.equalBegin()
	g.try_("cuMemcpyDtoH", g.sr.download(target, source, uint64(byteN)))
	g.setup.offset = 0
	g.equalFinish()
}

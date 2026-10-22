package gpu

import (
	"math/bits"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Cihaz belleği: depolama, boyut sınıflı havuz, konak hazırlık halkası
// ============================================================================
//
// Havuz: 1 MB'ye kadarki ayırmalar ikinin kuvveti sınıflarına yuvarlanır ve serbest kalan
// bloklar sürücüye geri verilmeden sınıfın listesinde bekler; daha büyükleri sürücüden
// alınan parçalardan en uygun boş bloğa bölünerek verilir ve serbest kalınca komşularıyla
// birleşir (memory_blocks.go). Bir depolama iki yoldan havuza döner: otomatik türevin işi biten
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

// storage bir GPU depolaması.
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
	// priv bloğun ait olduğu kaydın havuzu (kayıt sırasında ayrıldıysa; yoksa nil), want o
	// havuzdaki istek boyu (graph.go).
	priv *privatePool
	want uint64
	// pins bu depolamayı okuyan ya da yazan canlı kayıt sayısı (graph.go: kayıttan önce
	// ayrılmış olup kayda giren depolamalar); parked, kayıt yaşarken bırakılmış ve iadesi kaydın
	// bırakılmasına ertelenmiş olduğunu gösterir.
	pins   int
	parked bool
}

func (d *storage) Device() tensor.Device { return d.g.dev }
func (d *storage) Len() int              { return d.n }
func (d *storage) DType() tensor.DType   { return d.dtype }

// shadowLimit bundan büyük depolamalar konak gölgesi tutmaz.
const shadowLimit = 1 << 20

// storageOf depolamanın bu arka uca ait GPU depolaması olduğunu denetler. Kayıt sürerken
// depolama kayda girer (privatePool.hold): kayıt yaşadıkça belleği başka işe verilmez.
func (g *backend) storageOf(s tensor.Storage) *storage {
	d, ok := s.(*storage)
	if !ok || d.g != g {
		panic(messages.New(messages.H4015, g.dev, s.Device()))
	}
	if d.release2 {
		panic(internalf("storage already returned to the pool was used"))
	}
	if c := g.capturing; c != nil {
		c.hold(d)
	}
	return d
}

// addressOf görünümün ilk elemanının cihaz adresi.
func (g *backend) addressOf(v tensor.View) uint64 {
	d := g.storageOf(v.S)
	return d.addr + uint64(v.Offset*dtypeSize(d.dtype))
}

// pool boyut sınıflı serbest blok listeleri ve bellek sayaçları (bayt).
type pool struct {
	mu     sync.Mutex
	empty  map[uint64][]uint64
	fresh2 uint64 // son çöp toplamadan bu yana sürücüden ayrılan bayt
	// Sayaçlar: sürücüden ayrılmış, havuzda boşta, tensörlere verilmiş (canlı).
	reserved, pooled, live uint64
	peakLive, peakReserved uint64
	gcRuns, driverAllocs   int64
	// Konuma göre canlı bellek (yalnızca SetAllocSite açıkken; profile_api.go): her canlı
	// bloğun ayrıldığı konum, konum başına canlı bayt ve izlemenin tepesindeki dağılım.
	sites              map[uint64]siteKey
	liveBy, peakBy     map[siteKey]siteUse
	siteLive, sitePeak uint64
	// large 1 MB'den büyük blokların parçaları (memory_blocks.go).
	large largePool
}

// classSize küçük (≤ 1 MB) ayırmanın yuvarlandığı boy: 256 bayta kadar 256, üstü ikinin
// kuvvetleri. Daha büyükleri largeSize'a (512 bayt) yuvarlanır.
func classSize(byteN uint64) uint64 {
	if byteN <= 256 {
		return 256
	}
	return 1 << bits.Len64(byteN-1)
}

// release2 bloğu sınıfının listesine koyar (çöp toplayıcının temizlik iş parçacığından da
// çağrılır; yalnızca havuzun kilidini alır).
func (h *pool) release2(addr, class uint64) {
	h.mu.Lock()
	if class > smallLimit {
		h.large.give(addr)
	} else {
		h.empty[class] = append(h.empty[class], addr)
	}
	h.pooled += class
	h.live -= class
	h.untrackLocked(addr, class)
	h.mu.Unlock()
}

// untrack bloğun ayrıldığı konumun kaydını siler (blok kaydın havuzuna girerken: yeniden
// verilince track onu yeni konumuyla sayar).
func (h *pool) untrack(addr, class uint64) {
	h.mu.Lock()
	h.untrackLocked(addr, class)
	h.mu.Unlock()
}

func (h *pool) untrackLocked(addr, class uint64) {
	if s, ok := h.sites[addr]; ok {
		delete(h.sites, addr)
		u := h.liveBy[s]
		h.liveBy[s] = siteUse{u.bytes - class, u.blocks - 1}
		h.siteLive -= class
	}
}

// track tensöre verilen bloğu ayıran konumu kaydeder (izleme kapalıyken yalnızca bir atomik
// okuma). İzlenen canlı bellek yeni bir tepeye çıkınca konum başına dağılımın kopyası alınır;
// kopya konum sayısı kadardır (yüzlerce) ve tepe ilk adımdan sonra seyrek yükselir.
func (h *pool) track(addr, class uint64, dt tensor.DType) {
	f := allocSite.Load()
	if f == nil {
		return
	}
	key := siteKey{(*f)(), dt}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sites == nil {
		h.sites, h.liveBy, h.peakBy = map[uint64]siteKey{}, map[siteKey]siteUse{}, map[siteKey]siteUse{}
	}
	h.sites[addr] = key
	u := h.liveBy[key]
	h.liveBy[key] = siteUse{u.bytes + class, u.blocks + 1}
	h.siteLive += class
	if h.siteLive > h.sitePeak {
		h.sitePeak = h.siteLive
		clear(h.peakBy)
		for s, u := range h.liveBy {
			if u.blocks > 0 {
				h.peakBy[s] = u
			}
		}
	}
}

// siteKey canlı belleğin konum ve tür anahtarıdır; siteUse o anahtardaki canlı bayt ve blok
// sayısıdır.
type siteKey struct {
	site uint64
	dt   tensor.DType
}

type siteUse struct{ bytes, blocks uint64 }

// resetSites konum izlemesini sıfırlar (izleme yeniden açılınca önceki dağılım silinir).
func (h *pool) resetSites() {
	h.mu.Lock()
	h.sites, h.liveBy, h.peakBy = nil, nil, nil
	h.siteLive, h.sitePeak = 0, 0
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

// addLive canlı sayaçlarını artırır (kilit tutulurken).
func (h *pool) addLive(class uint64) {
	h.live += class
	h.peakLive = max(h.peakLive, h.live)
}

// newBlock sürücüden yeni ayrılan bloğu sayar.
func (h *pool) newBlock(class uint64) {
	h.mu.Lock()
	h.reserved += class
	h.peakReserved = max(h.peakReserved, h.reserved)
	h.driverAllocs++
	h.addLive(class)
	h.mu.Unlock()
}

// measurement sayaçların anlık görüntüsü.
func (h *pool) measurement() MemorySample {
	h.mu.Lock()
	defer h.mu.Unlock()
	return MemorySample{Reserved: h.reserved, Pooled: h.pooled, Live: h.live, PeakLive: h.peakLive,
		PeakReserved: h.peakReserved, GCRuns: h.gcRuns, DriverAllocs: h.driverAllocs}
}

type cleanupInfo struct {
	h                 *pool
	priv              *privatePool
	addr, class, want uint64
}

// back bloğu sahibine döndürür: kaydın havuzu ya da genel havuz.
func (t cleanupInfo) back() {
	if t.priv != nil {
		t.priv.give(t.want, t.addr, t.class)
		return
	}
	t.h.release2(t.addr, t.class)
}

// allocate n elemanlı dt türünde depolama verir; sifirla doğruysa içerik sıfırdır.
func (g *backend) allocate(n int, dt tensor.DType, reset bool) *storage {
	if dt != tensor.Float32 && dt != tensor.BFloat16 && dt != tensor.Float16 {
		none(messages.H4050, dt.String())
	}
	if n < 0 {
		panic(internalf("negative element count %d", n))
	}
	byteN := uint64(n) * uint64(dtypeSize(dt))
	if byteN > g.sr.totalMemory() {
		panic(tensor.SizeError(n, dt, int64(g.sr.totalMemory()), tensor.PlaceGPU))
	}
	d := &storage{g: g, n: n, dtype: dt}
	if n == 0 {
		return d
	}
	if g.gcThreshold > 0 && g.volume > g.gcThreshold {
		g.collectGarbage(0)
	}
	var addr uint64
	want := classSize(byteN)
	if byteN > smallLimit {
		want = largeSize(byteN)
	}
	taken := false
	if c := g.capturing; c != nil {
		d.priv, d.want = c, want
		addr, d.class, taken = c.take(want)
	}
	switch {
	case taken:
	case byteN > smallLimit:
		addr, d.class = g.allocLarge(want, byteN)
	default:
		d.class = want
		var ok bool
		if addr, ok = g.pool.al(d.class); !ok {
			addr = g.driverAlloc(d.class, byteN)
		}
	}
	g.volume += d.class
	d.addr = addr
	g.pool.track(addr, d.class, dt)
	d.clean = runtime.AddCleanup(d, func(t cleanupInfo) { t.back() }, cleanupInfo{&g.pool, d.priv, addr, d.class, d.want})
	if reset {
		g.reset(d)
	}
	return d
}

// reset depolamanın bütün baytlarını sıfırlar.
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

// fill32 float32 depolamanın bütün elemanlarını verilen bitlerle doldurur.
func (g *backend) fill32(d *storage, bitsList uint32) {
	if d.n == 0 {
		return
	}
	d.host = nil
	ev := g.measureStart()
	g.try_("cuMemsetD32Async", g.sr.fill32(d.addr, bitsList, uint64(d.n)))
	g.measureEnd("surucu_doldur", ev)
}

// dCopy cihaz içi kopya: hedefin hOfs'undan başlayarak kaynağın kOfs'undaki n eleman
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

// freshLimit son çöp toplamadan bu yana sürücüden ayrılabilecek bellek eşiğidir; aşılırsa ayırmadan
// önce çöp toplanır.
func (g *backend) freshLimit() uint64 { return max(g.sr.totalMemory()/32, 256<<20) }

// checkAllocResult sürücü ayırma sonucunu denetler: bellek yetmediyse H4014 ile panikler, başka
// bir hata varsa onu bildirir. requested kullanıcının istediği bayt sayısıdır (ileti için).
func (g *backend) checkAllocResult(s result, requested uint64) {
	if s == errNoMemory {
		empty, total, _ := g.sr.freeMemory()
		panic(messages.New(messages.H4014, bytesText(requested), bytesText(empty), bytesText(total), g.sr.errName(s)))
	}
	g.try_("cuMemAlloc", s)
}

// freeDriverBlock adresi sürücüye geri verir ve havuz sayaçlarından (reserved, pooled) size
// kadar düşer.
func (g *backend) freeDriverBlock(addr, size uint64) {
	g.try_("cuMemFree", g.sr.release(addr))
	g.pool.mu.Lock()
	g.pool.reserved -= size
	g.pool.pooled -= size
	g.pool.mu.Unlock()
}

// driverAlloc sürücüden yeni blok ayırır; gerekirse çöp toplamayı başlatır, bellek
// yetmezse havuzu boşaltıp yeniden dener.
func (g *backend) driverAlloc(class, requested uint64) uint64 {
	if g.pool.fresh2+class > g.freshLimit() {
		if addr, ok := g.collectGarbage(class); ok {
			return addr
		}
	}
	addr, s := g.sr.allocate(class)
	fromPool := false
	if s == errNoMemory {
		addr, s, fromPool = g.reclaimMemory(class)
	}
	g.checkAllocResult(s, requested)
	if fromPool {
		return addr // havuzdan alındı: al() sayaçları zaten işledi
	}
	g.pool.fresh2 += class
	g.pool.newBlock(class)
	return addr
}

// collectGarbage Go çöp toplayıcısını çalıştırır: ulaşılamayan depolamaların temizlikleri
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

// reclaimMemory çöp toplayıcıyı çalıştırır, temizliklerin havuza dönmesini kısa süre
// bekler, havuzdaki bütün blokları sürücüye geri verir ve ayırmayı yeniden dener. Blok
// havuzdan geldiyse (fromPool) sayaçlar al() ile işlenmiştir; yeni sürücü ayırması değildir.
func (g *backend) reclaimMemory(class uint64) (addr uint64, s result, fromPool bool) {
	runtime.GC()
	for i := 0; i < 20; i++ {
		if addr, ok := g.pool.al(class); ok {
			return addr, succeeded, true
		}
		time.Sleep(5 * time.Millisecond)
		runtime.Gosched()
	}
	g.equalize()
	g.releaseSmall()
	g.releaseEmptySegments()
	addr, s = g.sr.allocate(class)
	return addr, s, false
}

// releaseSmall havuzdaki boş küçük blokları sürücüye geri verir (cihaz eşzamanlanmış olmalı).
func (g *backend) releaseSmall() {
	g.pool.mu.Lock()
	empty := g.pool.empty
	g.pool.empty = map[uint64][]uint64{}
	g.pool.mu.Unlock()
	for class, l := range empty {
		for _, addr := range l {
			g.freeDriverBlock(addr, class)
		}
	}
}

// release depolamayı havuza iade eder (Release). Açıkça iade edilen bellek çöp değildir: son
// toplamadan bu yana verilenlerden düşülür, böylece toplama yalnızca çöp toplayıcıya kalan
// bellek birikince başlar (belirlenimci bırakma, tensor/lifetime.go).
func (g *backend) release(d *storage) {
	if d.n == 0 || d.release2 {
		return
	}
	d.release2 = true
	d.clean.Stop()
	d.host = nil
	g.volume -= min(g.volume, d.class)
	if d.pins > 0 {
		d.parked = true // canlı bir kayıt onu okur ya da yazar: kayıt bırakılınca iade edilir
		return
	}
	cleanupInfo{&g.pool, d.priv, d.addr, d.class, d.want}.back()
}

// ----------------------------------------------------------------------------
// Konak hazırlık halkası (sayfası kilitli bellek; eşzamansız yükleme)
// ----------------------------------------------------------------------------

// setup eşzamansız yüklemeler için sayfası kilitli konak belleği. Yüklenecek veri
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

// loadHost konak verisini depolamanın başına kopyalar (küçükse halka üzerinden
// eşzamansız).
func (g *backend) loadHost(d *storage, data unsafe.Pointer, byteN int) {
	if byteN == 0 {
		return
	}
	g.noCapture(messages.H4066)
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

// loadInt 32 bitlik tam sayıları yeni bir cihaz tamponuna yükler (indeksler, hedefler).
func (g *backend) loadInt(v []int32) *storage {
	d := g.allocate(len(v), tensor.Float32, false)
	if len(v) > 0 {
		g.loadHost(d, unsafe.Pointer(&v[0]), 4*len(v))
	}
	return d
}

// constIntLimit cihazda tutulan değişmeyen tablo sayısının sınırı; aşılınca tablolar bırakılır
// ve baştan doldurulur.
const constIntLimit = 1024

// constInts değişmeyen bir tam sayı tablosunu (şekillerden hesaplanan ofsetler) cihazda bir kez
// yükler ve sonraki çağrılarda aynı depolamayı verir: kayıt sırasında konaktan yükleme yapılamaz
// (graph.go), ısınma çağrısı tabloyu yüklemiş olur. Sınır dolunca (kayıt dışında) tablolar
// bırakılır; bir kaydın okuduğu tablonun belleği kayıt bırakılana kadar kalır (pins).
func (g *backend) constInts(gt *temp, v []int) *storage {
	var sb strings.Builder
	for _, x := range v {
		sb.WriteString(strconv.Itoa(x))
		sb.WriteByte(',')
	}
	key := sb.String()
	if d, ok := g.ints[key]; ok {
		return d
	}
	if len(g.ints) >= constIntLimit {
		if g.capturing != nil {
			return gt.loadInt(v) // kayıtta yüklenemez (H4066)
		}
		for _, d := range g.ints {
			g.release(d)
		}
		clear(g.ints)
	}
	if g.ints == nil {
		g.ints = map[string]*storage{}
	}
	d := g.loadInt(ints(v))
	g.ints[key] = d
	return d
}

// download cihazdaki bayt baytlık bölgeyi konağa kopyalar (eşzamanlar).
func (g *backend) download(target unsafe.Pointer, source uint64, byteN int) {
	if byteN == 0 {
		return
	}
	g.noCapture(messages.H4065)
	g.equalBegin()
	g.try_("cuMemcpyDtoH", g.sr.download(target, source, uint64(byteN)))
	g.setup.offset = 0
	g.equalFinish()
}

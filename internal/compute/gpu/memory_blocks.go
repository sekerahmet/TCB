package gpu

import (
	"runtime"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Büyük bloklar: parçalara bölünen önbellekli ayırıcı
// ============================================================================
//
// 1 MB'den büyük istekler boyut sınıflı listelerden değil, sürücüden alınan parçalardan
// (segment) verilir. İstek 512 bayta yuvarlanır ve boş bloklar arasından en küçük yetenine
// (best-fit) yerleştirilir; artan kısım 1 MB'den büyükse bölünüp boş blok olarak kalır.
// Serbest kalan blok aynı parçadaki boş komşularıyla birleştirilir. Böylece bir boyutta
// boşa çıkan bellek başka boyuttaki isteğe verilebilir; sınıflı listede bir sınıfın boş
// blokları yalnızca o sınıfı bekliyordu ve havuz, canlı belleğin ~1,4 katına şişiyordu
// (hedef model: canlı 12,6 GB, ayrılmış 17,3 GB). PyTorch'un CUDA caching allocator'ı aynı
// yöntemi kullanır (aynı yuvarlama, 1 MB bölme eşiği, 1–10 MB isteklere 20 MB'lık parça).
//
// Parça boyu: 10 MB'ye kadarki istekler için 20 MB (birkaç orta boy blok bir parçaya sığar),
// üstü 2 MB'nin katı. Bellek yetmezse tamamen boş parçalar sürücüye geri verilir.
//
// Benzetimde parçanın içindeki bloklar aynı bölgededir: bölgeler arası boşluk yalnızca
// parçanın dışına taşmayı yakalar, komşu bloğa taşmayı yakalamaz.

const (
	smallLimit   = 1 << 20  // bundan büyük istekler parçalardan verilir
	largeRound   = 512      // büyük isteklerin yuvarlandığı bayt
	splitLimit   = 1 << 20  // artan kısım bundan büyükse blok bölünür
	midLimit     = 10 << 20 // bundan küçük istekler ortak parçadan
	midSegment   = 20 << 20 // ortak parçanın boyu
	segmentRound = 2 << 20  // büyük parçaların yuvarlandığı bayt
)

// segment sürücüden alınmış bir bellek parçası.
type segment struct {
	addr, size uint64
}

// block bir parçanın ardışık bir dilimi: verilmiş ya da boş. prev ve next aynı parçadaki
// komşu bloklardır (adres sırasıyla).
type block struct {
	seg        *segment
	addr, size uint64
	free       bool
	prev, next *block
}

// largePool büyük blokların durumu (pool.mu ile korunur).
type largePool struct {
	free   []*block          // boş bloklar, (boy, adres) sırasıyla
	byAddr map[uint64]*block // her blok (verilmiş ya da boş) başlangıç adresine göre
}

// largeSize büyük isteğin yuvarlandığı boy.
func largeSize(byteN uint64) uint64 {
	return (byteN + largeRound - 1) &^ (largeRound - 1)
}

// segmentSize size boyundaki bir istek için sürücüden alınacak parçanın boyu.
func segmentSize(size uint64) uint64 {
	if size < midLimit {
		return midSegment
	}
	return (size + segmentRound - 1) &^ (segmentRound - 1)
}

// freeIndex b'nin boş listedeki yeri (yoksa eklenecek yer).
func (p *largePool) freeIndex(size, addr uint64) int {
	return sort.Search(len(p.free), func(i int) bool {
		f := p.free[i]
		return f.size > size || f.size == size && f.addr >= addr
	})
}

func (p *largePool) insertFree(b *block) {
	b.free = true
	i := p.freeIndex(b.size, b.addr)
	p.free = append(p.free, nil)
	copy(p.free[i+1:], p.free[i:])
	p.free[i] = b
}

func (p *largePool) removeFree(b *block) {
	i := p.freeIndex(b.size, b.addr)
	if i == len(p.free) || p.free[i] != b {
		panic(internalf("free block %#x (%d bytes) not in the free list", b.addr, b.size))
	}
	p.free = append(p.free[:i], p.free[i+1:]...)
	b.free = false
}

// split b'yi size boyunda başlangıç ve kalan iki bloğa böler (kalan boş listeye girer) ya
// da kalan küçükse bütün bloğu verir; verilen bloğun boyunu döndürür.
func (p *largePool) split(b *block, size uint64) uint64 {
	if b.size-size <= splitLimit {
		return b.size
	}
	rest := &block{seg: b.seg, addr: b.addr + size, size: b.size - size, prev: b, next: b.next}
	if b.next != nil {
		b.next.prev = rest
	}
	b.next = rest
	b.size = size
	p.byAddr[rest.addr] = rest
	p.insertFree(rest)
	return size
}

// take size boyuna yeten en küçük boş bloğu verir (gerekirse böler): adres ve verilen boy.
func (p *largePool) take(size uint64) (addr, got uint64, ok bool) {
	i := p.freeIndex(size, 0)
	if i == len(p.free) {
		return 0, 0, false
	}
	b := p.free[i]
	p.removeFree(b)
	return b.addr, p.split(b, size), true
}

// addSegment sürücüden yeni alınmış parçayı ekler ve başından size boyunda blok verir.
func (p *largePool) addSegment(addr, segSize, size uint64) uint64 {
	if p.byAddr == nil {
		p.byAddr = map[uint64]*block{}
	}
	b := &block{seg: &segment{addr: addr, size: segSize}, addr: addr, size: segSize}
	p.byAddr[addr] = b
	return p.split(b, size)
}

// give addr'deki verilmiş bloğu boşa çıkarır ve boş komşularıyla birleştirir; bloğun boyunu
// verir.
func (p *largePool) give(addr uint64) uint64 {
	b := p.byAddr[addr]
	if b == nil || b.free {
		panic(internalf("block %#x returned twice or never given", addr))
	}
	size := b.size
	if n := b.next; n != nil && n.free {
		p.removeFree(n)
		delete(p.byAddr, n.addr)
		b.size += n.size
		b.next = n.next
		if n.next != nil {
			n.next.prev = b
		}
	}
	if q := b.prev; q != nil && q.free {
		p.removeFree(q)
		delete(p.byAddr, b.addr)
		q.size += b.size
		q.next = b.next
		if b.next != nil {
			b.next.prev = q
		}
		b = q
	}
	p.insertFree(b)
	return size
}

// emptySegments tamamen boş parçaları boş listeden çıkarıp verir (sürücüye geri verilmek
// üzere).
func (p *largePool) emptySegments() []*segment {
	var out []*segment
	kept := p.free[:0]
	for _, b := range p.free {
		if b.prev == nil && b.next == nil && b.size == b.seg.size {
			delete(p.byAddr, b.addr)
			out = append(out, b.seg)
			continue
		}
		kept = append(kept, b)
	}
	p.free = kept
	return out
}

// takeLarge havuzdaki boş bloklardan size boyunda blok verir (sayaçlarıyla).
func (h *pool) takeLarge(size uint64) (addr, got uint64, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if addr, got, ok = h.large.take(size); ok {
		h.pooled -= got
		h.addLive(got)
	}
	return addr, got, ok
}

// newSegment sürücüden yeni alınan parçayı ekler ve başından size boyunda blok verir.
func (h *pool) newSegment(addr, segSize, size uint64) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	got := h.large.addSegment(addr, segSize, size)
	h.reserved += segSize
	h.peakReserved = max(h.peakReserved, h.reserved)
	h.pooled += segSize - got
	h.driverAllocs++
	h.addLive(got)
	return got
}

// allocLarge 1 MB'den büyük bir blok verir: önce boş bloklardan, yoksa (son toplamadan bu
// yana sürücüden çok ayrıldıysa önce çöp toplanıp yeniden denenerek) sürücüden yeni parça;
// bellek yetmezse tamamen boş parçalar ve küçük bloklar sürücüye geri verilip yeniden
// denenir. Adresi ve verilen bloğun boyunu döndürür.
func (g *backend) allocLarge(size, requested uint64) (uint64, uint64) {
	if addr, got, ok := g.pool.takeLarge(size); ok {
		return addr, got
	}
	seg := segmentSize(size)
	if g.pool.fresh2+seg > max(g.sr.totalMemory()/32, 256<<20) {
		g.collectGarbage(0)
		for i := 0; i < 64; i++ {
			if addr, got, ok := g.pool.takeLarge(size); ok {
				return addr, got
			}
			runtime.Gosched()
		}
	}
	addr, s := g.sr.allocate(seg)
	if s == errNoMemory {
		if addr, got, ok := g.reclaimLarge(size); ok {
			return addr, got
		}
		addr, s = g.sr.allocate(seg)
		if s == errNoMemory && seg > size {
			// Yuvarlanmış parça sığmıyorsa tam boyunu dene.
			seg = (size + segmentRound - 1) &^ (segmentRound - 1)
			addr, s = g.sr.allocate(seg)
		}
	}
	if s == errNoMemory {
		empty, total, _ := g.sr.freeMemory()
		panic(messages.New(messages.H4014, bytesText(requested), bytesText(empty), bytesText(total), g.sr.errName(s)))
	}
	g.try_("cuMemAlloc", s)
	g.pool.fresh2 += seg
	return addr, g.pool.newSegment(addr, seg, size)
}

// reclaimLarge bellek yetmeyince çöp toplar, boşa çıkan bloklardan size'ı karşılamayı dener;
// olmazsa tamamen boş parçaları ve küçük blokları sürücüye geri verir (cihaz eşzamanlanır).
func (g *backend) reclaimLarge(size uint64) (addr, got uint64, ok bool) {
	runtime.GC()
	for i := 0; i < 20; i++ {
		if addr, got, ok = g.pool.takeLarge(size); ok {
			return addr, got, true
		}
		runtime.Gosched()
	}
	g.equalize()
	g.releaseSmall()
	g.releaseEmptySegments()
	return 0, 0, false
}

// releaseEmptySegments tamamen boş parçaları sürücüye geri verir (cihaz eşzamanlanmış olmalı).
func (g *backend) releaseEmptySegments() {
	g.pool.mu.Lock()
	segs := g.pool.large.emptySegments()
	g.pool.mu.Unlock()
	for _, sg := range segs {
		g.try_("cuMemFree", g.sr.release(sg.addr))
		g.pool.mu.Lock()
		g.pool.reserved -= sg.size
		g.pool.pooled -= sg.size
		g.pool.mu.Unlock()
	}
}

package gpu

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// largePool bölme, en uygun boş blok ve komşu birleştirme kurallarını izler.
func TestLargePoolSplitBestFitCoalesce(t *testing.T) {
	var p largePool
	const mb = 1 << 20
	base := uint64(1 << 40)
	if got := p.addSegment(base, 20*mb, 3*mb); got != 3*mb {
		t.Fatalf("ilk blok %d, 3 MB beklenirdi", got)
	}
	a2, g2, ok := p.take(4 * mb)
	if !ok || a2 != base+3*mb || g2 != 4*mb {
		t.Fatalf("ikinci blok %#x %d %v; parçanın kalanından 4 MB beklenirdi", a2, g2, ok)
	}
	// Kalan 13 MB boş; artan kısım 1 MB'yi aşmayan istek bloğun tamamını alır.
	if len(p.free) != 1 || p.free[0].size != 13*mb {
		t.Fatalf("boş liste %v", p.free)
	}
	a3, g3, _ := p.take(12*mb + 512)
	if a3 != base+7*mb || g3 != 13*mb {
		t.Fatalf("12,0005 MB istek 13 MB'lik bloğun tamamını almalıydı: %#x %d", a3, g3)
	}
	// Ortadaki blok boşalır: komşuları dolu, birleşmez. Sonra ilki ve sonuncusu boşalır:
	// hepsi tek boş bloğa birleşir ve parça tamamen boştur.
	p.give(a2)
	if len(p.free) != 1 || p.free[0].size != 4*mb {
		t.Fatalf("ortadaki blok tek başına boş olmalı: %v", p.free)
	}
	p.give(base)
	p.give(a3)
	if len(p.free) != 1 || p.free[0].size != 20*mb || p.free[0].addr != base {
		t.Fatalf("boş bloklar birleşmedi: %+v", p.free)
	}
	if segs := p.emptySegments(); len(segs) != 1 || segs[0].size != 20*mb || len(p.byAddr) != 0 {
		t.Fatalf("boş parça geri verilmeli: %v", segs)
	}
	// En uygun: 8 MB ve 5 MB boş bloktan 5 MB istek küçüğünü alır.
	var q largePool
	q.addSegment(base, 8*mb, 8*mb)
	q.addSegment(base+64*mb, 5*mb, 5*mb)
	q.give(base)
	q.give(base + 64*mb)
	if a, _, _ := q.take(5 * mb); a != base+64*mb {
		t.Fatalf("en uygun blok seçilmedi: %#x", a)
	}
}

// Büyük bir tensör bırakılınca belleği başka boydaki isteklere verilir (sürücüden yeni
// parça alınmaz); bellek yetmeyince tamamen boş parçalar geri verilip istek karşılanır.
func TestLargeBlocksReuseAndReclaim(t *testing.T) {
	g := newSimBackend(96 << 20)
	g.enter()
	defer g.exit()
	a := g.allocate(30<<20/4, tensor.Float32, false) // 30 MB
	g.release(a)
	before := g.pool.measurement()
	b := g.allocate(18<<20/4, tensor.Float32, false)
	c := g.allocate(9<<20/4, tensor.Float32, false)
	if o := g.pool.measurement(); o.DriverAllocs != before.DriverAllocs || o.Reserved != before.Reserved {
		t.Fatalf("bırakılan 30 MB yeniden kullanılmadı: önce %+v, sonra %+v", before, o)
	}
	g.release(b)
	g.release(c)
	// 96 MB'lik kartta 30 MB'lik parça boşken 80 MB istenir: boş parça geri verilir.
	d := g.allocate(80<<20/4, tensor.Float32, false)
	if o := g.pool.measurement(); o.Reserved != 80<<20 || o.Live != 80<<20 {
		t.Fatalf("boş parça geri verilmedi: %+v", o)
	}
	g.release(d)
}

// Küçük blok ayırmada sürücü errNoMemory döndürdüğünde reclaimMemory çöp toplayıcıyla
// havuza dönen bloğu al() ile alırsa (live += class), driverAlloc üstüne bir de
// newBlock(class) çağırarak live, reserved ve driverAllocs sayaçlarını iki kez artırır.
func TestSmallReclaimFromPoolKeepsMemoryStats(t *testing.T) {
	const total = 1 << 20 // 1 MB
	const half = 1 << 19  // 512 KB (<= smallLimit)
	g := newSimBackend(total)
	g.enter()
	defer g.exit()
	a := g.allocate(half/4, tensor.Float32, false)
	func() {
		_ = g.allocate(half/4, tensor.Float32, false) // çöp toplayıcıyla havuza dönecek
	}()
	c := g.allocate(half/4, tensor.Float32, false)
	g.release(a)
	g.release(c)
	if m := g.pool.measurement(); m.Live != 0 || m.Reserved > total {
		t.Fatalf("reclaimMemory sonrası havuz sayaçları bozuldu: Live=%d (0 beklenirdi), Reserved=%d (en çok %d beklenirdi)",
			m.Live, m.Reserved, total)
	}
}


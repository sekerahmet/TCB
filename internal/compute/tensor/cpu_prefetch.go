package tensor

import (
	"sync/atomic"
	"unsafe"
)

// Ürünler arası önden getirme. Üretimde tek satırlı çarpımlar hep aynı sırayla gelir, araları norm
// ve dikkatle geçer; bellek yolu o aralıkta boştur, dönen işçiler boşa bekler. Burada her ağırlığın
// ardından hangisinin geldiği öğrenilir, çarpım bitince dönen işçiler sıradakini önbelleğe çağırır.
//
// Yalnız adres tutulur: tablo ağırlığı canlı tutmaz. Ağırlık serbest kaldıysa önden getirme
// yönergesi hata vermez, bellek okunmaz; sonuca hiçbir yerde girmez, bitler değişmez.

const (
	prefetchSlots = 1 << 10 // tablo; çakışan iki ağırlıktan sonuncusu kalır
	prefetchCap   = 4 << 20 // bir ipucunun en çok baytı: dev kafa matrisinin hepsi sığmaz
	prefetchChunk = 8 << 10 // işçi iki iş denetimi arasında bu kadar çağırır: yeni işi geç görmesin
)

// prefetchHint sıradaki ağırlığın yeridir; kurulduktan sonra değişmez.
type prefetchHint struct {
	addr uintptr
	size int
}

var ahead struct {
	last  atomic.Uintptr
	keys  [prefetchSlots]atomic.Uintptr
	hints [prefetchSlots]atomic.Pointer[prefetchHint]
	cur   atomic.Pointer[prefetchHint]
	seq   atomic.Uint64
}

func prefetchSlot(p uintptr) int { return int(uint64(p>>6) * 0x9E3779B97F4A7C15 >> 54) }

// prefetchBefore çarpımdan önce çağrılır: önceki ağırlığın ardılını yazar, süren ipucunu keser.
func prefetchBefore(w []byte) uintptr {
	self := uintptr(unsafe.Pointer(&w[0]))
	ahead.cur.Store(nil)
	if prev := ahead.last.Swap(self); prev != 0 && prev != self {
		i := prefetchSlot(prev)
		if h := ahead.hints[i].Load(); ahead.keys[i].Load() != prev || h == nil || h.addr != self {
			ahead.keys[i].Store(prev)
			ahead.hints[i].Store(&prefetchHint{addr: self, size: min(len(w), prefetchCap)})
		}
	}
	return self
}

// prefetchAfter çarpımdan sonra çağrılır: ardılı biliniyorsa dönen işçilere duyurur.
func prefetchAfter(self uintptr) {
	i := prefetchSlot(self)
	if ahead.keys[i].Load() != self {
		return
	}
	if h := ahead.hints[i].Load(); h != nil {
		ahead.seq.Add(1)
		ahead.cur.Store(h)
	}
}

// prefetchCursor bir işçinin süren ipucundaki yeridir; yalnız o işçi kullanır.
type prefetchCursor struct {
	seq      uint64
	pos, end int
}

// step payının sıradaki parçasını çağırır; yapacak iş yoksa false.
func (c *prefetchCursor) step(no int) bool {
	h := ahead.cur.Load()
	if h == nil {
		return false
	}
	if s := ahead.seq.Load(); s != c.seq {
		c.seq = s
		c.pos, c.end = h.size*no/returningWorker, h.size*(no+1)/returningWorker
	}
	if c.pos >= c.end {
		return false
	}
	n := min(prefetchChunk, c.end-c.pos)
	prefetchBytes(h.addr+uintptr(c.pos), n)
	c.pos += n
	return true
}

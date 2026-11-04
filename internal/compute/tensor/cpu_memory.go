package tensor

import (
	"math"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ----------------------------------------------------------------------------
// Bellek
// ----------------------------------------------------------------------------

// poolMin bundan küçük depolamalar havuza alınmaz (küçük ayırmalar ucuzdur).
const poolMin = 1 << 12

type poolKey struct {
	dtyp DType
	n    int
}

// cpuPool iade edilen depolamaları tür ve eleman sayısına göre tutar. Eğitimde
// aynı boyutlar her adımda yinelendiğinden tam boyut eşlemesi yeterlidir;
// sync.Pool kullanılmayan tamponları çöp toplayıcıya bırakır.
//
// 19.10.2026 ölçüldü (karakter modeli): sync.Pool yerine toplamı sınırlı yığınlar
// (matris çarpımı tamponlarındaki gibi) ayrılan baytı %8-13 azalttı ama adım süresini
// ancak ~%4 (gürültü sınırında) kısalttı ve çöp toplama sayısını değiştirmedi; profilde
// yeni ayırmaların neredeyse hepsi hâlâ allocate'te. Depolamaların çoğu (ileri geçişin
// ara sonuçları) havuza hiç iade edilmiyor; kazanç onları geri yayılımdan sonra iade
// etmekten gelir, bu da sahiplik bilgisi ister.
var cpuPool struct {
	mu sync.RWMutex
	m  map[poolKey]*sync.Pool
}

func poolFor(k poolKey, create bool) *sync.Pool {
	cpuPool.mu.RLock()
	h := cpuPool.m[k]
	cpuPool.mu.RUnlock()
	if h != nil || !create {
		return h
	}
	cpuPool.mu.Lock()
	defer cpuPool.mu.Unlock()
	if cpuPool.m == nil {
		cpuPool.m = map[poolKey]*sync.Pool{}
	}
	if h = cpuPool.m[k]; h == nil {
		h = &sync.Pool{}
		cpuPool.m[k] = h
	}
	return h
}

func cpuCheckDType(dt DType) {
	if !dt.IsFloat() && !dt.IsTyped() {
		panic(messages.New(messages.H3025, dt))
	}
}

// Alloc sıfırlanmış depolama verir; uygun tür ve boyutta iade edilmiş bir
// depolama varsa onu (paralel olarak sıfırlayıp) yeniden kullanır.
func (b cpuBackend) Alloc(n int, dt DType) Storage { return b.allocate(n, dt, true) }

// allocUninit (bosAyirici) içeriği belirsiz depolama verir: havuzdan gelen
// depolama sıfırlanmaz. Yalnızca her elemanı yazılacak çıktılar için.
func (b cpuBackend) allocUninit(n int, dt DType) Storage { return b.allocate(n, dt, false) }

func (cpuBackend) allocate(n int, dt DType, reset bool) Storage {
	cpuCheckDType(dt)
	checkElem(n, dt)
	if dt.IsTyped() {
		return newTypedStorage(n, dt)
	}
	if n >= poolMin {
		if h := poolFor(poolKey{dt, n}, false); h != nil {
			if s, ok := h.Get().(*cpuStorage); ok {
				s.release = false
				if !reset {
					return s
				}
				if dt == Float32 {
					d := s.d
					parallelFor(len(d), 1, func(lo, hi int) { clear(d[lo:hi]) })
				} else {
					d := s.h
					parallelFor(len(d), 1, func(lo, hi int) { clear(d[lo:hi]) })
				}
				return s
			}
		}
	}
	// Yeni ayırmayı Go çalışma zamanı zaten sıfırlar.
	if dt == Float32 {
		return &cpuStorage{d: make([]float32, n)}
	}
	return &cpuStorage{dtyp: dt, h: make([]uint16, n)}
}

// Release depolamayı havuza iade eder (Releaser). Çağıran, depolamaya başka
// hiçbir görünümün bakmadığını güvence eder.
func (cpuBackend) Release(s Storage) {
	c, ok := s.(*cpuStorage)
	if !ok || c.Len() < poolMin || c.dtyp.IsTyped() {
		return
	}
	if c.release {
		panic(messages.Internal("storage returned to the pool twice"))
	}
	c.release = true
	poolFor(poolKey{c.dtyp, c.Len()}, true).Put(c)
}

func (cpuBackend) FromHost(data []float32, dt DType) Storage {
	cpuCheckDType(dt)
	checkComputeFloat(dt)
	checkElem(len(data), dt)
	if dt == Float32 {
		d := make([]float32, len(data))
		copy(d, data)
		return &cpuStorage{d: d}
	}
	h := make([]uint16, len(data))
	narrow(h, data, dt)
	return &cpuStorage{dtyp: dt, h: h}
}

func (cpuBackend) ToHost(src View) []float32 {
	checkComputeFloat(src.DType())
	out := make([]float32, checkElem(src.Numel(), Float32))
	if src.DType() == Float32 {
		copyTo(out, src)
	} else {
		widenInto(out, src)
	}
	return out
}

func (b cpuBackend) Copy(dst Storage, dstOff int, src View) {
	n := src.Numel()
	c := cpuStore(dst)
	if c.dtyp.IsTyped() || src.DType().IsTyped() {
		copyTyped(View{S: dst, Offset: dstOff, Shape: src.Shape, Strides: ContiguousStrides(src.Shape)}, src)
		return
	}
	defer NoteImplicit(false, ImplicitKindOf(c.dtyp, src, true), n).Finish()
	if c.dtyp == Float32 {
		if src.DType() == Float32 {
			copyTo(c.d[dstOff:dstOff+n], src)
		} else {
			widenInto(c.d[dstOff:dstOff+n], src)
		}
		return
	}
	target := c.h[dstOff : dstOff+n]
	if s := cpuStore(src.S); s.dtyp == c.dtyp {
		// Aynı tür: bit düzeyinde kopya (yuvarlama yok).
		if src.IsContiguous() {
			k := s.h[src.Offset : src.Offset+n]
			parallelFor(n, 1, func(lo, hi int) { copy(target[lo:hi], k[lo:hi]) })
			return
		}
		copyRows(c.h, s.h, View{S: dst, Offset: dstOff, Shape: src.Shape, Strides: ContiguousStrides(src.Shape)}, src)
		return
	}
	// Farklı tür: float32 üzerinden (16 bit → float32 → 16 bit kayıpsızdır).
	narrow(target, b.ToHost(src), c.dtyp)
}

// copyRows aynı türdeki görünümü öğeleriyle kopyalar (d ve k görünümlerin dizileridir): 16 bitlik
// tür float32'den geçseydi her öğe açılıp yeniden daraltılırdı. Görünümler örtüşmemelidir.
func copyRows[E float32 | uint16](d, k []E, dst, src View) {
	iterRowsPar(dst.Shape, []int{dst.Offset, src.Offset}, [][]int{dst.Strides, src.Strides}, 0, func(off []int, m int, in []int) {
		h, q, hs, qs := off[0], off[1], in[0], in[1]
		if hs == 1 && qs == 1 {
			copy(d[h:h+m], k[q:q+m])
			return
		}
		for i := 0; i < m; i++ {
			d[h+i*hs] = k[q+i*qs]
		}
	})
}

func (cpuBackend) CopyStrided(dst, src View) {
	if dst.DType().IsTyped() || src.DType().IsTyped() {
		copyTyped(dst, src)
		return
	}
	defer NoteImplicit(false, ImplicitKindOf(dst.DType(), src, dst.IsContiguous()), dst.Numel()).Finish()
	if dst.DType() != Float32 && dst.DType() == src.DType() && dst.S != src.S {
		copyRows(cpuStore(dst.S).h, cpuStore(src.S).h, dst, src)
		return
	}
	var gt temp
	defer gt.finish()
	c := cpuStore(dst.S)
	k := gt.read(src)
	kd := cd(k.S)
	if c.dtyp == Float32 {
		copyRows(c.d, kd, dst, k)
		return
	}
	d, dtyp := c.h, c.dtyp
	iterRowsPar(dst.Shape, []int{dst.Offset, k.Offset}, [][]int{dst.Strides, k.Strides}, 0, func(off []int, m int, in []int) {
		h, q, hs, qs := off[0], off[1], in[0], in[1]
		for i := 0; i < m; i++ {
			d[h+i*hs] = narrowOne(kd[q+i*qs], dtyp)
		}
	})
}

// copyTo float32 görünümü geçici dizi kullanmadan doğrudan hedefe yazar.
func copyTo(out []float32, src View) {
	if src.IsContiguous() {
		copyPar(out, cd(src.S)[src.Offset:src.Offset+len(out)])
		return
	}
	gatherInto(out, src)
}

func (cpuBackend) Fill(dst Storage, v float32) {
	c := cpuStore(dst)
	checkComputeFloat(c.dtyp)
	if c.dtyp != Float32 {
		hv := narrowOne(v, c.dtyp)
		d := c.h
		parallelFor(len(d), 1, func(lo, hi int) {
			p := d[lo:hi]
			for i := range p {
				p[i] = hv
			}
		})
		return
	}
	d := c.d
	parallelFor(len(d), 1, func(lo, hi int) {
		p := d[lo:hi]
		if math.Float32bits(v) == 0 { // yalnızca +0: -0'ın işaret biti korunur
			clear(p)
			return
		}
		for i := range p {
			p[i] = v
		}
	})
}

package tensor

import (
	"fmt"
	"math"
	"sync"
)

// ============================================================================
// CPU arka ucu: depolama, veri türleri ve bellek işlemleri
// ============================================================================
//
// float32 depolama düz bir []float32, bfloat16 ve float16 depolama []uint16
// dilimidir. Hesap çekirdekleri (f32 önekli yöntemler) yalnızca float32 ile
// çalışır; Backend yöntemleri 16 bitlik girdileri float32'ye açar, çıktıyı
// float32 geçici tamponda hesaplayıp hedef türe bir kez (en yakına-çifte)
// yuvarlayarak yazar. Böylece depolama 16 bit, hesap ve iç birikim en az
// float32 olur. float32 yolunda ek maliyet yoktur.

// cpuStorage ana bellekte düz bir dilim.
type cpuStorage struct {
	VersionCounter
	tur  DType
	d    []float32 // Float32
	h    []uint16  // BFloat16, Float16
	iade bool      // havuza iade edildi (yeniden ayrılana kadar kullanılmamalı)
}

func (s *cpuStorage) Device() Device { return CPU }
func (s *cpuStorage) DType() DType   { return s.tur }
func (s *cpuStorage) Len() int {
	if s.tur == Float32 {
		return len(s.d)
	}
	return len(s.h)
}

func cpuDepo(s Storage) *cpuStorage {
	c, ok := s.(*cpuStorage)
	if !ok {
		panic(fmt.Sprintf("tensor: CPU arka ucuna %s cihazından depolama verildi", s.Device()))
	}
	return c
}

// cd depolamanın float32 CPU dilimini verir (nil için nil).
func cd(s Storage) []float32 {
	if s == nil {
		return nil
	}
	c := cpuDepo(s)
	if c.tur != Float32 {
		panic(fmt.Sprintf("tensor: CPU çekirdeği float32 depolama bekliyordu, %s verildi (iç hata)", c.tur))
	}
	return c.d
}

// ----------------------------------------------------------------------------
// Tür dönüşümleri (en yakına-çifte yuvarlama)
// ----------------------------------------------------------------------------

// f32ToBF16 float32'yi bfloat16'ya yuvarlar. NaN sessiz NaN olarak (işaret
// korunarak) kalır; taşma sonsuza gider.
func f32ToBF16(f float32) uint16 {
	b := math.Float32bits(f)
	if b&0x7FFFFFFF > 0x7F800000 {
		return uint16(b>>16) | 0x0040
	}
	b += 0x7FFF + (b>>16)&1
	return uint16(b >> 16)
}

// bf16ToF32 bfloat16'yı float32'ye açar (kayıpsız).
func bf16ToF32(h uint16) float32 { return math.Float32frombits(uint32(h) << 16) }

// f32ToF16 float32'yi IEEE yarım hassasiyete yuvarlar: 65520 ve üstü sonsuza,
// 2^-25 ve altı sıfıra gider; aradaki küçük değerler normal altı olur.
func f32ToF16(f float32) uint16 {
	b := math.Float32bits(f)
	isaret := uint16(b>>16) & 0x8000
	us := int((b >> 23) & 0xFF)
	ak := b & 0x7FFFFF
	if us == 0xFF {
		if ak != 0 {
			return isaret | 0x7E00 | uint16(ak>>13) // sessiz NaN
		}
		return isaret | 0x7C00
	}
	e := us - 127 + 15
	if e >= 0x1F {
		return isaret | 0x7C00
	}
	if e <= 0 {
		// Normal altı ya da sıfır: anlamlı kısım (gizli bit dahil) kaydırılır.
		if e < -10 {
			return isaret
		}
		ak |= 0x800000
		kay := uint(14 - e)
		m := ak >> kay
		kalan := ak & (1<<kay - 1)
		yarim := uint32(1) << (kay - 1)
		if kalan > yarim || (kalan == yarim && m&1 == 1) {
			m++ // en küçük normale taşabilir: kodlama yine doğrudur
		}
		return isaret | uint16(m)
	}
	h := uint32(e)<<10 | ak>>13
	kalan := ak & 0x1FFF
	if kalan > 0x1000 || (kalan == 0x1000 && h&1 == 1) {
		h++ // üsse (ve sonsuza) taşma kodlamada doğal olarak doğrudur
	}
	return isaret | uint16(h)
}

// f16ToF32 yarım hassasiyeti float32'ye açar (kayıpsız).
func f16ToF32(h uint16) float32 {
	isaret := uint32(h&0x8000) << 16
	us := uint32(h>>10) & 0x1F
	ak := uint32(h & 0x3FF)
	switch us {
	case 0x1F:
		return math.Float32frombits(isaret | 0x7F800000 | ak<<13)
	case 0:
		if ak == 0 {
			return math.Float32frombits(isaret)
		}
		e := uint32(127 - 14)
		for ak&0x400 == 0 {
			ak <<= 1
			e--
		}
		return math.Float32frombits(isaret | e<<23 | (ak&0x3FF)<<13)
	}
	return math.Float32frombits(isaret | (us+127-15)<<23 | ak<<13)
}

func daraltBir(v float32, dt DType) uint16 {
	if dt == BFloat16 {
		return f32ToBF16(v)
	}
	return f32ToF16(v)
}

// daralt dst = tür(src) (paralel).
func daralt(dst []uint16, src []float32, dt DType) {
	src = src[:len(dst)]
	parallelFor(len(dst), 2, func(lo, hi int) {
		d, s := dst[lo:hi], src[lo:hi]
		if dt == BFloat16 {
			for i, v := range s {
				d[i] = f32ToBF16(v)
			}
			return
		}
		for i, v := range s {
			d[i] = f32ToF16(v)
		}
	})
}

// genislet16 dst = float32(src) (paralel).
func genislet16(dst []float32, src []uint16, dt DType) {
	src = src[:len(dst)]
	parallelFor(len(dst), 2, func(lo, hi int) {
		d, s := dst[lo:hi], src[lo:hi]
		if dt == BFloat16 {
			for i, v := range s {
				d[i] = bf16ToF32(v)
			}
			return
		}
		for i, v := range s {
			d[i] = f16ToF32(v)
		}
	})
}

// genisletInto 16 bitlik görünümün elemanlarını mantıksal sırayla float32
// olarak out'a yazar.
func genisletInto(out []float32, v View) {
	c := cpuDepo(v.S)
	if v.IsContiguous() {
		genislet16(out, c.h[v.Offset:v.Offset+len(out)], c.tur)
		return
	}
	bf := c.tur == BFloat16
	iterRowsPar(v.Shape, []int{v.Offset, 0}, [][]int{v.Strides, contiguousStrides(v.Shape)}, 1, func(off []int, m int, in []int) {
		src, dst, s := off[0], off[1], in[0]
		o := out[dst : dst+m]
		for i := range o {
			if bf {
				o[i] = bf16ToF32(c.h[src+i*s])
			} else {
				o[i] = f16ToF32(c.h[src+i*s])
			}
		}
	})
}

// ----------------------------------------------------------------------------
// Bellek
// ----------------------------------------------------------------------------

// havuzEnAz bundan küçük depolamalar havuza alınmaz (küçük ayırmalar ucuzdur).
const havuzEnAz = 1 << 12

type havuzAnahtar struct {
	tur DType
	n   int
}

// cpuHavuz iade edilen depolamaları tür ve eleman sayısına göre tutar. Eğitimde
// aynı boyutlar her adımda yinelendiğinden tam boyut eşlemesi yeterlidir;
// sync.Pool kullanılmayan tamponları çöp toplayıcıya bırakır.
var cpuHavuz struct {
	mu sync.RWMutex
	m  map[havuzAnahtar]*sync.Pool
}

func havuzFor(k havuzAnahtar, olustur bool) *sync.Pool {
	cpuHavuz.mu.RLock()
	h := cpuHavuz.m[k]
	cpuHavuz.mu.RUnlock()
	if h != nil || !olustur {
		return h
	}
	cpuHavuz.mu.Lock()
	defer cpuHavuz.mu.Unlock()
	if cpuHavuz.m == nil {
		cpuHavuz.m = map[havuzAnahtar]*sync.Pool{}
	}
	if h = cpuHavuz.m[k]; h == nil {
		h = &sync.Pool{}
		cpuHavuz.m[k] = h
	}
	return h
}

func cpuTurDenetle(dt DType) {
	if dt != Float32 && dt != BFloat16 && dt != Float16 {
		panic(fmt.Sprintf("tensor: CPU arka ucu %s türünde depolama desteklemiyor", dt))
	}
}

// Alloc sıfırlanmış depolama verir; uygun tür ve boyutta iade edilmiş bir
// depolama varsa onu (paralel olarak sıfırlayıp) yeniden kullanır.
func (b cpuBackend) Alloc(n int, dt DType) Storage { return b.ayir(n, dt, true) }

// allocBos (bosAyirici) içeriği belirsiz depolama verir: havuzdan gelen
// depolama sıfırlanmaz. Yalnızca her elemanı yazılacak çıktılar için.
func (b cpuBackend) allocBos(n int, dt DType) Storage { return b.ayir(n, dt, false) }

func (cpuBackend) ayir(n int, dt DType, sifirla bool) Storage {
	cpuTurDenetle(dt)
	if n >= havuzEnAz {
		if h := havuzFor(havuzAnahtar{dt, n}, false); h != nil {
			if s, ok := h.Get().(*cpuStorage); ok {
				s.iade = false
				if !sifirla {
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
	return &cpuStorage{tur: dt, h: make([]uint16, n)}
}

// Release depolamayı havuza iade eder (Releaser). Çağıran, depolamaya başka
// hiçbir görünümün bakmadığını güvence eder.
func (cpuBackend) Release(s Storage) {
	c, ok := s.(*cpuStorage)
	if !ok || c.Len() < havuzEnAz {
		return
	}
	if c.iade {
		panic("tensor: depolama havuza iki kez iade edildi")
	}
	c.iade = true
	havuzFor(havuzAnahtar{c.tur, c.Len()}, true).Put(c)
}

func (cpuBackend) FromHost(data []float32, dt DType) Storage {
	cpuTurDenetle(dt)
	if dt == Float32 {
		d := make([]float32, len(data))
		copy(d, data)
		return &cpuStorage{d: d}
	}
	h := make([]uint16, len(data))
	daralt(h, data, dt)
	return &cpuStorage{tur: dt, h: h}
}

func (cpuBackend) ToHost(src View) []float32 {
	out := make([]float32, src.Numel())
	if src.DType() == Float32 {
		copyTo(out, src)
	} else {
		genisletInto(out, src)
	}
	return out
}

func (b cpuBackend) Copy(dst Storage, dstOff int, src View) {
	n := src.Numel()
	c := cpuDepo(dst)
	if c.tur == Float32 {
		if src.DType() == Float32 {
			copyTo(c.d[dstOff:dstOff+n], src)
		} else {
			genisletInto(c.d[dstOff:dstOff+n], src)
		}
		return
	}
	hedef := c.h[dstOff : dstOff+n]
	if s := cpuDepo(src.S); s.tur == c.tur && src.IsContiguous() {
		// Aynı tür: bit düzeyinde kopya (yuvarlama yok).
		k := s.h[src.Offset : src.Offset+n]
		parallelFor(n, 1, func(lo, hi int) { copy(hedef[lo:hi], k[lo:hi]) })
		return
	}
	// Farklı tür ya da adımlı: float32 üzerinden (16 bit → float32 → 16 bit kayıpsızdır).
	daralt(hedef, b.ToHost(src), c.tur)
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
	c := cpuDepo(dst)
	if c.tur != Float32 {
		hv := daraltBir(v, c.tur)
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
		if v == 0 {
			clear(p)
			return
		}
		for i := range p {
			p[i] = v
		}
	})
}

// ----------------------------------------------------------------------------
// Tür uyarlayıcıları: hesap çekirdekleri float32 görür
// ----------------------------------------------------------------------------

// oku görünümü float32 okunabilir hale getirir: float32 ise aynen, 16 bit ise
// mantıksal elemanları float32'ye açılmış bitişik geçici bir görünüm.
func oku(v View) View {
	if v.S == nil || v.S.DType() == Float32 {
		return v
	}
	tmp := make([]float32, v.Numel())
	genisletInto(tmp, v)
	return View{S: &cpuStorage{d: tmp}, Shape: v.Shape, Strides: contiguousStrides(v.Shape)}
}

func okuP(v *View) *View {
	if v == nil {
		return nil
	}
	w := oku(*v)
	return &w
}

// okuD düz depolamanın (tüm elemanlar) float32 karşılığı.
func okuD(s Storage) Storage {
	if s == nil || s.DType() == Float32 {
		return s
	}
	c := cpuDepo(s)
	tmp := make([]float32, len(c.h))
	genislet16(tmp, c.h, c.tur)
	return &cpuStorage{d: tmp}
}

var bitir0 = func() {}

// yaz hedef depolamanın float32 karşılığını ve geri yazma işlevini verir.
// 16 bitlik hedefte float32 geçici tampon kullanılır (eski doğruysa mevcut
// içerikle başlar: biriktirme ve yerinde güncellemeler için); geri yazma
// float32 sonucu hedef türe bir kez yuvarlar.
func yaz(s Storage, eski bool) (Storage, func()) {
	if s == nil || s.DType() == Float32 {
		return s, bitir0
	}
	c := cpuDepo(s)
	tmp := &cpuStorage{d: make([]float32, len(c.h))}
	if eski {
		genislet16(tmp.d, c.h, c.tur)
	}
	return tmp, func() { daralt(c.h, tmp.d, c.tur) }
}

func (b cpuBackend) AddTo(dst, src Storage, scale float32) {
	d, son := yaz(dst, true)
	b.f32AddTo(d, okuD(src), scale)
	son()
}

func (b cpuBackend) ScatterAdd(dst Storage, dstOff int, dstStrides []int, src Storage, shape []int) {
	d, son := yaz(dst, true)
	b.f32ScatterAdd(d, dstOff, dstStrides, okuD(src), shape)
	son()
}

func (b cpuBackend) ExpandAdd(dst Storage, dstShape []int, src Storage, srcShape []int, scale float32) {
	d, son := yaz(dst, true)
	b.f32ExpandAdd(d, dstShape, okuD(src), srcShape, scale)
	son()
}

func (b cpuBackend) Binary(op BinOp, out Storage, outShape []int, a, bv View) {
	o, son := yaz(out, false)
	b.f32Binary(op, o, outShape, oku(a), oku(bv))
	son()
}

func (b cpuBackend) BinaryGrad(mode GradMode, dst Storage, dstShape, outShape []int, g Storage, o View, y Storage) {
	d, son := yaz(dst, true)
	b.f32BinaryGrad(mode, d, dstShape, outShape, okuD(g), oku(o), okuD(y))
	son()
}

func (b cpuBackend) Unary(op UnOp, p float32, out Storage, x View) {
	o, son := yaz(out, false)
	b.f32Unary(op, p, o, oku(x))
	son()
}

func (b cpuBackend) UnaryGrad(op UnOp, p float32, gx, g Storage, x View, y Storage) {
	d, son := yaz(gx, true)
	b.f32UnaryGrad(op, p, d, okuD(g), oku(x), okuD(y))
	son()
}

func (b cpuBackend) Sum(out Storage, x View, axis int, scale float32) {
	o, son := yaz(out, false)
	b.f32Sum(o, oku(x), axis, scale)
	son()
}

func (b cpuBackend) SumAll(out Storage, x View, scale float32) {
	o, son := yaz(out, false)
	b.f32SumAll(o, oku(x), scale)
	son()
}

func (b cpuBackend) Max(out Storage, x View, axis int) {
	o, son := yaz(out, false)
	b.f32Max(o, oku(x), axis)
	son()
}

func (b cpuBackend) MaxGrad(gx, g Storage, x View, axis int) {
	d, son := yaz(gx, true)
	b.f32MaxGrad(d, okuD(g), oku(x), axis)
	son()
}

func (b cpuBackend) ArgMax(x View, axis int) []int { return b.f32ArgMax(oku(x), axis) }

func (b cpuBackend) Softmax(out Storage, x View, axis int, logKip bool) {
	o, son := yaz(out, false)
	b.f32Softmax(o, oku(x), axis, logKip)
	son()
}

func (b cpuBackend) SoftmaxGrad(gx, g, y Storage, shape []int, axis int, logKip bool) {
	d, son := yaz(gx, true)
	b.f32SoftmaxGrad(d, okuD(g), okuD(y), shape, axis, logKip)
	son()
}

func (b cpuBackend) Gemm(batch, M, N, K int, a View, transA bool, bv View, transB bool, c Storage) {
	o, son := yaz(c, true)
	b.f32Gemm(batch, M, N, K, oku(a), transA, oku(bv), transB, o)
	son()
}

func (b cpuBackend) Embedding(out Storage, w View, idx []int, D int) {
	o, son := yaz(out, false)
	b.f32Embedding(o, oku(w), idx, D)
	son()
}

func (b cpuBackend) EmbeddingGrad(gw, g Storage, idx []int, D int) {
	d, son := yaz(gw, true)
	b.f32EmbeddingGrad(d, okuD(g), idx, D)
	son()
}

func (b cpuBackend) CrossEntropy(x View, targets []int, C int) (Storage, Storage) {
	return b.f32CrossEntropy(oku(x), targets, C)
}

func (b cpuBackend) CrossEntropyGrad(gx, g Storage, x View, lse Storage, targets []int, C int) {
	d, son := yaz(gx, true)
	b.f32CrossEntropyGrad(d, okuD(g), oku(x), okuD(lse), targets, C)
	son()
}

func (b cpuBackend) LayerNorm(out Storage, x View, gamma, beta *View, D int, eps float32) (Storage, Storage) {
	o, son := yaz(out, false)
	mean, rstd := b.f32LayerNorm(o, oku(x), okuP(gamma), okuP(beta), D, eps)
	son()
	return mean, rstd
}

func (b cpuBackend) LayerNormGrad(gx, ggamma, gbeta, g Storage, x View, gamma *View, mean, rstd Storage, D int) {
	dx, son1 := yaz(gx, true)
	dg, son2 := yaz(ggamma, true)
	db, son3 := yaz(gbeta, true)
	b.f32LayerNormGrad(dx, dg, db, okuD(g), oku(x), okuP(gamma), mean, rstd, D)
	son1()
	son2()
	son3()
}

func (b cpuBackend) RMSNorm(out Storage, x View, gamma *View, D int, eps float32) Storage {
	o, son := yaz(out, false)
	rinv := b.f32RMSNorm(o, oku(x), okuP(gamma), D, eps)
	son()
	return rinv
}

func (b cpuBackend) RMSNormGrad(gx, ggamma, g Storage, x View, gamma *View, rinv Storage, D int) {
	dx, son1 := yaz(gx, true)
	dg, son2 := yaz(ggamma, true)
	b.f32RMSNormGrad(dx, dg, okuD(g), oku(x), okuP(gamma), rinv, D)
	son1()
	son2()
}

func (b cpuBackend) SGDStep(p, g, vel Storage, lr, momentum, wd float32) {
	pd, son1 := yaz(p, true)
	vd, son2 := yaz(vel, true)
	b.f32SGDStep(pd, okuD(g), vd, lr, momentum, wd)
	son1()
	son2()
}

func (b cpuBackend) AdamWStep(p, g, m, v Storage, lr, b1, b2, eps, wd, bc1, bc2 float32) {
	pd, son1 := yaz(p, true)
	md, son2 := yaz(m, true)
	vd, son3 := yaz(v, true)
	b.f32AdamWStep(pd, okuD(g), md, vd, lr, b1, b2, eps, wd, bc1, bc2)
	son1()
	son2()
	son3()
}

func (b cpuBackend) SumSquares(s Storage) float64 { return b.f32SumSquares(okuD(s)) }

func (b cpuBackend) Scale(s Storage, k float32) {
	d, son := yaz(s, true)
	b.f32Scale(d, k)
	son()
}

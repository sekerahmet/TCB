package tensor

import (
	"fmt"
	"math"
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
	dtyp    DType
	d       []float32 // Float32
	h       []uint16  // BFloat16, Float16
	release bool      // havuza iade edildi (yeniden ayrılana kadar kullanılmamalı)
}

func (s *cpuStorage) Device() Device { return CPU }
func (s *cpuStorage) DType() DType   { return s.dtyp }
func (s *cpuStorage) Len() int {
	if s.dtyp == Float32 {
		return len(s.d)
	}
	return len(s.h)
}

func cpuStore(s Storage) *cpuStorage {
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
	c := cpuStore(s)
	if c.dtyp != Float32 {
		panic(fmt.Sprintf("tensor: CPU çekirdeği float32 depolama bekliyordu, %s verildi (iç hata)", c.dtyp))
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
	sign := uint16(b>>16) & 0x8000
	us := int((b >> 23) & 0xFF)
	ak := b & 0x7FFFFF
	if us == 0xFF {
		if ak != 0 {
			return sign | 0x7E00 | uint16(ak>>13) // sessiz NaN
		}
		return sign | 0x7C00
	}
	e := us - 127 + 15
	if e >= 0x1F {
		return sign | 0x7C00
	}
	if e <= 0 {
		// Normal altı ya da sıfır: anlamlı kısım (gizli bit dahil) kaydırılır.
		if e < -10 {
			return sign
		}
		ak |= 0x800000
		shiftN := uint(14 - e)
		m := ak >> shiftN
		rest := ak & (1<<shiftN - 1)
		half := uint32(1) << (shiftN - 1)
		if rest > half || (rest == half && m&1 == 1) {
			m++ // en küçük normale taşabilir: kodlama yine doğrudur
		}
		return sign | uint16(m)
	}
	h := uint32(e)<<10 | ak>>13
	rest := ak & 0x1FFF
	if rest > 0x1000 || (rest == 0x1000 && h&1 == 1) {
		h++ // üsse (ve sonsuza) taşma kodlamada doğal olarak doğrudur
	}
	return sign | uint16(h)
}

// f16ToF32 yarım hassasiyeti float32'ye açar (kayıpsız).
func f16ToF32(h uint16) float32 {
	sign := uint32(h&0x8000) << 16
	us := uint32(h>>10) & 0x1F
	ak := uint32(h & 0x3FF)
	switch us {
	case 0x1F:
		return math.Float32frombits(sign | 0x7F800000 | ak<<13)
	case 0:
		if ak == 0 {
			return math.Float32frombits(sign)
		}
		e := uint32(127 - 14)
		for ak&0x400 == 0 {
			ak <<= 1
			e--
		}
		return math.Float32frombits(sign | e<<23 | (ak&0x3FF)<<13)
	}
	return math.Float32frombits(sign | (us+127-15)<<23 | ak<<13)
}

func narrowOne(v float32, dt DType) uint16 {
	if dt == BFloat16 {
		return f32ToBF16(v)
	}
	return f32ToF16(v)
}

// daralt dst = tür(src) (paralel).
func narrow(dst []uint16, src []float32, dt DType) {
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
func widen16(dst []float32, src []uint16, dt DType) {
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
func widenInto(out []float32, v View) {
	c := cpuStore(v.S)
	if v.IsContiguous() {
		widen16(out, c.h[v.Offset:v.Offset+len(out)], c.dtyp)
		return
	}
	bf := c.dtyp == BFloat16
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
// Tür uyarlayıcıları: hesap çekirdekleri float32 görür
// ----------------------------------------------------------------------------

// gecici, bir arka uç çağrısının 16 bitlik depolamalar için açtığı float32 geçici
// tamponları toplar; bitir hepsini havuza iade eder (çağrı bitince hiçbir görünüm onlara
// bakmaz). Böylece 16 bitlik yoldaki her işlem tam boy yeni ayırma yapmaz; float32 yolu
// geçici tampon kullanmaz.
type temp struct {
	s [8]*cpuStorage
	n int
	f []*cpuStorage // 8'den fazlası (nadir)
}

// tampon n elemanlı float32 geçici depolama verir; sifirla doğruysa sıfırlanmıştır.
func (g *temp) buffer(n int, reset bool) *cpuStorage {
	t := cpuBackend{}.allocate(n, Float32, reset).(*cpuStorage)
	if g.n < len(g.s) {
		g.s[g.n] = t
		g.n++
	} else {
		g.f = append(g.f, t)
	}
	return t
}

// bitir geçici tamponları havuza iade eder.
func (g *temp) finish() {
	for _, t := range g.s[:g.n] {
		cpuBackend{}.Release(t)
	}
	for _, t := range g.f {
		cpuBackend{}.Release(t)
	}
	g.n, g.f = 0, nil
}

// oku görünümü float32 okunabilir hale getirir: float32 ise aynen, 16 bit ise
// mantıksal elemanları float32'ye açılmış bitişik geçici bir görünüm.
func (g *temp) read(v View) View {
	if v.S == nil || v.S.DType() == Float32 {
		return v
	}
	tmp := g.buffer(v.Numel(), false)
	widenInto(tmp.d, v)
	return View{S: tmp, Shape: v.Shape, Strides: contiguousStrides(v.Shape)}
}

func (g *temp) readP(v *View) *View {
	if v == nil {
		return nil
	}
	w := g.read(*v)
	return &w
}

// okuD düz depolamanın (tüm elemanlar) float32 karşılığı.
func (g *temp) readD(s Storage) Storage {
	if s == nil || s.DType() == Float32 {
		return s
	}
	c := cpuStore(s)
	tmp := g.buffer(len(c.h), false)
	widen16(tmp.d, c.h, c.dtyp)
	return tmp
}

var finish0 = func() {}

// yaz hedef depolamanın float32 karşılığını ve geri yazma işlevini verir.
// 16 bitlik hedefte float32 geçici tampon kullanılır (eski doğruysa mevcut
// içerikle başlar: biriktirme ve yerinde güncellemeler için; değilse sıfırdır);
// geri yazma float32 sonucu hedef türe bir kez yuvarlar.
func (g *temp) write(s Storage, old bool) (Storage, func()) {
	if s == nil || s.DType() == Float32 {
		return s, finish0
	}
	c := cpuStore(s)
	tmp := g.buffer(len(c.h), !old)
	if old {
		widen16(tmp.d, c.h, c.dtyp)
	}
	return tmp, func() { narrow(c.h, tmp.d, c.dtyp) }
}

func (b cpuBackend) AddTo(dst, src Storage, scale float32) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(dst, true)
	b.f32AddTo(d, gt.readD(src), scale)
	last()
}

func (b cpuBackend) ScatterAdd(dst Storage, dstOff int, dstStrides []int, src Storage, shape []int) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(dst, true)
	b.f32ScatterAdd(d, dstOff, dstStrides, gt.readD(src), shape)
	last()
}

func (b cpuBackend) ExpandAdd(dst Storage, dstShape []int, src Storage, srcShape []int, scale float32) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(dst, true)
	b.f32ExpandAdd(d, dstShape, gt.readD(src), srcShape, scale)
	last()
}

func (b cpuBackend) Binary(op BinOp, out Storage, outShape []int, a, bv View) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Binary(op, o, outShape, gt.read(a), gt.read(bv))
	last()
}

func (b cpuBackend) BinaryGrad(mode GradMode, dst Storage, dstShape, outShape []int, g Storage, o View, y Storage) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(dst, true)
	b.f32BinaryGrad(mode, d, dstShape, outShape, gt.readD(g), gt.read(o), gt.readD(y))
	last()
}

func (b cpuBackend) Unary(op UnOp, p float32, out Storage, x View) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Unary(op, p, o, gt.read(x))
	last()
}

func (b cpuBackend) UnaryGrad(op UnOp, p float32, gx, g Storage, x View, y Storage) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32UnaryGrad(op, p, d, gt.readD(g), gt.read(x), gt.readD(y))
	last()
}

func (b cpuBackend) Sum(out Storage, x View, axis int, scale float32) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Sum(o, gt.read(x), axis, scale)
	last()
}

func (b cpuBackend) SumAll(out Storage, x View, scale float32) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32SumAll(o, gt.read(x), scale)
	last()
}

func (b cpuBackend) Max(out Storage, x View, axis int) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Max(o, gt.read(x), axis)
	last()
}

func (b cpuBackend) MaxGrad(gx, g Storage, x View, axis int) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32MaxGrad(d, gt.readD(g), gt.read(x), axis)
	last()
}

func (b cpuBackend) ArgMax(x View, axis int) []int {
	var gt temp
	defer gt.finish()
	return b.f32ArgMax(gt.read(x), axis)
}

func (b cpuBackend) Softmax(out Storage, x View, axis int, logMode bool) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Softmax(o, gt.read(x), axis, logMode)
	last()
}

func (b cpuBackend) SoftmaxGrad(gx, g, y Storage, shape []int, axis int, logMode bool) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32SoftmaxGrad(d, gt.readD(g), gt.readD(y), shape, axis, logMode)
	last()
}

func (b cpuBackend) Gemm(batch, M, N, K int, a View, transA bool, bv View, transB bool, c Storage) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(c, true)
	// 16 bitlik girdiler havuzdan gelen float32 tamponlara açılır (paketleme sırasında açmak
	// ölçümde daha yavaştı: doğrudan A okuması ve vektörel kopya kaybediliyordu).
	b.f32Gemm(batch, M, N, K, gt.read(a), transA, gt.read(bv), transB, o)
	last()
}

func (b cpuBackend) Embedding(out Storage, w View, idx []int, D int) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Embedding(o, gt.read(w), idx, D)
	last()
}

func (b cpuBackend) EmbeddingGrad(gw, g Storage, idx []int, D int) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gw, true)
	b.f32EmbeddingGrad(d, gt.readD(g), idx, D)
	last()
}

func (b cpuBackend) CrossEntropy(x View, targets []int, C int) (Storage, Storage) {
	var gt temp
	defer gt.finish()
	return b.f32CrossEntropy(gt.read(x), targets, C)
}

func (b cpuBackend) CrossEntropyGrad(gx, g Storage, x View, lse Storage, targets []int, C int) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32CrossEntropyGrad(d, gt.readD(g), gt.read(x), gt.readD(lse), targets, C)
	last()
}

func (b cpuBackend) LayerNorm(out Storage, x View, gamma, beta *View, D int, eps float32) (Storage, Storage) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	mean, rstd := b.f32LayerNorm(o, gt.read(x), gt.readP(gamma), gt.readP(beta), D, eps)
	last()
	return mean, rstd
}

func (b cpuBackend) LayerNormGrad(gx, ggamma, gbeta, g Storage, x View, gamma *View, mean, rstd Storage, D int) {
	var gt temp
	defer gt.finish()
	dx, last1 := gt.write(gx, true)
	dg, last2 := gt.write(ggamma, true)
	db, last3 := gt.write(gbeta, true)
	b.f32LayerNormGrad(dx, dg, db, gt.readD(g), gt.read(x), gt.readP(gamma), mean, rstd, D)
	last1()
	last2()
	last3()
}

func (b cpuBackend) RMSNorm(out Storage, x View, gamma *View, D int, eps float32) Storage {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	rinv := b.f32RMSNorm(o, gt.read(x), gt.readP(gamma), D, eps)
	last()
	return rinv
}

func (b cpuBackend) RMSNormGrad(gx, ggamma, g Storage, x View, gamma *View, rinv Storage, D int) {
	var gt temp
	defer gt.finish()
	dx, last1 := gt.write(gx, true)
	dg, last2 := gt.write(ggamma, true)
	b.f32RMSNormGrad(dx, dg, gt.readD(g), gt.read(x), gt.readP(gamma), rinv, D)
	last1()
	last2()
}

func (b cpuBackend) Attention(out Storage, q, k, v View, mask Storage, p AttentionParams) Storage {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	lse := b.f32Attention(o, gt.read(q), gt.read(k), gt.read(v), gt.readD(mask), p)
	last()
	return lse
}

func (b cpuBackend) AttentionGrad(gq, gk, gv, g Storage, q, k, v View, mask Storage, o, lse Storage, p AttentionParams) {
	var gt temp
	defer gt.finish()
	// q, k, v aynı tensörse gradyanları aynı depolamadır: 16 bitlikte tek geçici tampon
	// kullanılır (ayrı tamponlar birbirinin sonucunu ezerdi).
	var d [3]Storage
	var lasts []func()
	g3 := [3]Storage{gq, gk, gv}
	for i, s := range g3 {
		for j := 0; j < i && s != nil; j++ {
			if g3[j] == s {
				d[i] = d[j]
			}
		}
		if s != nil && d[i] == nil {
			var last func()
			d[i], last = gt.write(s, true)
			lasts = append(lasts, last)
		}
	}
	b.f32AttentionGrad(d[0], d[1], d[2], gt.readD(g), gt.read(q), gt.read(k), gt.read(v), gt.readD(mask), gt.readD(o), lse, p)
	for _, last := range lasts {
		last()
	}
}

func (b cpuBackend) SGDStep(p, g, vel Storage, lr, momentum, wd float32) {
	var gt temp
	defer gt.finish()
	pd, last1 := gt.write(p, true)
	vd, last2 := gt.write(vel, true)
	b.f32SGDStep(pd, gt.readD(g), vd, lr, momentum, wd)
	last1()
	last2()
}

func (b cpuBackend) AdamWStep(p, g, m, v Storage, lr, b1, b2, eps, wd, bc1, bc2 float32) {
	var gt temp
	defer gt.finish()
	pd, last1 := gt.write(p, true)
	md, last2 := gt.write(m, true)
	vd, last3 := gt.write(v, true)
	b.f32AdamWStep(pd, gt.readD(g), md, vd, lr, b1, b2, eps, wd, bc1, bc2)
	last1()
	last2()
	last3()
}

func (b cpuBackend) SumSquares(s Storage) float64 {
	var gt temp
	defer gt.finish()
	return b.f32SumSquares(gt.readD(s))
}

func (b cpuBackend) Scale(s Storage, k float32) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(s, true)
	b.f32Scale(d, k)
	last()
}

package tensor

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// CPU arka ucu: genişletilmiş eleman bazlı işlemler. SiLU sigmoid gibi vektörel üstelle
// (expVek) hesaplanır; diğer tekli işlemler float64 kitaplık işlevleriyle hesaplanıp
// float32'ye bir kez yuvarlanır (başvuru doğruluğu). 16 bitlik türler tür uyarlayıcılarıyla
// (gecici) float32'ye açılır.

// unaryLoopWide unaryDongu'nun genişletilmiş işlemleri: y = f(x).
func unaryLoopWide(op UnOp, y, x []float32) {
	f64 := func(f func(float64) float64) {
		for i, v := range x {
			y[i] = float32(f(float64(v)))
		}
	}
	switch op {
	case OpSilu:
		siluForward(y, x)
	case OpTan:
		f64(math.Tan)
	case OpAsin:
		f64(math.Asin)
	case OpAcos:
		f64(math.Acos)
	case OpAtan:
		f64(math.Atan)
	case OpSinh:
		f64(math.Sinh)
	case OpCosh:
		f64(math.Cosh)
	case OpLog1p:
		f64(math.Log1p)
	case OpLog2:
		f64(math.Log2)
	case OpLog10:
		f64(math.Log10)
	case OpExpm1:
		f64(math.Expm1)
	case OpFloor:
		f64(math.Floor)
	case OpCeil:
		f64(math.Ceil)
	case OpSign:
		for i, v := range x {
			y[i] = sign32(v)
		}
	case OpAbs:
		for i, v := range x {
			y[i] = abs32(v)
		}
	case OpGeluExact:
		f64(geluExactValue)
	default:
		panic(messages.Internal(fmt.Sprintf("unknown unary op %d", op)))
	}
}

const (
	invSqrt2   = 0.70710678118654752440 // 1/√2
	invSqrt2Pi = 0.39894228040143267794 // 1/√(2π)
)

// geluExactValue x·Φ(x), Φ(x) = ½·erfc(−x/√2). erfc kullanılır, 1+erf değil: x çok
// negatifken Φ(x) 1'e yakın iki sayının farkı olarak değil, kuyruk olarak göreli
// doğrulukla bulunur. Φ(x) = 0 olan uçta (x → −∞) sonuç 0'dır (−∞·0 NaN üretirdi).
func geluExactValue(x float64) float64 {
	phi := 0.5 * math.Erfc(-x*invSqrt2)
	if phi == 0 {
		return 0
	}
	return x * phi
}

// geluExactSlope x·Φ(x)'in türevi Φ(x) + x·φ(x), φ standart normal yoğunluğu. φ(x) = 0
// iken (|x| büyük ya da sonsuz) türev Φ(x)'tir: x·0 sonsuzda NaN üretirdi.
func geluExactSlope(x float64) float64 {
	phi := 0.5 * math.Erfc(-x*invSqrt2)
	density := invSqrt2Pi * math.Exp(-0.5*x*x)
	if density == 0 {
		return phi
	}
	return phi + x*density
}

// sign32 x > 0 ise 1, x < 0 ise −1, sıfırda (işaretiyle) sıfır, NaN'da NaN.
func sign32(x float32) float32 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return x
}

// siluForward y = x·σ(x); σ sigmoid'deki gibi e = e^{-|x|} ile sayısal kararlı biçimde
// (x ≥ 0 için 1/(1+e), aksi halde e/(1+e)) hesaplanır. x < −87 civarında σ(x) normal
// altı bölgededir: sonucun göreli hatası büyür, mutlak hatası 1e-38'in altında kalır.
// sigmoidBlock bir blok için dst[i] = σ(xs[i]) hesaplar (len(xs) ≤ vecBlock; dst, xs ile örtüşmez):
// e = e^{-|x|}; x ≥ 0 için 1/(1+e), aksi halde e/(1+e) (sayısal kararlı).
func sigmoidBlock(dst, xs []float32) {
	for i, v := range xs {
		dst[i] = -abs32(v)
	}
	expVec(dst, dst)
	for i, v := range xs {
		e := dst[i]
		if v >= 0 {
			dst[i] = 1 / (1 + e)
		} else {
			dst[i] = e / (1 + e)
		}
	}
}

func siluForward(y, x []float32) {
	var t [vecBlock]float32
	for i0 := 0; i0 < len(x); i0 += vecBlock {
		xs := x[i0:min(i0+vecBlock, len(x))]
		tb := t[:len(xs)]
		sigmoidBlock(tb, xs)
		ys := y[i0 : i0+len(xs)]
		for i, v := range xs {
			s := tb[i]
			if s == 0 {
				v = 0 // x -> -sonsuz: x·σ(x) -> 0 (-sonsuz·0 NaN üretirdi)
			}
			ys[i] = v * s
		}
	}
}

// siluBackward gx += g·d, d = s + x·s·(1−s) (s = σ(x)). s·(1−s) = 0 iken (|x| büyük ya da
// sonsuz) d = s alınır: x·0 sonsuzda NaN üretirdi.
func siluBackward(gx, g, x []float32) {
	var t [vecBlock]float32
	for i0 := 0; i0 < len(x); i0 += vecBlock {
		xs := x[i0:min(i0+vecBlock, len(x))]
		tb := t[:len(xs)]
		sigmoidBlock(tb, xs)
		gs, gxs := g[i0:i0+len(xs)], gx[i0:i0+len(xs)]
		for i, v := range xs {
			s := tb[i]
			d := s
			if q := s * (1 - s); q != 0 {
				d = s + v*q
			}
			gxs[i] += gs[i] * d
		}
	}
}

// unaryGradLoopWide unaryGradDongu'nun genişletilmiş işlemleri: gx += g·f'(x, y).
// Türev çarpanları float64'te hesaplanır.
func unaryGradLoopWide(op UnOp, gx, g, x, y []float32) {
	f64 := func(f func(x, y float64) float64) {
		for i, v := range g {
			gx[i] += v * float32(f(float64(x[i]), float64(y[i])))
		}
	}
	switch op {
	case OpSilu:
		siluBackward(gx, g, x)
	case OpTan:
		f64(func(_, y float64) float64 { return 1 + y*y })
	case OpAsin:
		f64(func(x, _ float64) float64 { return 1 / math.Sqrt((1-x)*(1+x)) })
	case OpAcos:
		f64(func(x, _ float64) float64 { return -1 / math.Sqrt((1-x)*(1+x)) })
	case OpAtan:
		f64(func(x, _ float64) float64 { return 1 / (1 + x*x) })
	case OpSinh:
		f64(func(x, _ float64) float64 { return math.Cosh(x) })
	case OpCosh:
		f64(func(x, _ float64) float64 { return math.Sinh(x) })
	case OpLog1p:
		f64(func(x, _ float64) float64 { return 1 / (1 + x) })
	case OpLog2:
		f64(func(x, _ float64) float64 { return 1 / (x * math.Ln2) })
	case OpLog10:
		f64(func(x, _ float64) float64 { return 1 / (x * math.Ln10) })
	case OpExpm1:
		// exp(x), y+1 değil: x çok negatifken y = -1'e yuvarlanır ve y+1 türevi sıfırlardı.
		f64(func(x, _ float64) float64 { return math.Exp(x) })
	case OpAbs:
		// Türev işaret(x): 0'da (±0) çarpan sıfırdır, NaN'da NaN.
		for i, v := range g {
			gx[i] += v * sign32(x[i])
		}
	case OpGeluExact:
		f64(func(x, _ float64) float64 { return geluExactSlope(x) })
	case OpFloor, OpCeil, OpSign:
		// Türev 0 (tensör katmanı bu işlemlerin geri yayılımını arka uca göndermez).
	default:
		panic(messages.Internal(fmt.Sprintf("unknown unary op %d", op)))
	}
}

// ----------------------------------------------------------------------------
// İkili: iki argümanlı arktanjant ve karşılaştırmalar
// ----------------------------------------------------------------------------

// binValue tek elemanlı ikili işlem (genişletilmiş işlemler).
func binValue(op BinOp, a, b float32) float32 {
	var d bool
	switch op {
	case OpAtan2:
		return float32(math.Atan2(float64(a), float64(b)))
	case OpLt:
		d = a < b
	case OpGt:
		d = a > b
	case OpLe:
		d = a <= b
	case OpGe:
		d = a >= b
	default:
		panic(messages.Internal(fmt.Sprintf("unknown binary op %d", op)))
	}
	if d {
		return 1
	}
	return 0
}

// binFlatWide binFlat'in genişletilmiş işlemleri.
func binFlatWide(op BinOp, out, a, b []float32) {
	for i, x := range a {
		out[i] = binValue(op, x, b[i])
	}
}

// binStridedWide binStrided'ın genişletilmiş işlemleri.
func binStridedWide(op BinOp, out []float32, oo, os int, a []float32, ao, as int, b []float32, bo, bs int, n int) {
	for i := 0; i < n; i++ {
		out[oo+i*os] = binValue(op, a[ao+i*as], b[bo+i*bs])
	}
}

// ----------------------------------------------------------------------------
// Sınırlama, koşullu seçim, arktanjant gradyanı
// ----------------------------------------------------------------------------

// clamp32 x'i [lo, hi]'ye sınırlar; NaN korunur.
func clamp32(x, lo, hi float32) float32 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func (cpuBackend) f32Clamp(out Storage, x View, lo, hi float32) {
	xd := gatherCPU(x)
	y := cd(out)
	parallelFor(len(xd), 1, func(a, b int) {
		for i := a; i < b; i++ {
			y[i] = clamp32(xd[i], lo, hi)
		}
	})
}

func (cpuBackend) f32ClampGrad(gx, g Storage, x View, lo, hi float32) {
	xd := gatherCPU(x)
	pg, gd := cd(gx), cd(g)
	parallelFor(len(xd), 1, func(a, b int) {
		for i := a; i < b; i++ {
			if v := xd[i]; v >= lo && v <= hi {
				pg[i] += gd[i]
			}
		}
	})
}

func (cpuBackend) f32Where(out Storage, outShape []int, c, a, b View) {
	od, cdat, ad, bd := cd(out), cd(c.S), cd(a.S), cd(b.S)
	sc := BroadcastStrides(c.Strides, c.Shape, outShape)
	sa := BroadcastStrides(a.Strides, a.Shape, outShape)
	sb := BroadcastStrides(b.Strides, b.Shape, outShape)
	iterRowsPar(outShape, []int{c.Offset, a.Offset, b.Offset, 0}, [][]int{sc, sa, sb, ContiguousStrides(outShape)}, 3,
		func(off []int, m int, in []int) {
			for i := 0; i < m; i++ {
				v := bd[off[2]+i*in[2]]
				if cdat[off[0]+i*in[0]] != 0 {
					v = ad[off[1]+i*in[1]]
				}
				od[off[3]+i*in[3]] = v
			}
		})
}

func (cpuBackend) f32WhereGrad(dst Storage, dstShape, outShape []int, g Storage, c View, pick bool) {
	dg, gd, cdat := cd(dst), cd(g), cd(c.S)
	sc := BroadcastStrides(c.Strides, c.Shape, outShape)
	sd := BroadcastStrides(ContiguousStrides(dstShape), dstShape, outShape)
	// Hedefin indirgenen (adım 0) eksenleri bölünmez: her hedef elemanı tek görevde ve
	// sabit sırayla biriktirilir (belirlenimci).
	iterRowsPar(outShape, []int{0, 0, c.Offset}, [][]int{ContiguousStrides(outShape), sd, sc}, 1,
		func(off []int, m int, in []int) {
			for i := 0; i < m; i++ {
				if (cdat[off[2]+i*in[2]] != 0) == pick {
					dg[off[1]+i*in[1]] += gd[off[0]+i*in[0]]
				}
			}
		})
}

// f32Atan2Grad önce çıktı şeklinde iki terimi (float64 ile) hesaplar, sonra her birini
// girdinin şekline indirgeyerek ekler.
func (b cpuBackend) f32Atan2Grad(ga, gb Storage, aShape, bShape, outShape []int, g Storage, a, bv View) {
	n := NumelOf(outShape)
	if n == 0 {
		return
	}
	gd := cd(g)
	ta := make([]float32, n)
	tb := make([]float32, n)
	sa := BroadcastStrides(a.Strides, a.Shape, outShape)
	sb := BroadcastStrides(bv.Strides, bv.Shape, outShape)
	ad, bd := cd(a.S), cd(bv.S)
	iterRowsPar(outShape, []int{a.Offset, bv.Offset, 0}, [][]int{sa, sb, ContiguousStrides(outShape)}, 2,
		func(off []int, m int, in []int) {
			for i := 0; i < m; i++ {
				y, x := float64(ad[off[0]+i*in[0]]), float64(bd[off[1]+i*in[1]])
				o := off[2] + i*in[2]
				r := y*y + x*x
				if r == 0 || math.IsInf(r, 1) {
					// 0'da tanımsız, sonsuzda limit: terimler 0 (NaN girdide r NaN'dır, yayılır).
					continue
				}
				gv := float64(gd[o])
				ta[o] = float32(gv * x / r)
				tb[o] = float32(-gv * y / r)
			}
		})
	if ga != nil {
		b.f32BinaryGrad(GradPlus, ga, aShape, outShape, &cpuStorage{d: ta, dtyp: Float32}, View{}, nil)
	}
	if gb != nil {
		b.f32BinaryGrad(GradPlus, gb, bShape, outShape, &cpuStorage{d: tb, dtyp: Float32}, View{}, nil)
	}
}

// ----------------------------------------------------------------------------
// Tür uyarlayıcıları
// ----------------------------------------------------------------------------

func (b cpuBackend) Clamp(out Storage, x View, lo, hi float32) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Clamp(o, gt.read(x), lo, hi)
	last()
}

func (b cpuBackend) ClampGrad(gx, g Storage, x View, lo, hi float32) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32ClampGrad(d, gt.readD(g), gt.read(x), lo, hi)
	last()
}

func (b cpuBackend) Where(out Storage, outShape []int, c, a, bv View) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	b.f32Where(o, outShape, gt.read(c), gt.read(a), gt.read(bv))
	last()
}

func (b cpuBackend) WhereGrad(dst Storage, dstShape, outShape []int, g Storage, c View, pick bool) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(dst, true)
	b.f32WhereGrad(d, dstShape, outShape, gt.readD(g), gt.read(c), pick)
	last()
}

func (b cpuBackend) Atan2Grad(ga, gb Storage, aShape, bShape, outShape []int, g Storage, a, bv View) {
	var gt temp
	defer gt.finish()
	d, last := gt.writeAll(ga, gb)
	b.f32Atan2Grad(d[0], d[1], aShape, bShape, outShape, gt.readD(g), gt.read(a), gt.read(bv))
	last()
}

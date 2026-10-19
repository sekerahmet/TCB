package tensor

import (
	"fmt"
	"math"
)

// CPU arka ucu: genişletilmiş eleman bazlı işlemler. SiLU sigmoid gibi vektörel üstelle
// (expVek) hesaplanır; diğer tekli işlemler float64 kitaplık işlevleriyle hesaplanıp
// float32'ye bir kez yuvarlanır (başvuru doğruluğu). 16 bitlik türler tür uyarlayıcılarıyla
// (gecici) float32'ye açılır.

// unaryDonguGenis unaryDongu'nun genişletilmiş işlemleri: y = f(x).
func unaryDonguGenis(op UnOp, y, x []float32) {
	f64 := func(f func(float64) float64) {
		for i, v := range x {
			y[i] = float32(f(float64(v)))
		}
	}
	switch op {
	case OpSilu:
		siluIleri(y, x)
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
			y[i] = isaret32(v)
		}
	case OpAbs:
		for i, v := range x {
			y[i] = abs32(v)
		}
	default:
		panic(fmt.Sprintf("tensor: bilinmeyen tekli işlem %d", op))
	}
}

// isaret32 x > 0 ise 1, x < 0 ise −1, sıfırda (işaretiyle) sıfır, NaN'da NaN.
func isaret32(x float32) float32 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return x
}

// siluIleri y = x·σ(x); σ sigmoid'deki gibi e = e^{-|x|} ile sayısal kararlı biçimde
// (x ≥ 0 için 1/(1+e), aksi halde e/(1+e)) hesaplanır. x < −87 civarında σ(x) normal
// altı bölgededir: sonucun göreli hatası büyür, mutlak hatası 1e-38'in altında kalır.
func siluIleri(y, x []float32) {
	var t [vekBlok]float32
	for i0 := 0; i0 < len(x); i0 += vekBlok {
		xs := x[i0:min(i0+vekBlok, len(x))]
		tb := t[:len(xs)]
		for i, v := range xs {
			tb[i] = -abs32(v)
		}
		expVek(tb, tb)
		ys := y[i0 : i0+len(xs)]
		for i, v := range xs {
			e := tb[i]
			s := e / (1 + e)
			if v >= 0 {
				s = 1 / (1 + e)
			}
			if s == 0 {
				v = 0 // x -> -sonsuz: x·σ(x) -> 0 (-sonsuz·0 NaN üretirdi)
			}
			ys[i] = v * s
		}
	}
}

// siluGeri gx += g·d, d = s + x·s·(1−s) (s = σ(x)). s·(1−s) = 0 iken (|x| büyük ya da
// sonsuz) d = s alınır: x·0 sonsuzda NaN üretirdi.
func siluGeri(gx, g, x []float32) {
	var t [vekBlok]float32
	for i0 := 0; i0 < len(x); i0 += vekBlok {
		xs := x[i0:min(i0+vekBlok, len(x))]
		tb := t[:len(xs)]
		for i, v := range xs {
			tb[i] = -abs32(v)
		}
		expVek(tb, tb)
		gs, gxs := g[i0:i0+len(xs)], gx[i0:i0+len(xs)]
		for i, v := range xs {
			e := tb[i]
			s := e / (1 + e)
			if v >= 0 {
				s = 1 / (1 + e)
			}
			d := s
			if q := s * (1 - s); q != 0 {
				d = s + v*q
			}
			gxs[i] += gs[i] * d
		}
	}
}

// unaryGradDonguGenis unaryGradDongu'nun genişletilmiş işlemleri: gx += g·f'(x, y).
// Türev çarpanları float64'te hesaplanır.
func unaryGradDonguGenis(op UnOp, gx, g, x, y []float32) {
	f64 := func(f func(x, y float64) float64) {
		for i, v := range g {
			gx[i] += v * float32(f(float64(x[i]), float64(y[i])))
		}
	}
	switch op {
	case OpSilu:
		siluGeri(gx, g, x)
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
		f64(func(_, y float64) float64 { return y + 1 })
	case OpAbs:
		// Türev işaret(x): 0'da (±0) çarpan sıfırdır, NaN'da NaN.
		for i, v := range g {
			gx[i] += v * isaret32(x[i])
		}
	case OpFloor, OpCeil, OpSign:
		// Türev 0 (tensör katmanı bu işlemlerin geri yayılımını arka uca göndermez).
	default:
		panic(fmt.Sprintf("tensor: bilinmeyen tekli işlem %d", op))
	}
}

// ----------------------------------------------------------------------------
// İkili: iki argümanlı arktanjant ve karşılaştırmalar
// ----------------------------------------------------------------------------

// binDeger tek elemanlı ikili işlem (genişletilmiş işlemler).
func binDeger(op BinOp, a, b float32) float32 {
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
		panic(fmt.Sprintf("tensor: bilinmeyen ikili işlem %d", op))
	}
	if d {
		return 1
	}
	return 0
}

// binFlatGenis binFlat'in genişletilmiş işlemleri.
func binFlatGenis(op BinOp, out, a, b []float32) {
	for i, x := range a {
		out[i] = binDeger(op, x, b[i])
	}
}

// binStridedGenis binStrided'ın genişletilmiş işlemleri.
func binStridedGenis(op BinOp, out []float32, oo, os int, a []float32, ao, as int, b []float32, bo, bs int, n int) {
	for i := 0; i < n; i++ {
		out[oo+i*os] = binDeger(op, a[ao+i*as], b[bo+i*bs])
	}
}

// ----------------------------------------------------------------------------
// Sınırlama, koşullu seçim, arktanjant gradyanı
// ----------------------------------------------------------------------------

// sinirla32 x'i [lo, hi]'ye sınırlar; NaN korunur.
func sinirla32(x, lo, hi float32) float32 {
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
			y[i] = sinirla32(xd[i], lo, hi)
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
	sc := broadcastStrides(c.Strides, c.Shape, outShape)
	sa := broadcastStrides(a.Strides, a.Shape, outShape)
	sb := broadcastStrides(b.Strides, b.Shape, outShape)
	iterRowsPar(outShape, []int{c.Offset, a.Offset, b.Offset, 0}, [][]int{sc, sa, sb, contiguousStrides(outShape)}, 3,
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

func (cpuBackend) f32WhereGrad(dst Storage, dstShape, outShape []int, g Storage, c View, sec bool) {
	dg, gd, cdat := cd(dst), cd(g), cd(c.S)
	sc := broadcastStrides(c.Strides, c.Shape, outShape)
	sd := broadcastStrides(contiguousStrides(dstShape), dstShape, outShape)
	// Hedefin indirgenen (adım 0) eksenleri bölünmez: her hedef elemanı tek görevde ve
	// sabit sırayla biriktirilir (belirlenimci).
	iterRowsPar(outShape, []int{0, 0, c.Offset}, [][]int{contiguousStrides(outShape), sd, sc}, 1,
		func(off []int, m int, in []int) {
			for i := 0; i < m; i++ {
				if (cdat[off[2]+i*in[2]] != 0) == sec {
					dg[off[1]+i*in[1]] += gd[off[0]+i*in[0]]
				}
			}
		})
}

// f32Atan2Grad önce çıktı şeklinde iki terimi (float64 ile) hesaplar, sonra her birini
// girdinin şekline indirgeyerek ekler.
func (b cpuBackend) f32Atan2Grad(ga, gb Storage, aShape, bShape, outShape []int, g Storage, a, bv View) {
	n := numelOf(outShape)
	if n == 0 {
		return
	}
	gd := cd(g)
	ta := make([]float32, n)
	tb := make([]float32, n)
	sa := broadcastStrides(a.Strides, a.Shape, outShape)
	sb := broadcastStrides(bv.Strides, bv.Shape, outShape)
	ad, bd := cd(a.S), cd(bv.S)
	iterRowsPar(outShape, []int{a.Offset, bv.Offset, 0}, [][]int{sa, sb, contiguousStrides(outShape)}, 2,
		func(off []int, m int, in []int) {
			for i := 0; i < m; i++ {
				y, x := float64(ad[off[0]+i*in[0]]), float64(bd[off[1]+i*in[1]])
				o := off[2] + i*in[2]
				r := y*y + x*x
				if r == 0 {
					continue
				}
				gv := float64(gd[o])
				ta[o] = float32(gv * x / r)
				tb[o] = float32(-gv * y / r)
			}
		})
	if ga != nil {
		b.f32BinaryGrad(GradPlus, ga, aShape, outShape, &cpuStorage{d: ta, tur: Float32}, View{}, nil)
	}
	if gb != nil {
		b.f32BinaryGrad(GradPlus, gb, bShape, outShape, &cpuStorage{d: tb, tur: Float32}, View{}, nil)
	}
}

// ----------------------------------------------------------------------------
// Tür uyarlayıcıları
// ----------------------------------------------------------------------------

func (b cpuBackend) Clamp(out Storage, x View, lo, hi float32) {
	var gt gecici
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	b.f32Clamp(o, gt.oku(x), lo, hi)
	son()
}

func (b cpuBackend) ClampGrad(gx, g Storage, x View, lo, hi float32) {
	var gt gecici
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	b.f32ClampGrad(d, gt.okuD(g), gt.oku(x), lo, hi)
	son()
}

func (b cpuBackend) Where(out Storage, outShape []int, c, a, bv View) {
	var gt gecici
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	b.f32Where(o, outShape, gt.oku(c), gt.oku(a), gt.oku(bv))
	son()
}

func (b cpuBackend) WhereGrad(dst Storage, dstShape, outShape []int, g Storage, c View, sec bool) {
	var gt gecici
	defer gt.bitir()
	d, son := gt.yaz(dst, true)
	b.f32WhereGrad(d, dstShape, outShape, gt.okuD(g), gt.oku(c), sec)
	son()
}

func (b cpuBackend) Atan2Grad(ga, gb Storage, aShape, bShape, outShape []int, g Storage, a, bv View) {
	var gt gecici
	defer gt.bitir()
	da, sonA := gt.yaz(ga, true)
	db, sonB := gt.yaz(gb, true)
	b.f32Atan2Grad(da, db, aShape, bShape, outShape, gt.okuD(g), gt.oku(a), gt.oku(bv))
	sonA()
	sonB()
}

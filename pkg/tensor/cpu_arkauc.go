package tensor

import (
	"fmt"
	"math"
)

// ============================================================================
// CPU arka ucu: depolama, bellek işlemleri ve eleman bazlı çekirdekler
// ============================================================================
//
// Başvuru (referans) ve test arka ucudur; aynı zamanda çok çekirdekli ve
// önbellek dostu olacak şekilde yazılmıştır.

// cpuBackend Backend arayüzünün CPU uygulaması.
type cpuBackend struct{}

func (cpuBackend) Device() Device { return CPU }

// ----------------------------------------------------------------------------
// Adımlı gezinme
// ----------------------------------------------------------------------------

// coalesce, tüm işlenenler için bitişik olan komşu eksenleri birleştirerek
// gezinme döngüsünün boyut sayısını azaltır. Uzunluğu 1 olan eksenler atılır.
func coalesce(shape []int, strides [][]int) ([]int, [][]int) {
	ns := make([]int, 0, len(shape))
	nst := make([][]int, len(strides))
	for k := range nst {
		nst[k] = make([]int, 0, len(shape))
	}
	for d := 0; d < len(shape); d++ {
		if shape[d] == 1 {
			continue
		}
		if len(ns) > 0 {
			last := len(ns) - 1
			ok := true
			for k := range strides {
				if nst[k][last] != strides[k][d]*shape[d] {
					ok = false
					break
				}
			}
			if ok {
				ns[last] *= shape[d]
				for k := range strides {
					nst[k][last] = strides[k][d]
				}
				continue
			}
		}
		ns = append(ns, shape[d])
		for k := range strides {
			nst[k] = append(nst[k], strides[k][d])
		}
	}
	if len(ns) == 0 {
		ns = append(ns, 1)
		for k := range nst {
			nst[k] = append(nst[k], 0)
		}
	}
	return ns, nst
}

// iterRows şeklin son ekseni dışındaki tüm indeksleri gezer ve her satır için
// fn'i işlenenlerin satır başı ofsetleri, satır uzunluğu ve satır içi adımlarla
// çağırır. strides[k], k. işlenenin her eksendeki adımıdır; base[k] başlangıç
// ofsetidir. Eksenler önce birleştirilir.
func iterRows(shape []int, base []int, strides [][]int, fn func(off []int, n int, inner []int)) {
	if numelOf(shape) == 0 {
		return
	}
	sh, st := coalesce(shape, strides)
	nd := len(sh)
	nops := len(base)
	off := make([]int, nops)
	copy(off, base)
	inner := make([]int, nops)
	for k := range st {
		inner[k] = st[k][nd-1]
	}
	n := sh[nd-1]
	if nd == 1 {
		fn(off, n, inner)
		return
	}
	idx := make([]int, nd-1)
	for {
		fn(off, n, inner)
		d := nd - 2
		for ; d >= 0; d-- {
			idx[d]++
			for k := 0; k < nops; k++ {
				off[k] += st[k][d]
			}
			if idx[d] < sh[d] {
				break
			}
			for k := 0; k < nops; k++ {
				off[k] -= st[k][d] * sh[d]
			}
			idx[d] = 0
		}
		if d < 0 {
			return
		}
	}
}

// iterRowsPar iterRows'un paralel biçimidir: gezinmeyi, yaz. işlenenin adımı
// sıfır olmayan bir eksende parçalara bölüp işçilere dağıtır. Yazılan işlenenin
// yayınsız eksenlerdeki eşlemesi bire bir olduğundan (bitişik, dilim ya da eksen
// sırası görünümü) farklı parçalar farklı hedeflere yazar; adımı sıfır olan
// (indirgenen) eksenler bölünmez, böylece her hedef elemanı tek bir görevde ve
// sabit sırayla biriktirilir (sonuç belirlenimcidir). fn eşzamanlı çağrılabilir.
func iterRowsPar(shape []int, base []int, strides [][]int, yaz int, fn func(off []int, n int, inner []int)) {
	n := numelOf(shape)
	if n == 0 {
		return
	}
	sh, st := coalesce(shape, strides)
	w := workers()
	if n < paralelEsik || w == 1 {
		iterRows(sh, base, st, fn)
		return
	}
	// Bölünecek eksen: yeterince parçaya ayrılabilen en dış eksen; yoksa en çok
	// parçaya ayrılabilen. Son (satır içi) eksen, önbellek satırlarını
	// paylaştırmamak için satirParca genişliğinde bloklara bölünür.
	son := len(sh) - 1
	birim := func(k int) int {
		if k == son {
			return satirParca
		}
		return 1
	}
	parca := func(k int) int { return (sh[k] + birim(k) - 1) / birim(k) }
	d := -1
	for k := range sh {
		if st[yaz][k] == 0 {
			continue
		}
		if parca(k) >= w {
			d = k
			break
		}
		if d < 0 || parca(k) > parca(d) {
			d = k
		}
	}
	if d < 0 || parca(d) < 2 {
		iterRows(sh, base, st, fn)
		return
	}
	u := birim(d)
	parallelFor(parca(d), n/sh[d]*u, func(lo, hi int) {
		lo, hi = lo*u, min(hi*u, sh[d])
		alt := copyInts(sh)
		alt[d] = hi - lo
		b := make([]int, len(base))
		for k := range base {
			b[k] = base[k] + lo*st[k][d]
		}
		iterRows(alt, b, st, fn)
	})
}

// satirParca satır içi eksen bölünürken bir parçanın en az eleman sayısı.
const satirParca = 64

// gatherCPU görünümün elemanlarını mantıksal sırayla verir. Görünüm zaten
// bitişikse depoyu kopyalamadan döndürür; dönen dilim yalnızca okunmalıdır.
func gatherCPU(v View) []float32 {
	d := cd(v.S)
	n := v.Numel()
	if v.IsContiguous() {
		return d[v.Offset : v.Offset+n]
	}
	out := make([]float32, n)
	gatherInto(out, v)
	return out
}

// gatherInto bitişik olmayan görünümün elemanlarını mantıksal sırayla out'a
// (paralel, satırlara bölerek) yazar.
func gatherInto(out []float32, v View) {
	d := cd(v.S)
	iterRowsPar(v.Shape, []int{v.Offset, 0}, [][]int{v.Strides, contiguousStrides(v.Shape)}, 1, func(off []int, m int, in []int) {
		src, dst, s := off[0], off[1], in[0]
		o := out[dst : dst+m]
		if s == 1 {
			copy(o, d[src:src+m])
			return
		}
		for i := range o {
			o[i] = d[src+i*s]
		}
	})
}

// copyPar bitişik kopyayı paralel yapar.
func copyPar(dst, src []float32) {
	src = src[:len(dst)]
	parallelFor(len(dst), 1, func(lo, hi int) { copy(dst[lo:hi], src[lo:hi]) })
}

// ----------------------------------------------------------------------------
// Gradyan biriktirme
// ----------------------------------------------------------------------------

func (cpuBackend) f32AddTo(dst, src Storage, scale float32) {
	d, s := cd(dst), cd(src)
	s = s[:len(d)]
	parallelFor(len(d), 1, func(lo, hi int) {
		if scale == 1 {
			for i := lo; i < hi; i++ {
				d[i] += s[i]
			}
			return
		}
		for i := lo; i < hi; i++ {
			d[i] += scale * s[i]
		}
	})
}

func (cpuBackend) f32ScatterAdd(dst Storage, dstOff int, dstStrides []int, src Storage, shape []int) {
	d, s := cd(dst), cd(src)
	iterRowsPar(shape, []int{0, dstOff}, [][]int{contiguousStrides(shape), dstStrides}, 1, func(off []int, n int, in []int) {
		si, di, ss, ds := off[0], off[1], in[0], in[1]
		for i := 0; i < n; i++ {
			d[di+i*ds] += s[si+i*ss]
		}
	})
}

func (cpuBackend) f32ExpandAdd(dst Storage, dstShape []int, src Storage, srcShape []int, scale float32) {
	d, s := cd(dst), cd(src)
	if numelOf(srcShape) == 1 {
		// Skaler yayın (SumAll/MeanAll geri yayılımı).
		v := scale * s[0]
		d = d[:numelOf(dstShape)]
		parallelFor(len(d), 1, func(lo, hi int) {
			p := d[lo:hi]
			for i := range p {
				p[i] += v
			}
		})
		return
	}
	sst := broadcastStrides(contiguousStrides(srcShape), srcShape, dstShape)
	iterRowsPar(dstShape, []int{0, 0}, [][]int{contiguousStrides(dstShape), sst}, 0, func(off []int, n int, in []int) {
		di, si, ds, ss := off[0], off[1], in[0], in[1]
		if ds == 1 && ss == 0 {
			// Satır yayını (ör. eksen toplamının geri yayılımı): tek değer satıra eklenir.
			v := scale * s[si]
			row := d[di : di+n]
			for i := range row {
				row[i] += v
			}
			return
		}
		for i := 0; i < n; i++ {
			d[di+i*ds] += scale * s[si+i*ss]
		}
	})
}

// ----------------------------------------------------------------------------
// İkili işlemler
// ----------------------------------------------------------------------------

// binFlat eşit uzunluklu bitişik dilimlerde işlem yapar.
func binFlat(op BinOp, out, a, b []float32) {
	b = b[:len(a)]
	out = out[:len(a)]
	switch op {
	case OpAdd:
		for i, x := range a {
			out[i] = x + b[i]
		}
	case OpSub:
		for i, x := range a {
			out[i] = x - b[i]
		}
	case OpMul:
		for i, x := range a {
			out[i] = x * b[i]
		}
	case OpDiv:
		for i, x := range a {
			out[i] = x / b[i]
		}
	}
}

// binStrided adımlı genel çekirdek.
func binStrided(op BinOp, out []float32, oo, os int, a []float32, ao, as int, b []float32, bo, bs int, n int) {
	switch op {
	case OpAdd:
		for i := 0; i < n; i++ {
			out[oo+i*os] = a[ao+i*as] + b[bo+i*bs]
		}
	case OpSub:
		for i := 0; i < n; i++ {
			out[oo+i*os] = a[ao+i*as] - b[bo+i*bs]
		}
	case OpMul:
		for i := 0; i < n; i++ {
			out[oo+i*os] = a[ao+i*as] * b[bo+i*bs]
		}
	case OpDiv:
		for i := 0; i < n; i++ {
			out[oo+i*os] = a[ao+i*as] / b[bo+i*bs]
		}
	}
}

func (cpuBackend) f32Binary(op BinOp, out Storage, outShape []int, a, b View) {
	od := cd(out)
	n := numelOf(outShape)
	if n == 0 {
		return
	}
	ac, bc := a.IsContiguous(), b.IsContiguous()
	switch {
	case ac && bc && sameShape(a.Shape, outShape) && sameShape(b.Shape, outShape):
		ad, bd := cd(a.S)[a.Offset:a.Offset+n], cd(b.S)[b.Offset:b.Offset+n]
		parallelFor(n, 1, func(lo, hi int) { binFlat(op, od[lo:hi], ad[lo:hi], bd[lo:hi]) })
	case ac && bc && sameShape(a.Shape, outShape) && isSuffixShape(b.Shape, outShape) && b.Numel() >= 8:
		// Satır yayını (ör. sapma toplama): b her satıra uygulanır.
		m := b.Numel()
		ad, bd := cd(a.S)[a.Offset:a.Offset+n], cd(b.S)[b.Offset:b.Offset+m]
		parallelFor(n/m, m, func(lo, hi int) {
			for r := lo; r < hi; r++ {
				binFlat(op, od[r*m:(r+1)*m], ad[r*m:(r+1)*m], bd)
			}
		})
	default:
		ad, bd := cd(a.S), cd(b.S)
		sa := broadcastStrides(a.Strides, a.Shape, outShape)
		sb := broadcastStrides(b.Strides, b.Shape, outShape)
		iterRowsPar(outShape, []int{a.Offset, b.Offset, 0}, [][]int{sa, sb, contiguousStrides(outShape)}, 2, func(off []int, m int, in []int) {
			binStrided(op, od, off[2], in[2], ad, off[0], in[0], bd, off[1], in[1], m)
		})
	}
}

func (cpuBackend) f32BinaryGrad(mode GradMode, dst Storage, dstShape, outShape []int, g Storage, o View, y Storage) {
	dg, gd, yd := cd(dst), cd(g), cd(y)
	n := numelOf(outShape)
	var od []float32
	if o.S != nil {
		od = cd(o.S)
	}
	// Hızlı yol: tüm işlenenler aynı şekilde ve bitişik.
	if sameShape(dstShape, outShape) && (o.S == nil || (o.IsContiguous() && sameShape(o.Shape, outShape))) {
		if od != nil {
			od = od[o.Offset : o.Offset+n]
		}
		parallelFor(n, 1, func(lo, hi int) {
			switch mode {
			case GradPlus:
				for i := lo; i < hi; i++ {
					dg[i] += gd[i]
				}
			case GradMinus:
				for i := lo; i < hi; i++ {
					dg[i] -= gd[i]
				}
			case GradMul:
				for i := lo; i < hi; i++ {
					dg[i] += gd[i] * od[i]
				}
			case GradDiv:
				for i := lo; i < hi; i++ {
					dg[i] += gd[i] / od[i]
				}
			case GradDivB:
				for i := lo; i < hi; i++ {
					dg[i] -= gd[i] * yd[i] / od[i]
				}
			}
		})
		return
	}
	// Satır indirgemesi (ör. (R, D) + (D) toplamının sapma gradyanı): satırlar
	// sabit uzunluklu parçalara bölünür, parça toplamları sabit sırayla eklenir.
	if m := numelOf(dstShape); m > 0 && isSuffixShape(dstShape, outShape) && n%m == 0 && mode <= GradMul &&
		(mode != GradMul || (o.IsContiguous() && sameShape(o.Shape, outShape))) {
		if od != nil {
			od = od[o.Offset : o.Offset+n]
		}
		satirIndirge(mode, dg[:m], gd[:n], od, n/m, m)
		return
	}
	cst := contiguousStrides(outShape)
	dst2 := broadcastStrides(contiguousStrides(dstShape), dstShape, outShape)
	ost := make([]int, len(outShape))
	ooff := 0
	if o.S != nil {
		ost = broadcastStrides(o.Strides, o.Shape, outShape)
		ooff = o.Offset
	}
	// Hedefin sıfır adımlı (indirgenen) eksenleri bölünmez: her hedef elemanı tek
	// görevde biriktirilir.
	iterRowsPar(outShape, []int{0, 0, ooff}, [][]int{cst, dst2, ost}, 1, func(off []int, m int, in []int) {
		gi, di, oi := off[0], off[1], off[2]
		gs, ds, os := in[0], in[1], in[2]
		switch mode {
		case GradPlus:
			for i := 0; i < m; i++ {
				dg[di+i*ds] += gd[gi+i*gs]
			}
		case GradMinus:
			for i := 0; i < m; i++ {
				dg[di+i*ds] -= gd[gi+i*gs]
			}
		case GradMul:
			for i := 0; i < m; i++ {
				dg[di+i*ds] += gd[gi+i*gs] * od[oi+i*os]
			}
		case GradDiv:
			for i := 0; i < m; i++ {
				dg[di+i*ds] += gd[gi+i*gs] / od[oi+i*os]
			}
		case GradDivB:
			for i := 0; i < m; i++ {
				dg[di+i*ds] -= gd[gi+i*gs] * yd[gi+i*gs] / od[oi+i*os]
			}
		}
	})
}

// satirIndirge dg (m) += Σ_r terim(g[r], o[r]) (R satır). Satırlar, uzunluğu
// yalnızca m'ye bağlı parçalara bölünür; her parçanın kısmi toplamı ayrı
// hesaplanır ve parçalar sabit sırayla birleştirilir (belirlenimci).
func satirIndirge(mode GradMode, dg, gd, od []float32, R, m int) {
	rpc := max(1, indirgemeParca/m)
	nc := (R + rpc - 1) / rpc
	kismi := make([]float32, nc*m)
	parallelFor(nc, rpc*m, func(lo, hi int) {
		for c := lo; c < hi; c++ {
			acc := kismi[c*m : (c+1)*m]
			for r := c * rpc; r < min((c+1)*rpc, R); r++ {
				g := gd[r*m : (r+1)*m]
				switch mode {
				case GradPlus, GradMinus:
					for j, v := range g {
						acc[j] += v
					}
				case GradMul:
					o := od[r*m : (r+1)*m]
					for j, v := range g {
						acc[j] += v * o[j]
					}
				}
			}
		}
	})
	isaret := float32(1)
	if mode == GradMinus {
		isaret = -1
	}
	parallelFor(m, nc, func(lo, hi int) {
		for j := lo; j < hi; j++ {
			var s float32
			for c := 0; c < nc; c++ {
				s += kismi[c*m+j]
			}
			dg[j] += isaret * s
		}
	})
}

// ----------------------------------------------------------------------------
// Tekli işlemler
// ----------------------------------------------------------------------------
// GELU sabitleri (tanh yaklaşımı). GELU, 0.5·(1 + tanh(u)) = σ(2u) özdeşliğiyle
// tek bir üstelle hesaplanır: ileri x / (1 + e^{-2u}), geri s = σ(2u) ile
// s + 2·x·s·(1-s)·u' (0.5(1+tanh) = s, 0.5(1-tanh²) = 2s(1-s)).
const (
	geluC = 0.7978845608028654 // sqrt(2/pi)
	geluA = 0.044715
)

// vekBlok vektörel üstel için blok uzunluğu (yığında, L1'de kalır).
const vekBlok = 256

func abs32(x float32) float32 { return math.Float32frombits(math.Float32bits(x) &^ (1 << 31)) }

// unaryMaliyet bir elemanın yaklaşık göreli maliyeti (işçi sayısı seçimi için).
func unaryMaliyet(op UnOp) int {
	switch op {
	case OpAddScalar, OpMulScalar, OpRSubScalar, OpRelu:
		return 1
	case OpRDivScalar, OpSqrt:
		return 4
	}
	return 16
}

// unaryDongu y = f(x). İşlem seçimi döngünün dışındadır: her işlemin kendi sıkı
// döngüsü vardır, eleman başına dolaylı çağrı yapılmaz.
func unaryDongu(op UnOp, p float32, y, x []float32) {
	y = y[:len(x)]
	switch op {
	case OpExp:
		expVek(y, x)
	case OpLog:
		for i, v := range x {
			y[i] = log32(v)
		}
	case OpSqrt:
		for i, v := range x {
			y[i] = sqrt32(v)
		}
	case OpPow:
		switch p {
		case 1:
			copy(y, x)
		case 2:
			for i, v := range x {
				y[i] = v * v
			}
		default:
			pd := float64(p)
			for i, v := range x {
				y[i] = float32(math.Pow(float64(v), pd))
			}
		}
	case OpTanh:
		for i, v := range x {
			y[i] = tanh32(v)
		}
	case OpRelu:
		for i, v := range x {
			if v > 0 {
				y[i] = v
			} else {
				y[i] = 0
			}
		}
	case OpGelu:
		// x / (1 + e^{-2u}); üsteller bloklar halinde vektörel.
		var t [vekBlok]float32
		for i0 := 0; i0 < len(x); i0 += vekBlok {
			xs := x[i0:min(i0+vekBlok, len(x))]
			tb := t[:len(xs)]
			for i, v := range xs {
				tb[i] = -2 * geluC * (v + geluA*v*v*v)
			}
			expVek(tb, tb)
			ys := y[i0 : i0+len(xs)]
			for i, v := range xs {
				ys[i] = v / (1 + tb[i])
			}
		}
	case OpSigmoid:
		// e = e^{-|x|}; x ≥ 0 için 1/(1+e), aksi halde e/(1+e) (sayısal kararlı).
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
				if v >= 0 {
					ys[i] = 1 / (1 + e)
				} else {
					ys[i] = e / (1 + e)
				}
			}
		}
	case OpAddScalar:
		for i, v := range x {
			y[i] = v + p
		}
	case OpMulScalar:
		for i, v := range x {
			y[i] = v * p
		}
	case OpRSubScalar:
		for i, v := range x {
			y[i] = p - v
		}
	case OpRDivScalar:
		for i, v := range x {
			y[i] = p / v
		}
	case OpSin:
		for i, v := range x {
			y[i] = sinf32(v)
		}
	case OpCos:
		for i, v := range x {
			y[i] = cosf32(v)
		}
	default:
		panic(fmt.Sprintf("tensor: bilinmeyen tekli işlem %d", op))
	}
}

// unaryGradDongu gx += g·f'(x, y); işlem seçimi döngünün dışındadır.
func unaryGradDongu(op UnOp, p float32, gx, g, x, y []float32) {
	g = g[:len(gx)]
	x = x[:len(gx)]
	y = y[:len(gx)]
	switch op {
	case OpExp:
		for i, v := range g {
			gx[i] += v * y[i]
		}
	case OpLog:
		for i, v := range g {
			gx[i] += v * (1 / x[i])
		}
	case OpSqrt:
		for i, v := range g {
			gx[i] += v * (0.5 / y[i])
		}
	case OpPow:
		switch p {
		case 1:
			for i, v := range g {
				gx[i] += v
			}
		case 2:
			for i, v := range g {
				gx[i] += v * (2 * x[i])
			}
		default:
			pd := float64(p)
			for i, v := range g {
				gx[i] += v * (p * float32(math.Pow(float64(x[i]), pd-1)))
			}
		}
	case OpTanh:
		for i, v := range g {
			gx[i] += v * (1 - y[i]*y[i])
		}
	case OpRelu:
		for i, v := range g {
			if x[i] > 0 {
				gx[i] += v
			} else {
				gx[i] += v * 0 // NaN/sonsuz gradyan yayılımı önceki davranışla aynı
			}
		}
	case OpGelu:
		// s = σ(2u): s + 2·x·s·(1-s)·u'; üsteller bloklar halinde vektörel.
		var t [vekBlok]float32
		for i0 := 0; i0 < len(g); i0 += vekBlok {
			xs := x[i0:min(i0+vekBlok, len(g))]
			tb := t[:len(xs)]
			for i, v := range xs {
				tb[i] = -2 * geluC * (v + geluA*v*v*v)
			}
			expVek(tb, tb)
			gs, gxs := g[i0:i0+len(xs)], gx[i0:i0+len(xs)]
			for i, v := range xs {
				s := 1 / (1 + tb[i])
				du := geluC * (1 + 3*geluA*v*v)
				gxs[i] += gs[i] * (s + 2*v*s*(1-s)*du)
			}
		}
	case OpSigmoid:
		for i, v := range g {
			gx[i] += v * (y[i] * (1 - y[i]))
		}
	case OpAddScalar:
		for i, v := range g {
			gx[i] += v
		}
	case OpMulScalar:
		for i, v := range g {
			gx[i] += v * p
		}
	case OpRSubScalar:
		for i, v := range g {
			gx[i] -= v
		}
	case OpRDivScalar:
		for i, v := range g {
			gx[i] += v * (-y[i] / x[i])
		}
	case OpSin:
		for i, v := range g {
			gx[i] += v * cosf32(x[i])
		}
	case OpCos:
		for i, v := range g {
			gx[i] -= v * sinf32(x[i])
		}
	default:
		panic(fmt.Sprintf("tensor: bilinmeyen tekli işlem %d", op))
	}
}

func (cpuBackend) f32Unary(op UnOp, p float32, out Storage, x View) {
	xd := gatherCPU(x)
	y := cd(out)
	parallelFor(len(xd), unaryMaliyet(op), func(lo, hi int) {
		unaryDongu(op, p, y[lo:hi], xd[lo:hi])
	})
}

func (cpuBackend) f32UnaryGrad(op UnOp, p float32, gx, g Storage, x View, y Storage) {
	xd := gatherCPU(x)
	pg, gd, yd := cd(gx), cd(g), cd(y)
	parallelFor(len(xd), unaryMaliyet(op), func(lo, hi int) {
		unaryGradDongu(op, p, pg[lo:hi], gd[lo:hi], xd[lo:hi], yd[lo:hi])
	})
}

// float32 yardımcıları
func exp32(x float32) float32  { return expf32(x) }
func log32(x float32) float32  { return float32(math.Log(float64(x))) }
func sqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }
func tanh32(x float32) float32 { return tanhf32(x) }

package tensor

import "math"

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
func iterRowsPar(shape []int, base []int, strides [][]int, write int, fn func(off []int, n int, inner []int)) {
	n := numelOf(shape)
	if n == 0 {
		return
	}
	sh, st := coalesce(shape, strides)
	w := workers()
	if n < parallelThreshold || w == 1 {
		iterRows(sh, base, st, fn)
		return
	}
	// Bölünecek eksen: yeterince parçaya ayrılabilen en dış eksen; yoksa en çok
	// parçaya ayrılabilen. Son (satır içi) eksen, önbellek satırlarını
	// paylaştırmamak için satirParca genişliğinde bloklara bölünür.
	last := len(sh) - 1
	unit := func(k int) int {
		if k == last {
			return rowPart
		}
		return 1
	}
	part := func(k int) int { return (sh[k] + unit(k) - 1) / unit(k) }
	d := -1
	for k := range sh {
		if st[write][k] == 0 {
			continue
		}
		if part(k) >= w {
			d = k
			break
		}
		if d < 0 || part(k) > part(d) {
			d = k
		}
	}
	if d < 0 || part(d) < 2 {
		iterRows(sh, base, st, fn)
		return
	}
	u := unit(d)
	parallelFor(part(d), n/sh[d]*u, func(lo, hi int) {
		lo, hi = lo*u, min(hi*u, sh[d])
		sub := copyInts(sh)
		sub[d] = hi - lo
		b := make([]int, len(base))
		for k := range base {
			b[k] = base[k] + lo*st[k][d]
		}
		iterRows(sub, b, st, fn)
	})
}

// rowPart satır içi eksen bölünürken bir parçanın en az eleman sayısı.
const rowPart = 64

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
	default:
		binFlatWide(op, out, a, b)
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
	default:
		binStridedWide(op, out, oo, os, a, ao, as, b, bo, bs, n)
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
		rowReduce(mode, dg[:m], gd[:n], od, n/m, m)
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

// rowReduce dg (m) += Σ_r terim(g[r], o[r]) (R satır). Satırlar, uzunluğu
// yalnızca m'ye bağlı parçalara bölünür; her parçanın kısmi toplamı ayrı
// hesaplanır ve parçalar sabit sırayla birleştirilir (belirlenimci).
func rowReduce(mode GradMode, dg, gd, od []float32, R, m int) {
	rpc := max(1, reducePart/m)
	nc := (R + rpc - 1) / rpc
	partial := make([]float32, nc*m)
	parallelFor(nc, rpc*m, func(lo, hi int) {
		for c := lo; c < hi; c++ {
			acc := partial[c*m : (c+1)*m]
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
	sign := float32(1)
	if mode == GradMinus {
		sign = -1
	}
	parallelFor(m, nc, func(lo, hi int) {
		for j := lo; j < hi; j++ {
			var s float32
			for c := 0; c < nc; c++ {
				s += partial[c*m+j]
			}
			dg[j] += sign * s
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

	geluM2C float32 = -2 * geluC // e^{-2u} üssünün katsayısı
	gelu3A  float32 = 3 * geluA  // u'/c = 1 + 3a·x²
)

// GELU'nun vektör çekirdekleri (geluForwardVec, geluBackwardVec) AVX2'li işlemcide AVX2,
// aksi halde aşağıdaki Go eşleridir; işlem sırası aynıdır, yalnızca üstel
// (expVec ile expf32 gibi) 1-2 ulp farklı olabilir.

// geluForwardGo y = x / (1 + e^{-2u}), u = c·(x + a·x³). Pay −∞'da en küçük sonlu sayıdır:
// −∞/∞ NaN yerine −0 (limit) çıkar; NaN korunur.
func geluForwardGo(y, x []float32) {
	y = y[:len(x)]
	for i, v := range x {
		x3 := v * v * v
		u := float32(x3*geluA) + v
		y[i] = max(v, -math.MaxFloat32) / (1 + expf32(u*geluM2C))
	}
}

// geluBackwardGo gx += g·d; s = σ(2u) = 1/(1+e^{-2u}), q = s(1-s) ile
// d = s + 2x·q·u' (u' = c·(1 + 3a·x²)). q = 0 iken (|x| büyük) ikinci terim
// sıfırdır; |x| ≳ 1.8e19'da x² taşar ve 0·sonsuz NaN üretirdi, bu yüzden d = s.
func geluBackwardGo(gx, g, x []float32) {
	g = g[:len(gx)]
	x = x[:len(gx)]
	for i, v := range x {
		x2 := v * v
		u := float32(x2*v*geluA) + v
		s := 1 / (1 + expf32(u*geluM2C))
		q := s * (1 - s)
		d := s
		if q != 0 {
			du := float32(float32(x2*gelu3A)+1) * geluC
			d = float32(float32((v+v)*q)*du) + s
		}
		gx[i] += float32(g[i] * d)
	}
}

// vecBlock vektörel üstel için blok uzunluğu (yığında, L1'de kalır).
const vecBlock = 256

func abs32(x float32) float32 { return math.Float32frombits(math.Float32bits(x) &^ (1 << 31)) }

// unaryCost bir elemanın yaklaşık göreli maliyeti (işçi sayısı seçimi için).
func unaryCost(op UnOp) int {
	switch op {
	case OpAddScalar, OpMulScalar, OpRSubScalar, OpRelu, OpAbs:
		return 1
	case OpRDivScalar, OpSqrt:
		return 4
	}
	return 16
}

// unaryLoop y = f(x). İşlem seçimi döngünün dışındadır: her işlemin kendi sıkı
// döngüsü vardır, eleman başına dolaylı çağrı yapılmaz.
func unaryLoop(op UnOp, p float32, y, x []float32) {
	y = y[:len(x)]
	switch op {
	case OpExp:
		expVec(y, x)
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
			y[i] = tanhf32(v)
		}
	case OpRelu:
		for i, v := range x {
			if v > 0 || v != v { // NaN yayılır
				y[i] = v
			} else {
				y[i] = 0
			}
		}
	case OpGelu:
		geluForwardVec(y, x)
	case OpSigmoid:
		for i0 := 0; i0 < len(x); i0 += vecBlock {
			xs := x[i0:min(i0+vecBlock, len(x))]
			sigmoidBlock(y[i0:i0+len(xs)], xs)
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
		unaryLoopWide(op, y, x) // cpu_eleman_genis.go
	}
}

// unaryGradLoop gx += g·f'(x, y); işlem seçimi döngünün dışındadır.
func unaryGradLoop(op UnOp, p float32, gx, g, x, y []float32) {
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
		case 0:
			// x^0 = 1 sabittir: gradyan 0 (x = 0'da 0·x^-1 = 0·sonsuz NaN üretirdi).
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
		geluBackwardVec(gx, g, x)
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
		unaryGradLoopWide(op, gx, g, x, y) // cpu_eleman_genis.go
	}
}

func (cpuBackend) f32Unary(op UnOp, p float32, out Storage, x View) {
	xd := gatherCPU(x)
	y := cd(out)
	parallelFor(len(xd), unaryCost(op), func(lo, hi int) {
		unaryLoop(op, p, y[lo:hi], xd[lo:hi])
	})
}

func (cpuBackend) f32UnaryGrad(op UnOp, p float32, gx, g Storage, x View, y Storage) {
	xd := gatherCPU(x)
	pg, gd, yd := cd(gx), cd(g), cd(y)
	parallelFor(len(xd), unaryCost(op), func(lo, hi int) {
		unaryGradLoop(op, p, pg[lo:hi], gd[lo:hi], xd[lo:hi], yd[lo:hi])
	})
}

// float32 yardımcıları
func log32(x float32) float32  { return float32(math.Log(float64(x))) }
func sqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }

// Compile-time checks: optional backend interfaces are found by type assertion, so a renamed
// method would otherwise drop a fast path silently.
var (
	_ Backend        = cpuBackend{}
	_ Releaser       = cpuBackend{}
	_ AttentionTiler = cpuBackend{}
	_ Storage        = (*cpuStorage)(nil)
)

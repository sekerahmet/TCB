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

// cpuStorage ana bellekte düz float32 dilimi.
type cpuStorage struct{ d []float32 }

func (s *cpuStorage) Device() Device { return CPU }
func (s *cpuStorage) Len() int       { return len(s.d) }

// cd depolamanın CPU dilimini verir (nil için nil).
func cd(s Storage) []float32 {
	if s == nil {
		return nil
	}
	c, ok := s.(*cpuStorage)
	if !ok {
		panic(fmt.Sprintf("tensor: CPU arka ucuna %s cihazından depolama verildi", s.Device()))
	}
	return c.d
}

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
// Bellek
// ----------------------------------------------------------------------------

func (cpuBackend) Alloc(n int) Storage { return &cpuStorage{d: make([]float32, n)} }

func (cpuBackend) FromHost(data []float32) Storage {
	d := make([]float32, len(data))
	copy(d, data)
	return &cpuStorage{d: d}
}

func (cpuBackend) ToHost(src View) []float32 {
	out := make([]float32, src.Numel())
	copyTo(out, src)
	return out
}

func (cpuBackend) Copy(dst Storage, dstOff int, src View) {
	copyTo(cd(dst)[dstOff:dstOff+src.Numel()], src)
}

// copyTo görünümü geçici dizi kullanmadan doğrudan hedefe yazar.
func copyTo(out []float32, src View) {
	if src.IsContiguous() {
		copyPar(out, cd(src.S)[src.Offset:src.Offset+len(out)])
		return
	}
	gatherInto(out, src)
}

func (cpuBackend) Fill(dst Storage, v float32) {
	d := cd(dst)
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
// Gradyan biriktirme
// ----------------------------------------------------------------------------

func (cpuBackend) AddTo(dst, src Storage, scale float32) {
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

func (cpuBackend) ScatterAdd(dst Storage, dstOff int, dstStrides []int, src Storage, shape []int) {
	d, s := cd(dst), cd(src)
	iterRowsPar(shape, []int{0, dstOff}, [][]int{contiguousStrides(shape), dstStrides}, 1, func(off []int, n int, in []int) {
		si, di, ss, ds := off[0], off[1], in[0], in[1]
		for i := 0; i < n; i++ {
			d[di+i*ds] += s[si+i*ss]
		}
	})
}

func (cpuBackend) ExpandAdd(dst Storage, dstShape []int, src Storage, srcShape []int, scale float32) {
	d, s := cd(dst), cd(src)
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

func (cpuBackend) Binary(op BinOp, out Storage, outShape []int, a, b View) {
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

func (cpuBackend) BinaryGrad(mode GradMode, dst Storage, dstShape, outShape []int, g Storage, o View, y Storage) {
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

// GELU sabitleri (tanh yaklaşımı).
const (
	geluC = 0.7978845608028654 // sqrt(2/pi)
	geluA = 0.044715
)

// sigmoid32 sayısal kararlı lojistik işlev.
func sigmoid32(x float32) float32 {
	if x >= 0 {
		return float32(1 / (1 + math.Exp(-float64(x))))
	}
	e := math.Exp(float64(x))
	return float32(e / (1 + e))
}

// unaryFns tekli işlemin ileri işlevini ve yerel türevini (x, y'ye göre) verir.
func unaryFns(op UnOp, p float32) (func(x float32) float32, func(x, y float32) float32) {
	switch op {
	case OpExp:
		return exp32, func(x, y float32) float32 { return y }
	case OpLog:
		return log32, func(x, y float32) float32 { return 1 / x }
	case OpSqrt:
		return sqrt32, func(x, y float32) float32 { return 0.5 / y }
	case OpPow:
		switch p {
		case 1:
			return func(x float32) float32 { return x }, func(x, y float32) float32 { return 1 }
		case 2:
			return func(x float32) float32 { return x * x }, func(x, y float32) float32 { return 2 * x }
		}
		pd := float64(p)
		return func(x float32) float32 { return float32(math.Pow(float64(x), pd)) },
			func(x, y float32) float32 { return p * float32(math.Pow(float64(x), pd-1)) }
	case OpTanh:
		return tanh32, func(x, y float32) float32 { return 1 - y*y }
	case OpRelu:
		return func(x float32) float32 {
				if x > 0 {
					return x
				}
				return 0
			}, func(x, y float32) float32 {
				if x > 0 {
					return 1
				}
				return 0
			}
	case OpGelu:
		return func(x float32) float32 {
				xd := float64(x)
				u := geluC * (xd + geluA*xd*xd*xd)
				return float32(0.5 * xd * (1 + math.Tanh(u)))
			}, func(x, y float32) float32 {
				xd := float64(x)
				u := geluC * (xd + geluA*xd*xd*xd)
				th := math.Tanh(u)
				du := geluC * (1 + 3*geluA*xd*xd)
				return float32(0.5*(1+th) + 0.5*xd*(1-th*th)*du)
			}
	case OpSigmoid:
		return sigmoid32, func(x, y float32) float32 { return y * (1 - y) }
	case OpAddScalar:
		return func(x float32) float32 { return x + p }, func(x, y float32) float32 { return 1 }
	case OpMulScalar:
		return func(x float32) float32 { return x * p }, func(x, y float32) float32 { return p }
	case OpRSubScalar:
		return func(x float32) float32 { return p - x }, func(x, y float32) float32 { return -1 }
	case OpRDivScalar:
		return func(x float32) float32 { return p / x }, func(x, y float32) float32 { return -y / x }
	}
	panic(fmt.Sprintf("tensor: bilinmeyen tekli işlem %d", op))
}

func (cpuBackend) Unary(op UnOp, p float32, out Storage, x View) {
	f, _ := unaryFns(op, p)
	xd := gatherCPU(x)
	y := cd(out)
	parallelFor(len(xd), 4, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			y[i] = f(xd[i])
		}
	})
}

func (cpuBackend) UnaryGrad(op UnOp, p float32, gx, g Storage, x View, y Storage) {
	_, df := unaryFns(op, p)
	xd := gatherCPU(x)
	pg, gd, yd := cd(gx), cd(g), cd(y)
	parallelFor(len(xd), 4, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			pg[i] += gd[i] * df(xd[i], yd[i])
		}
	})
}

// float32 yardımcıları
func exp32(x float32) float32  { return float32(math.Exp(float64(x))) }
func log32(x float32) float32  { return float32(math.Log(float64(x))) }
func sqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }
func tanh32(x float32) float32 { return float32(math.Tanh(float64(x))) }

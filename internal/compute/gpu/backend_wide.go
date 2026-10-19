package gpu

import (
	"fmt"
	"math/bits"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: genişletilmiş eleman bazlı işlemler (sınırlama, koşullu seçim, atan2
// gradyanı), birikimli toplam ve sıralama.

func (g *backend) Clamp(out tensor.Storage, x tensor.View, lo, hi float32) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	xv := gt.read(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	checkSize(n)
	c := g.takeKernel("sinirla:false", clampKernel(false))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().write(o, 0).readV(xv).u32(n).f32(lo).f32(hi))
	last()
}

func (g *backend) ClampGrad(gx, gs tensor.Storage, x tensor.View, lo, hi float32) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(gx, true)
	gd := gt.readD(gs)
	xv := gt.read(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	checkSize(n)
	c := g.takeKernel("sinirla:true", clampKernel(true))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().write(d, 0).read(gd, 0).readV(xv).u32(n).f32(lo).f32(hi))
	last()
}

// yayinMeta görünümlerin çıktı şekline yayınlanmış adımlarını birleştirir: nd = 0 ise
// hepsi düz bitişiktir (meta boş).
func broadcastMeta(op string, outShape []int, vs ...tensor.View) (int, []int) {
	steps := make([][]int, 0, len(vs)+1)
	for _, v := range vs {
		steps = append(steps, broadcastStride(v.Strides, v.Shape, outShape))
	}
	steps = append(steps, contiguous(outShape))
	sh, st := mergeAxes(outShape, steps...)
	nd := len(sh)
	isPlain := nd == 1
	for _, s := range st {
		isPlain = isPlain && s[0] == 1
	}
	if isPlain {
		return 0, nil
	}
	if nd > maxAxis {
		none(fmt.Sprintf("%d eksenli yayınlamalı %s (en çok %d)", nd, op, maxAxis))
	}
	meta := append([]int{}, sh...)
	for _, s := range st[:len(vs)] {
		meta = append(meta, s...)
	}
	return nd, meta
}

func (g *backend) Where(out tensor.Storage, outShape []int, c, a, b tensor.View) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	c, a, b = gt.readStrided(c), gt.readStrided(a), gt.readStrided(b)
	n := numel(outShape)
	if n == 0 {
		return
	}
	checkSize(n)
	nd, meta := broadcastMeta("koşullu seçim", outShape, c, a, b)
	k := g.takeKernel(fmt.Sprintf("secim:%d", nd), selectKernel(nd))
	iz, bl := plain(n, sameBlock)
	g.launch(k, iz, bl, g.param().readV(c).readV(a).readV(b).write(o, 0).u32(n).arr(4*nd, meta...))
	last()
}

func (g *backend) WhereGrad(dst tensor.Storage, dstShape, outShape []int, gs tensor.Storage, c tensor.View, pick bool) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(dst, true)
	gd := gt.readD(gs)
	c = gt.readStrided(c)
	term := tAsecO
	if !pick {
		term = tAsecOdegil
	}
	g.reduce(reduceJob{a: gd, o: g.storageOf(c.S), oOf: c.Offset, stO2: broadcastStride(c.Strides, c.Shape, outShape),
		output: d, shape2: outShape, stA2: contiguous(outShape), st: broadcastStride(contiguous(dstShape), dstShape, outShape),
		term: term, add: true, scaleF: 1})
	last()
}

func (g *backend) Atan2Grad(ga, gb tensor.Storage, aShape, bShape, outShape []int, gs tensor.Storage, a, b tensor.View) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	n := numel(outShape)
	if n == 0 {
		return
	}
	checkSize(n)
	da, lastA := gt.write(ga, true)
	db, lastB := gt.write(gb, true)
	gd := gt.readD(gs)
	a, b = gt.readStrided(a), gt.readStrided(b)
	ta, tb := gt.buffer(n, false), gt.buffer(n, false)
	nd, meta := broadcastMeta("atan2 gradyanı", outShape, a, b)
	k := g.takeKernel(fmt.Sprintf("atan2_turev:%d", nd), atan2GradKernel(nd))
	iz, bl := plain(n, sameBlock)
	g.launch(k, iz, bl, g.param().readV(a).readV(b).read(gd, 0).write(ta, 0).write(tb, 0).u32(n).arr(3*nd, meta...))
	for _, h := range []struct {
		d  *storage
		t  *storage
		sh []int
	}{{da, ta, aShape}, {db, tb, bShape}} {
		if h.d == nil {
			continue
		}
		g.reduce(reduceJob{a: h.t, output: h.d, shape2: outShape, stA2: contiguous(outShape),
			st: broadcastStride(contiguous(h.sh), h.sh, outShape), term: tA, add: true, scaleF: 1})
	}
	lastA()
	lastB()
}

// ----------------------------------------------------------------------------
// Birikimli toplam
// ----------------------------------------------------------------------------

// eksenUcu (dış, eksen, iç) üçlüsü.
func axisEnd(shape []int, axis int) (outer, L, ic int) {
	outer, ic = 1, 1
	for _, d := range shape[:axis] {
		outer *= d
	}
	for _, d := range shape[axis+1:] {
		ic *= d
	}
	return outer, shape[axis], ic
}

// birikim satır başına bir blokla birikimli toplam başlatır.
func (g *backend) accum(y *storage, x tensor.View, shape []int, axis int, inverse, add bool) {
	outer, L, ic := axisEnd(shape, axis)
	row := outer * ic
	if row == 0 || L == 0 {
		return
	}
	checkSize(outer * L * ic)
	B := powerOfTwo((L+7)/8, 32, 256)
	C := (L + B - 1) / B
	k := g.takeKernel(fmt.Sprintf("birikim:%d:%v:%v", B, inverse, add), accumKernel(B, inverse, add))
	g.launch(k, [3]uint32{uint32(row), 1, 1}, [3]uint32{uint32(B), 1, 1},
		g.param().readV(x).write(y, 0).u32(L).u32(ic).u32(C))
}

func (g *backend) CumSum(out tensor.Storage, x tensor.View, axis int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	g.accum(o, gt.read(x), x.Shape, axis, false, false)
	last()
}

func (g *backend) CumSumGrad(gx, gs tensor.Storage, shape []int, axis int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(gx, true)
	gd := gt.readD(gs)
	g.accum(d, tensor.View{S: gd, Shape: shape, Strides: contiguous(shape)}, shape, axis, true, true)
	last()
}

// ----------------------------------------------------------------------------
// Sıralama
// ----------------------------------------------------------------------------

func (g *backend) Sort(vals, idx tensor.Storage, x tensor.View, axis, kCount int, desc bool) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	v, lastV := gt.write(vals, false)
	id := g.storageOf(idx)
	xv := gt.read(x)
	outer, L, ic := axisEnd(x.Shape, axis)
	row := outer * ic
	if row == 0 || kCount == 0 {
		return
	}
	checkSize(outer * L * ic)
	P := 1 << bits.Len(uint(L-1))
	if L <= 1 {
		P = 1
	}
	valued := v != nil
	if P <= sharedSortLimit {
		k := g.takeKernel(fmt.Sprintf("sirala_ortak:%d:%v:%v", P, desc, valued), sortSharedKernel(P, desc, valued))
		p := g.param().readV(xv).write(id, 0)
		if valued {
			p.write(v, 0)
		}
		g.launch(k, [3]uint32{uint32(row), 1, 1}, [3]uint32{uint32(max(1, min(P/2, 512))), 1, 1},
			p.u32(L).u32(ic).u32(kCount))
		lastV()
		return
	}
	g.sortGlobal(v, id, xv, row, L, ic, kCount, P, desc)
	lastV()
}

// siralaGenel P > ortakSiralamaSiniri satırları genel bellekteki anahtar tamponunda her
// bitonik adım için bir başlatmayla sıralar.
func (g *backend) sortGlobal(v, id *storage, xv tensor.View, row, L, ic, kCount, P int, desc bool) {
	total := row * P
	checkSize(total)
	lp := bits.Len(uint(P)) - 1
	buf := g.allocate(2*total, tensor.Float32, false) // 64 bitlik anahtarlar
	defer g.release(buf)
	kb := g.takeKernel(fmt.Sprintf("sirala_baslat:%v", desc), sortInitKernel(desc))
	iz, bl := plain(total, sameBlock)
	g.launch(kb, iz, bl, g.param().readV(xv).write(buf, 0).u32(total).u32(lp).u32(L).u32(ic))
	ka := g.takeKernel("sirala_adim", sortStepKernel)
	iz, bl = plain(total/2, sameBlock)
	for kk := 2; kk <= P; kk *= 2 {
		for j := kk / 2; j > 0; j /= 2 {
			g.launch(ka, iz, bl, g.param().write(buf, 0).u32(total/2).u32(lp).u32(bits.Len(uint(j))-1).u32(kk))
		}
	}
	valued := v != nil
	kc := g.takeKernel(fmt.Sprintf("sirala_cikti:%v", valued), sortOutputKernel(valued))
	iz, bl = plain(row*kCount, sameBlock)
	p := g.param().read(buf, 0).readV(xv).write(id, 0)
	if valued {
		p.write(v, 0)
	}
	g.launch(kc, iz, bl, p.u32(row*kCount).u32(lp).u32(L).u32(ic).u32(kCount))
}

func (g *backend) GatherGrad(gx, gs, idx tensor.Storage, shape []int, axis, kCount int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(gx, true)
	gd, id := gt.readD(gs), gt.readD(idx)
	outer, L, ic := axisEnd(shape, axis)
	n := outer * kCount * ic
	if n == 0 {
		return
	}
	checkSize(outer * L * ic)
	k := g.takeKernel("topla_geri", sumBackwardKernel)
	iz, bl := plain(n, sameBlock)
	g.launch(k, iz, bl, g.param().write(d, 0).read(gd, 0).read(id, 0).u32(n).u32(L).u32(ic).u32(kCount))
	last()
}

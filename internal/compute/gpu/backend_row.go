package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: softmax, çapraz entropi, normlar ve gömme.

// axisOrder şekli eksen etrafında (dış, eksen, iç) üçlüsüne ayırır.
func axisOrder(shape []int, axis int) (outer, L, ic int) {
	outer, ic = 1, 1
	for i := 0; i < axis; i++ {
		outer *= shape[i]
	}
	for i := axis + 1; i < len(shape); i++ {
		ic *= shape[i]
	}
	return outer, shape[axis], ic
}

func (g *backend) Softmax(out tensor.Storage, x tensor.View, axis int, logMode bool) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	xv := gt.read(x)
	outer, L, ic := axisOrder(x.Shape, axis)
	row := outer * ic
	if row == 0 || L == 0 {
		return
	}
	checkSize(row * L)
	B := powerOfTwo(L, 32, 256)
	c := g.takeKernel(fmt.Sprintf("softmax:%d:%v", B, logMode), softmaxKernel(B, logMode))
	g.launch(c, [3]uint32{uint32(row), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().readV(xv).write(o, 0).u32(L).u32(ic))
	last()
}

func (g *backend) SoftmaxGrad(gx, gs, y tensor.Storage, shape []int, axis int, logMode bool) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(gx, true)
	gd, yd := gt.readD(gs), gt.readD(y)
	outer, L, ic := axisOrder(shape, axis)
	row := outer * ic
	if row == 0 || L == 0 {
		return
	}
	checkSize(row * L)
	B := powerOfTwo(L, 32, 256)
	c := g.takeKernel(fmt.Sprintf("softmax_turev:%d:%v", B, logMode), softmaxGradKernel(B, logMode))
	g.launch(c, [3]uint32{uint32(row), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().write(d, 0).read(gd, 0).read(yd, 0).u32(L).u32(ic))
	last()
}

// ints []int'i 32 bitlik tam sayılara çevirir (indeksler, hedefler).
func ints(v []int) []int32 {
	r := make([]int32, len(v))
	for i, x := range v {
		if x < -1<<31 || x >= 1<<31 {
			panic(fmt.Sprintf("tensor: GPU: indeks %d 32 bite sığmıyor", x))
		}
		r[i] = int32(x)
	}
	return r
}

// yukleGecici tam sayıları geçici bir cihaz tamponuna yükler (çağrı sonunda iade).
func (t *temp) loadInt(v []int) *storage {
	d := t.g.loadInt(ints(v))
	t.t = append(t.t, d)
	return d
}

func validCount(targets []int) int {
	n := 0
	for _, t := range targets {
		if t >= 0 {
			n++
		}
	}
	return n
}

func (g *backend) CrossEntropy(x tensor.View, targets []int, C int) (tensor.Storage, tensor.Storage) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	R := len(targets)
	loss := g.allocate(1, tensor.Float32, true)
	lse := g.allocate(R, tensor.Float32, false)
	if R == 0 || C == 0 {
		return loss, lse
	}
	checkSize(R * C)
	xv := gt.readNative(x)
	xt := g.storageOf(xv.S).dtype
	target := gt.loadInt(targets)
	loss2 := gt.buffer(R, false)
	vec := ceVectorized(xt, xv, C)
	c := g.takeKernel(fmt.Sprintf("ce_ileri:%v:%v", xt, vec), ceForwardKernel(xt, vec))
	g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{ceBlock, 1, 1}, g.param().readV(xv).read(target, 0).write(lse, 0).write(loss2, 0).u32(C))
	scaleF := float32(0)
	if n := validCount(targets); n > 0 {
		scaleF = float32(1 / float64(n))
	}
	g.reduce(reduceJob{a: loss2, output: loss, shape2: []int{R}, stA2: []int{1}, st: []int{0}, term: tA, scaleF: scaleF})
	return loss, lse
}

// ceVectorized satırların 4'lü vektörlerle okunup yazılabildiği (C % 4 = 0, hizalı görünüm;
// float16'da tekil okuma).
func ceVectorized(dt tensor.DType, xv tensor.View, C int) bool {
	return C%4 == 0 && xv.Offset%4 == 0 && dt != tensor.Float16
}

func (g *backend) CrossEntropyGrad(gx, gs tensor.Storage, x tensor.View, lse tensor.Storage, targets []int, C int) {
	g.enter()
	defer g.exit()
	g.ceBackward(gx, gs, x, lse, targets, C, false)
}

// CrossEntropyGradSet (tensor.CrossEntropyGradSetter) gx'in üzerine yazar.
func (g *backend) CrossEntropyGradSet(gx, gs tensor.Storage, x tensor.View, lse tensor.Storage, targets []int, C int) {
	g.enter()
	defer g.exit()
	g.ceBackward(gx, gs, x, lse, targets, C, true)
}

func (g *backend) ceBackward(gx, gs tensor.Storage, x tensor.View, lse tensor.Storage, targets []int, C int, over bool) {
	n := validCount(targets)
	R := len(targets)
	gt := g.temp()
	defer gt.finish()
	d := g.storageOf(gx)
	if n == 0 || C == 0 {
		if over {
			g.fill(d, 0)
		}
		return
	}
	gd := gt.readD(gs)
	xv := gt.readNative(x)
	xt := g.storageOf(xv.S).dtype
	target := gt.loadInt(targets)
	ld := gt.readD(lse)
	vec := ceVectorized(xt, xv, C) && d.dtype != tensor.Float16
	c := g.takeKernel(fmt.Sprintf("ce_geri:%v:%v:%v:%v", xt, d.dtype, vec, over), ceBackwardKernel(xt, d.dtype, vec, over))
	g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{ceBlock, 1, 1},
		g.param().write(d, 0).read(gd, 0).readV(xv).read(ld, 0).read(target, 0).u32(C).f32(float32(1/float64(n))))
}

func (g *backend) Embedding(out tensor.Storage, w tensor.View, idx []int, D int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	if len(idx) == 0 || D == 0 {
		return
	}
	checkSize(len(idx) * D)
	wv := w
	if !w.IsContiguous() {
		wv = gt.read(w)
	}
	checkSize(wv.Numel())
	idxD := gt.loadInt(idx)
	dt := g.storageOf(wv.S).dtype
	c := g.takeKernel("gomme:"+dt.String(), embedKernel(dt))
	g.launch(c, [3]uint32{uint32(len(idx)), 1, 1}, [3]uint32{embedBlock, 1, 1},
		g.param().readV(wv).read(idxD, 0).write(o, 0).u32(D))
	last()
}

func (g *backend) EmbeddingGrad(gw, gs tensor.Storage, idx []int, D int) {
	g.enter()
	defer g.exit()
	if len(idx) == 0 || D == 0 {
		return
	}
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(gw, true)
	checkSize(d.n)
	gd := gt.readD(gs)
	singular, begin, pos := embedGroups(idx)
	td, bd, kd := g.loadInt(singular), g.loadInt(begin), g.loadInt(pos)
	gt.t = append(gt.t, td, bd, kd)
	c := g.takeKernel("gomme_geri", embedBackwardKernel)
	g.launch(c, [3]uint32{uint32(len(singular)), 1, 1}, [3]uint32{embedBlock, 1, 1},
		g.param().write(d, 0).read(gd, 0).read(td, 0).read(bd, 0).read(kd, 0).u32(D))
	last()
}

// normForward katman ya da RMS normunun ileri yönü.
func (g *backend) normForward(out tensor.Storage, x tensor.View, gamma, beta *tensor.View, D int, eps float32, layer bool) (*storage, *storage) {
	gt := g.temp()
	defer gt.finish()
	R := 0
	if D > 0 {
		R = x.Numel() / D
	}
	var avg *storage
	if layer {
		avg = g.allocate(R, tensor.Float32, false)
	}
	rstd := g.allocate(R, tensor.Float32, false)
	o, last := gt.write(out, false)
	if R == 0 {
		return avg, rstd
	}
	checkSize(R * D)
	xv := gt.read(x)
	gv, bv := gt.readP(gamma), gt.readP(beta)
	a := normConfig{B: powerOfTwo(D, 32, 256), layer: layer, gamma: gv != nil, beta: bv != nil}
	c := g.takeKernel(a.key("norm_ileri"), normForwardKernel(a))
	p := g.param().readV(xv)
	for _, v := range []*tensor.View{gv, bv} {
		if v != nil {
			p.readV(*v)
		} else {
			p.empty()
		}
	}
	p.write(o, 0).writeTo(avg).write(rstd, 0).u32(D).f32(eps)
	g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(a.B), 1, 1}, p)
	last()
	return avg, rstd
}

// rowSums hedef (D) += Σ_satır kaynak (R×D).
func (g *backend) rowSums(target tensor.Storage, source *storage, R, D int) {
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(target, true)
	g.reduce(reduceJob{a: source, output: d, shape2: []int{R, D}, stA2: []int{D, 1}, st: []int{0, 1}, term: tA, add: true, scaleF: 1})
	last()
}

// normBackward katman ya da RMS normunun geri yönü.
func (g *backend) normBackward(gx, ggamma, gbeta, gs tensor.Storage, x tensor.View, gamma *tensor.View, avg, rstd tensor.Storage, D int, layer bool) {
	gt := g.temp()
	defer gt.finish()
	R := 0
	if D > 0 {
		R = x.Numel() / D
	}
	if R == 0 || (gx == nil && ggamma == nil && gbeta == nil) {
		return
	}
	checkSize(R * D)
	gd := gt.readD(gs)
	if gx != nil || ggamma != nil {
		xv := gt.read(x)
		gv := gt.readP(gamma)
		dx, last := gt.write(gx, true)
		var term *storage
		if ggamma != nil {
			term = gt.buffer(R*D, false)
		}
		a := normConfig{B: powerOfTwo(D, 32, 256), layer: layer, gamma: gv != nil, xGrad: dx != nil, gammaGrad: term != nil}
		c := g.takeKernel(a.key("norm_geri"), normBackwardKernel(a))
		p := g.param().writeTo(dx).writeTo(term).read(gd, 0).readV(xv)
		if gv != nil {
			p.readV(*gv)
		} else {
			p.empty()
		}
		var od *storage
		if avg != nil {
			od = g.storageOf(avg)
		}
		p.readFrom(od).read(g.storageOf(rstd), 0).u32(D)
		g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(a.B), 1, 1}, p)
		last()
		if term != nil {
			g.rowSums(ggamma, term, R, D)
		}
	}
	if gbeta != nil {
		g.rowSums(gbeta, gd, R, D)
	}
}

func (g *backend) LayerNorm(out tensor.Storage, x tensor.View, gamma, beta *tensor.View, D int, eps float32) (tensor.Storage, tensor.Storage) {
	g.enter()
	defer g.exit()
	avg, rstd := g.normForward(out, x, gamma, beta, D, eps, true)
	return avg, rstd
}

func (g *backend) LayerNormGrad(gx, ggamma, gbeta, gs tensor.Storage, x tensor.View, gamma *tensor.View, mean, rstd tensor.Storage, D int) {
	g.enter()
	defer g.exit()
	g.normBackward(gx, ggamma, gbeta, gs, x, gamma, mean, rstd, D, true)
}

func (g *backend) RMSNorm(out tensor.Storage, x tensor.View, gamma *tensor.View, D int, eps float32) tensor.Storage {
	g.enter()
	defer g.exit()
	_, rinv := g.normForward(out, x, gamma, nil, D, eps, false)
	return rinv
}

func (g *backend) RMSNormGrad(gx, ggamma, gs tensor.Storage, x tensor.View, gamma *tensor.View, rinv tensor.Storage, D int) {
	g.enter()
	defer g.exit()
	g.normBackward(gx, ggamma, nil, gs, x, gamma, nil, rinv, D, false)
}

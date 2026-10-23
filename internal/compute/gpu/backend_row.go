package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	o := g.storageOf(out)
	xv := gt.readNative(x)
	outer, L, ic := axisOrder(x.Shape, axis)
	row := outer * ic
	if row == 0 || L == 0 {
		return
	}
	checkSize(row * L)
	B := powerOfTwo(L, 32, 256)
	xt := g.storageOf(xv.S).dtype
	c := g.takeKernel(fmt.Sprintf("softmax:%d:%v:%s:%s", B, logMode, xt, o.dtype), softmaxKernel(B, logMode, xt, o.dtype))
	g.launch(c, [3]uint32{uint32(row), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().readV(xv).write(o, 0).u32(L).u32(ic))
}

func (g *backend) SoftmaxGrad(gx, gs, y tensor.Storage, shape []int, axis int, logMode bool) {
	g.enter()
	defer g.exit()
	d, gd, yd := g.storageOf(gx), g.storageOf(gs), g.storageOf(y)
	outer, L, ic := axisOrder(shape, axis)
	row := outer * ic
	if row == 0 || L == 0 {
		return
	}
	checkSize(row * L)
	B := powerOfTwo(L, 32, 256)
	dt := [3]tensor.DType{d.dtype, gd.dtype, yd.dtype}
	c := g.takeKernel(fmt.Sprintf("softmax_turev:%d:%v:%v", B, logMode, dt), softmaxGradKernel(B, logMode, dt))
	g.launch(c, [3]uint32{uint32(row), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().write(d, 0).read(gd, 0).read(yd, 0).u32(L).u32(ic))
}

// ints []int'i 32 bitlik tam sayılara çevirir (indeksler, hedefler).
func ints(v []int) []int32 {
	r := make([]int32, len(v))
	for i, x := range v {
		if x < -1<<31 || x >= 1<<31 {
			panic(messages.New(messages.H4018, x))
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
	g.reduce(reduceJob{a: loss2, output: loss, shape2: []int{R}, stA2: []int{1}, st: []int{0}, term: tA, scaleF: scaleF, sum: true})
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
	c := g.takeKernel("gomme:"+dt.String(), embedKernel(dt, false))
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
	plan := planEmbed(embedGroups(idx))
	c := g.takeKernel("gomme_geri", embedBackwardKernel)
	// sum, satır gruplarını (tekil, baş, konum) src'den dst'ye sırayla toplar.
	sum := func(dst, src *storage, a [3][]int32) {
		n := len(a[0])
		if n == 0 {
			return
		}
		td, bd, kd := g.loadInt(a[0]), g.loadInt(a[1]), g.loadInt(a[2])
		gt.t = append(gt.t, td, bd, kd)
		g.launch(c, [3]uint32{uint32(n), 1, 1}, [3]uint32{embedBlock, 1, 1},
			g.param().write(dst, 0).read(src, 0).read(td, 0).read(bd, 0).read(kd, 0).u32(D))
	}
	sum(d, gd, plan.light)
	if plan.nChunks > 0 {
		checkSize(plan.nChunks * D)
		part := g.allocate(plan.nChunks*D, tensor.Float32, true)
		gt.t = append(gt.t, part)
		sum(part, gd, plan.chunks)
		sum(d, part, plan.heavy)
	}
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
	o := g.storageOf(out)
	if R == 0 {
		return avg, rstd
	}
	checkSize(R * D)
	gv, bv := gt.readNativeP(gamma), gt.readNativeP(beta)
	if H, S0, S1, ok := rowMap(x, D); ok && normWarpD(D) {
		a := normConfig{D: D, layer: layer, gamma: gv != nil, beta: bv != nil,
			dt: [5]tensor.DType{g.storageOf(x.S).dtype, o.dtype, g.viewType(gv), g.viewType(bv), tensor.Float32}}
		c := g.takeKernel(a.key("norm_ileri"), normForwardWarpKernel(a))
		p := normAffineOperands(g.param().readV(x), gv, bv)
		p.write(o, 0).writeTo(avg).write(rstd, 0).u32(R).u32(H).u32(S0).u32(S1).f32(eps)
		g.launch(c, [3]uint32{uint32((R + warpRows - 1) / warpRows), 1, 1}, [3]uint32{32 * warpRows, 1, 1}, p)
		return avg, rstd
	}
	xv := gt.readNative(x)
	a := normConfig{B: powerOfTwo(D, 32, 256), layer: layer, gamma: gv != nil, beta: bv != nil,
		dt: [5]tensor.DType{g.storageOf(xv.S).dtype, o.dtype, g.viewType(gv), g.viewType(bv), tensor.Float32}}
	c := g.takeKernel(a.key("norm_ileri"), normForwardKernel(a))
	p := normAffineOperands(g.param().readV(xv), gv, bv)
	p.write(o, 0).writeTo(avg).write(rstd, 0).u32(D).f32(eps)
	g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(a.B), 1, 1}, p)
	return avg, rstd
}

// normAffineOperands ileri norm çekirdeklerinin γ ve β işlenenlerini ekler (olmayan boş).
func normAffineOperands(p *paramWriter, gv, bv *tensor.View) *paramWriter {
	for _, v := range []*tensor.View{gv, bv} {
		if v != nil {
			p.readV(*v)
		} else {
			p.empty()
		}
	}
	return p
}

// rowMap x'in satırlarını (son eksen D, bitişik) iki düzeyli adımla verir: satır r'nin ilk
// elemanı (r / H)·S1 + (r % H)·S0'dadır (görünümün ofsetine göre). Ön eksenler, birbirinin
// devamı olanlar birleştirildikten sonra en çok iki kalıyorsa ok doğrudur; yoksa çağıran
// girdiyi bitişik kopyaya alır.
func rowMap(x tensor.View, D int) (H, S0, S1 int, ok bool) {
	n := len(x.Shape)
	if n == 0 || x.Shape[n-1] != D || (D > 1 && x.Strides[n-1] != 1) {
		return 0, 0, 0, false
	}
	var dims, strides []int
	for i := 0; i < n-1; i++ {
		if x.Shape[i] == 1 {
			continue
		}
		if k := len(dims) - 1; k >= 0 && strides[k] == x.Strides[i]*x.Shape[i] {
			dims[k] *= x.Shape[i]
			strides[k] = x.Strides[i]
			continue
		}
		dims, strides = append(dims, x.Shape[i]), append(strides, x.Strides[i])
	}
	switch len(dims) {
	case 0:
		return 1, 0, D, true
	case 1:
		return 1, 0, strides[0], true
	case 2:
		return dims[1], strides[1], strides[0], true
	}
	return 0, 0, 0, false
}

// rowSums hedef (D) += Σ_satır kaynak (R×D).
func (g *backend) rowSums(target tensor.Storage, source *storage, R, D int) {
	g.reduce(reduceJob{a: source, output: g.storageOf(target), shape2: []int{R, D}, stA2: []int{D, 1}, st: []int{0, 1}, term: tA, add: true, scaleF: 1})
}

// normBackwardFused γ'nın gradyanı istenen geri yön (kernel_norm.go normBackwardFusedKernel):
// P blok, her biri ardışık satır parçası; blok başına kısmi γ toplamları [P, D] sonra satırlar
// boyunca sabit sırayla indirgenir. P SM başına 2048 iş parçacığını dolduracak kadardır (ayar SM
// sayısından seçilir; aynı kartta her koşuda aynı sonuç).
func (g *backend) normBackwardFused(gt *temp, dx *storage, ggamma tensor.Storage, gd *storage, xv tensor.View, gv *tensor.View, avg, rstd tensor.Storage, R, D int, layer, set bool, dxT tensor.DType) {
	B := powerOfTwo(D, 32, 256)
	C := (D + B - 1) / B
	_, sm := g.sr.capability()
	P := min(R, max(sm, 1)*max(2048/B, 1))
	chunk := (R + P - 1) / P
	P = (R + chunk - 1) / chunk
	part := gt.buffer(P*D, false)
	a := normConfig{B: B, layer: layer, gamma: gv != nil, xGrad: dx != nil, gammaGrad: true, set: set,
		dt: [5]tensor.DType{g.storageOf(xv.S).dtype, gd.dtype, g.viewType(gv), tensor.Float32, dxT}}
	c := g.takeKernel(fmt.Sprintf("%s:%d", a.key("norm_geri_parcali"), C), normBackwardFusedKernel(a, C))
	p := g.param().writeTo(dx).write(part, 0).read(gd, 0).readV(xv)
	g.normOperands(p, gv, avg, rstd).u32(D).u32(R).u32(chunk)
	g.launch(c, [3]uint32{uint32(P), 1, 1}, [3]uint32{uint32(B), 1, 1}, p)
	g.rowSums(ggamma, part, P, D)
}

// normBackwardWarp γ'nın gradyanı istenen geri yönü warp çekirdeğiyle yürütür (normWarpGradD):
// satırlar blok parçalarına bölünür (normBackwardFused ile SM başına aynı iş parçacığı sayısı: 2048), x iki düzeyli
// adımla yerinde okunur, γ'nın kısmi toplamları [P, D] sabit sırayla toplanır.
func (g *backend) normBackwardWarp(gt *temp, gx, ggamma tensor.Storage, gd *storage, x tensor.View, gamma *tensor.View, avg, rstd tensor.Storage, R, D, H, S0, S1 int, layer, set bool) {
	gv := gt.readNativeP(gamma)
	var dx *storage
	dxT := tensor.Float32
	if gx != nil {
		dx = g.storageOf(gx)
		dxT = dx.dtype
	}
	_, sm := g.sr.capability()
	P := min((R+warpRows-1)/warpRows, max(sm, 1)*8)
	chunk := (R + P - 1) / P
	P = (R + chunk - 1) / chunk
	part := gt.buffer(P*D, false)
	a := normConfig{D: D, layer: layer, gamma: gv != nil, xGrad: dx != nil, gammaGrad: true, set: set,
		dt: [5]tensor.DType{g.storageOf(x.S).dtype, gd.dtype, g.viewType(gv), tensor.Float32, dxT}}
	c := g.takeKernel(a.key("norm_geri"), normBackwardWarpKernel(a))
	p := g.param().writeTo(dx).write(part, 0).read(gd, 0).readV(x)
	g.normOperands(p, gv, avg, rstd).u32(R).u32(chunk).u32(H).u32(S0).u32(S1)
	g.launch(c, [3]uint32{uint32(P), 1, 1}, [3]uint32{32 * warpRows, 1, 1}, p)
	g.rowSums(ggamma, part, P, D)
}

// normOperands norm geri çekirdeklerinin ortak işlenenlerini parametre listesine ekler: gamma (yoksa boş),
// ortalama (yoksa boş) ve rstd.
func (g *backend) normOperands(p *paramWriter, gv *tensor.View, avg, rstd tensor.Storage) *paramWriter {
	if gv != nil {
		p.readV(*gv)
	} else {
		p.empty()
	}
	var od *storage
	if avg != nil {
		od = g.storageOf(avg)
	}
	return p.readFrom(od).read(g.storageOf(rstd), 0)
}

// normBackward katman ya da RMS normunun geri yönü.
// set doğruysa gx tazedir (NormGradSetter): eski içeriği okunmadan üzerine yazılır; satır başına
// bloklu çekirdeğin yolunda (γ gradyanı yok ya da çok geniş satır) önce sıfırlanır.
func (g *backend) normBackward(gx, ggamma, gbeta, gs tensor.Storage, x tensor.View, gamma *tensor.View, avg, rstd tensor.Storage, D int, layer, set bool) {
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
	gd := g.storageOf(gs)
	if H, S0, S1, ok := rowMap(x, D); ok && ggamma != nil && normWarpGradD(D) {
		g.normBackwardWarp(gt, gx, ggamma, gd, x, gamma, avg, rstd, R, D, H, S0, S1, layer, set)
		if gbeta != nil {
			g.rowSums(gbeta, gd, R, D)
		}
		return
	}
	if gx != nil || ggamma != nil {
		xv := gt.readNative(x)
		gv := gt.readNativeP(gamma)
		var dx *storage
		if gx != nil {
			dx = g.storageOf(gx)
		}
		dxT := tensor.Float32
		if dx != nil {
			dxT = dx.dtype
		}
		if B := powerOfTwo(D, 32, 256); ggamma != nil && D <= B*normGradCols {
			g.normBackwardFused(gt, dx, ggamma, gd, xv, gv, avg, rstd, R, D, layer, set, dxT)
			if gbeta != nil {
				g.rowSums(gbeta, gd, R, D)
			}
			return
		}
		if set && dx != nil {
			g.fill(dx, 0) // satır başına bloklu çekirdek ekler: taze gradyan önce sıfırlanır
		}
		var term *storage
		if ggamma != nil {
			term = gt.buffer(R*D, false)
		}
		a := normConfig{B: powerOfTwo(D, 32, 256), layer: layer, gamma: gv != nil, xGrad: dx != nil, gammaGrad: term != nil,
			dt: [5]tensor.DType{g.storageOf(xv.S).dtype, gd.dtype, g.viewType(gv), tensor.Float32, dxT}}
		c := g.takeKernel(a.key("norm_geri"), normBackwardKernel(a))
		p := g.param().writeTo(dx).writeTo(term).read(gd, 0).readV(xv)
		g.normOperands(p, gv, avg, rstd).u32(D)
		g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(a.B), 1, 1}, p)
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
	g.normBackward(gx, ggamma, gbeta, gs, x, gamma, mean, rstd, D, true, false)
}

// LayerNormGradSet (tensor.NormGradSetter) LayerNormGrad'ın taze gx'in üzerine yazan biçimi.
func (g *backend) LayerNormGradSet(gx, ggamma, gbeta, gs tensor.Storage, x tensor.View, gamma *tensor.View, mean, rstd tensor.Storage, D int) {
	g.enter()
	defer g.exit()
	g.normBackward(gx, ggamma, gbeta, gs, x, gamma, mean, rstd, D, true, true)
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
	g.normBackward(gx, ggamma, nil, gs, x, gamma, nil, rinv, D, false, false)
}

// RMSNormGradSet (tensor.NormGradSetter) RMSNormGrad'ın taze gx'in üzerine yazan biçimi.
func (g *backend) RMSNormGradSet(gx, ggamma, gs tensor.Storage, x tensor.View, gamma *tensor.View, rinv tensor.Storage, D int) {
	g.enter()
	defer g.exit()
	g.normBackward(gx, ggamma, nil, gs, x, gamma, nil, rinv, D, false, true)
}

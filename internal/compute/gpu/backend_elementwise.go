package gpu

import (
	"fmt"
	"math/bits"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Arka uç: eleman bazlı işlemler, gradyan biriktirme ve genel indirgeme planı.

// reduceJob genel indirgemenin bir çağrısı: ortak gezinme şekli üzerinde A, O (ve Y = A
// adımlı) okunur; çıktı adımı 0 olan eksenler indirgenir.
type reduceJob struct {
	a, o, y        *storage
	aOf, oOf       int
	output         *storage
	outputOf       int
	shape2         []int
	stA2, stO2, st []int // st: çıktı adımları
	term           termType
	add            bool
	scaleF         float32
}

// powerOfTwo n'den küçük olmayan en küçük ikinin kuvveti (sınırlı).
func powerOfTwo(n, minN, maxN int) int {
	if n <= minN {
		return minN
	}
	return min(1<<bits.Len(uint(n-1)), maxN)
}

func (g *backend) reduce(is reduceJob) {
	var kSh, kA, kO, kC, rSh, rA, rO []int
	for d, n := range is.shape2 {
		if n == 1 {
			continue
		}
		oa := 0
		if is.stO2 != nil {
			oa = is.stO2[d]
		}
		if is.st[d] != 0 || n == 0 {
			kSh, kA, kO, kC = append(kSh, n), append(kA, is.stA2[d]), append(kO, oa), append(kC, is.st[d])
		} else {
			rSh, rA, rO = append(rSh, n), append(rA, is.stA2[d]), append(rO, oa)
		}
	}
	kSh, ks := mergeAxes(kSh, kA, kO, kC)
	rSh, rs := mergeAxes(rSh, rA, rO)
	K, R := numel(kSh), numel(rSh)
	if K == 0 {
		return
	}
	checkSize(K)
	checkSize(R)
	if len(kSh) > maxAxis || len(rSh) > maxAxis {
		none(messages.H4051, len(kSh), len(rSh), maxAxis)
	}
	config := reduceConfig{nk: len(kSh), nr: len(rSh), term: is.term, add: is.add}
	for i, d := range []*storage{is.a, is.o, is.y, is.output} {
		config.dt[i] = tensor.Float32
		if d != nil {
			config.dt[i] = d.dtype
		}
	}
	// Çıktı başına blok: indirgenen eksen yeterince uzunsa; ancak çıktılar bitişik
	// sütunlarsa (A'nın korunan iç adımı 1) ve çok sayıdaysa iş parçacığı başına çıktı
	// okumaları birleştirir.
	col := ks[0][len(kSh)-1] == 1 && K >= 256
	if R >= 64 && !col {
		config.block = powerOfTwo(R, 32, 256)
	}
	meta := append(append(append(append(append(append(append([]int{}, kSh...), ks[0]...), ks[1]...), ks[2]...), rSh...), rs[0]...), rs[1]...)
	if S := reducePartCount(K, R, config.block); S > 1 {
		g.reduceTwoStage(is, config, meta, kSh, ks[2], K, R, S)
		return
	}
	c := g.takeKernel(config.key(), reduceKernel(config))
	p := g.param().read(is.a, is.aOf)
	if is.o != nil {
		p.read(is.o, is.oOf)
	} else {
		p.empty()
	}
	if is.y != nil {
		p.read(is.y, 0)
	} else {
		p.empty()
	}
	p.write(is.output, is.outputOf).u32(K).u32(R).f32(is.scaleF).arr(len(meta), meta...)
	if config.block > 0 {
		g.launch(c, [3]uint32{uint32(K), 1, 1}, [3]uint32{uint32(config.block), 1, 1}, p)
		return
	}
	iz, bl := plain(K, sameBlock)
	g.launch(c, iz, bl, p)
}

// reducePartCount indirgenen aralığın kaç parçaya bölüneceği: çıktı az, indirgenen eksen
// uzunken (ör. sapma gradyanı K = 512, R = 8192; hepsinin toplamı K = 1) tek aşamada çok az
// iş parçacığı çalışırdı. Parça sayısı yalnızca K ve R'ye bağlıdır (belirlenimci).
func reducePartCount(K, R, block int) int {
	is := K // aynı anda çalışan iş parçacığı
	if block > 0 {
		is = K * block
	}
	const target = 1 << 17
	if is >= target/2 || R < 2048 {
		return 1
	}
	return min((target+is-1)/is, R/512, 512)
}

// reduceTwoStage belirlenimci iki aşamalı indirgeme: her parça kısmi toplamını yazar,
// ikinci aşama kısmi toplamları parça sırasıyla toplayıp çıktıya (ölçek, ekleme ve çıktı
// adımlarıyla) yazar.
func (g *backend) reduceTwoStage(is reduceJob, config reduceConfig, meta, kSh, kSt []int, K, R, S int) {
	P := (R + S - 1) / S
	partial := g.allocate(S*K, tensor.Float32, false)
	defer g.release(partial)
	config.chunked, config.add, config.dt[3] = true, false, tensor.Float32
	c := g.takeKernel(config.key(), reduceKernel(config))
	p := g.param().read(is.a, is.aOf)
	if is.o != nil {
		p.read(is.o, is.oOf)
	} else {
		p.empty()
	}
	p.readFrom(is.y).write(partial, 0).u32(K).u32(R).f32(1).arr(len(meta), meta...).u32(P)
	if config.block > 0 {
		g.launch(c, [3]uint32{uint32(K), uint32(S), 1}, [3]uint32{uint32(config.block), 1, 1}, p)
	} else {
		g.launch(c, [3]uint32{blocks(K, sameBlock), uint32(S), 1}, [3]uint32{sameBlock, 1, 1}, p)
	}
	shape2 := append([]int{S}, kSh...)
	g.reduce(reduceJob{a: partial, output: is.output, outputOf: is.outputOf, shape2: shape2,
		stA2: append([]int{K}, contiguous(kSh)...), st: append([]int{0}, kSt...), term: tA, add: is.add, scaleF: is.scaleF})
}

// plainV depolamanın tamamına bitişik görünüm.
func plainV(d *storage) tensor.View { return tensor.View{S: d, Shape: []int{d.n}, Strides: []int{1}} }

// AddTo dst += ölçek·src; iki taraf da kendi türünde okunur ve yazılır (16 bitlik hedef
// float32'de toplanıp bir kez yuvarlanır).
func (g *backend) AddTo(dst, src tensor.Storage, scale float32) {
	g.enter()
	defer g.exit()
	d, s := g.storageOf(dst), g.storageOf(src)
	v := plainV(d)
	g.copyStrided(v, tensor.View{S: s, Shape: v.Shape, Strides: v.Strides}, true, scale)
}

func (g *backend) ScatterAdd(dst tensor.Storage, dstOff int, dstStrides []int, src tensor.Storage, shape []int) {
	g.enter()
	defer g.exit()
	g.reduce(reduceJob{a: g.storageOf(src), output: g.storageOf(dst), outputOf: dstOff, shape2: shape, stA2: contiguous(shape),
		st: dstStrides, term: tA, add: true, scaleF: 1})
}

// ExpandAdd dst += ölçek·yayın(src); türler AddTo'daki gibi korunur.
func (g *backend) ExpandAdd(dst tensor.Storage, dstShape []int, src tensor.Storage, srcShape []int, scale float32) {
	g.enter()
	defer g.exit()
	d, s := g.storageOf(dst), g.storageOf(src)
	kv := tensor.View{S: s, Shape: dstShape, Strides: broadcastStride(contiguous(srcShape), srcShape, dstShape)}
	g.copyStrided(tensor.View{S: d, Shape: dstShape, Strides: contiguous(dstShape)}, kv, true, scale)
}

func (g *backend) Binary(op tensor.BinOp, out tensor.Storage, outShape []int, a, b tensor.View) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o := g.storageOf(out)
	n := numel(outShape)
	if n == 0 {
		return
	}
	checkSize(n)
	sa := broadcastStride(a.Strides, a.Shape, outShape)
	sb := broadcastStride(b.Strides, b.Shape, outShape)
	sh, st := mergeAxes(outShape, sa, sb, contiguous(outShape))
	nd := len(sh)
	if nd == 1 && st[0][0] == 1 && st[1][0] == 1 {
		nd = 0 // üçü de düz bitişik
	}
	if nd > maxAxis {
		none(messages.H4052, nd, maxAxis)
	}
	var meta []int
	if nd > 0 {
		meta = append(append(append(meta, sh...), st[0]...), st[1]...)
	}
	dt := [3]tensor.DType{g.storageOf(a.S).dtype, g.storageOf(b.S).dtype, o.dtype}
	c := g.takeKernel(fmt.Sprintf("ikili:%d:%d:%v", op, nd, dt), binaryKernel(op, nd, dt))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().readV(a).readV(b).write(o, 0).u32(n).arr(3*nd, meta...))
}

// readStrided float32 görünümü olduğu gibi (adımlarıyla) bırakır; 16 bitlik görünümü
// float32 bitişiğe açar.
func (t *temp) readStrided(v tensor.View) tensor.View {
	if t.g.storageOf(v.S).dtype == tensor.Float32 {
		return v
	}
	return t.read(v)
}

var gradTerm = map[tensor.GradMode]struct {
	t      termType
	scaleF float32
}{
	tensor.GradPlus: {tA, 1}, tensor.GradMinus: {tA, -1}, tensor.GradMul: {tAO, 1},
	tensor.GradDiv: {tAbolO, 1}, tensor.GradDivB: {tAYbO, -1},
}

func (g *backend) BinaryGrad(mode tensor.GradMode, dst tensor.Storage, dstShape, outShape []int, gs tensor.Storage, o tensor.View, y tensor.Storage) {
	g.enter()
	defer g.exit()
	var yd *storage
	if y != nil {
		yd = g.storageOf(y)
	}
	tm := gradTerm[mode]
	is := reduceJob{a: g.storageOf(gs), y: yd, output: g.storageOf(dst), shape2: outShape, stA2: contiguous(outShape),
		st: broadcastStride(contiguous(dstShape), dstShape, outShape), term: tm.t, add: true, scaleF: tm.scaleF}
	if o.S != nil && tm.t != tA {
		is.o, is.oOf, is.stO2 = g.storageOf(o.S), o.Offset, broadcastStride(o.Strides, o.Shape, outShape)
	}
	g.reduce(is)
}

func (g *backend) Unary(op tensor.UnOp, p float32, out tensor.Storage, x tensor.View) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o := g.storageOf(out)
	xv := gt.readNative(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	checkSize(n)
	mode2 := ""
	if op == tensor.OpPow {
		mode2 = powMode(p)
	}
	xt := g.storageOf(xv.S).dtype
	c := g.takeKernel(fmt.Sprintf("tekli:%d:%s:%s:%s", op, mode2, xt, o.dtype), unaryKernel(op, mode2, xt, o.dtype))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().readV(xv).write(o, 0).u32(n).f32(p))
}

func (g *backend) UnaryGrad(op tensor.UnOp, p float32, gx, gs tensor.Storage, x tensor.View, y tensor.Storage) {
	g.enter()
	defer g.exit()
	g.unaryGrad(op, p, gx, gs, x, y, false)
}

// UnaryGradSet (tensor.UnaryGradSetter) UnaryGrad gibidir; gx'in üzerine yazar.
func (g *backend) UnaryGradSet(op tensor.UnOp, p float32, gx, gs tensor.Storage, x tensor.View, y tensor.Storage) {
	g.enter()
	defer g.exit()
	g.unaryGrad(op, p, gx, gs, x, y, true)
}

func (g *backend) unaryGrad(op tensor.UnOp, p float32, gx, gs tensor.Storage, x tensor.View, y tensor.Storage, set bool) {
	if op == tensor.OpPow && p == 0 || op == tensor.OpFloor || op == tensor.OpCeil || op == tensor.OpSign {
		// x^0 sabittir, taban/tavan/işaret parçalı sabittir: gradyan 0
		if set {
			g.fill(g.storageOf(gx), 0)
		}
		return
	}
	gt := g.temp()
	defer gt.finish()
	d, gd := g.storageOf(gx), g.storageOf(gs)
	var yd *storage
	if y != nil {
		yd = g.storageOf(y)
	}
	if x.S == nil {
		// Doğrusal işlemlerin (x + s, x · s, s − x) geri yönü x'i okumaz; verisi çizgede
		// tutulmaz (yalnızca şekli gelir). Çekirdeğe aynı boyda okunabilir bir yer tutucu
		// (gelen gradyan) verilir.
		x = tensor.DenseView(gs, x.Shape)
	}
	xv := gt.readNative(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	checkSize(n)
	mode2 := ""
	if op == tensor.OpPow {
		mode2 = powMode(p)
	}
	if yd == nil {
		yd = gd // y'yi kullanmayan işlemler için okunabilir yer tutucu (aynı boyda)
	}
	dt := [4]tensor.DType{d.dtype, gd.dtype, g.storageOf(xv.S).dtype, yd.dtype}
	c := g.takeKernel(fmt.Sprintf("tekli_turev:%d:%s:%v:%v", op, mode2, dt, set), unaryGradKernel(op, mode2, dt, set))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().write(d, 0).read(gd, 0).readV(xv).read(yd, 0).u32(n).f32(p))
}

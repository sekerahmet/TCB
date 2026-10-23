package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Adımlı kopya, tür dönüşümü, yayınlamalı ekleme ve doldurma çekirdekleri
// ============================================================================

// maxAxis adımlı çekirdeklerin desteklediği en çok eksen (birleştirmeden sonra).
const maxAxis = 8

// sameBlock eleman bazlı çekirdeklerin blok boyu.
const sameBlock = 256

// parse i doğrusal indeksini (satır öncelikli) şekle göre ayrıştırıp her işlenenin
// ofsetini verir. sekil ve adımlar "meta" dizisinden okunur: sekil[d] = meta[d],
// adim[k][d] = meta[(k+1)·nd + d]. Ofsetler işaretli 32 bittir.
func (k *emitter) parse(i reg, nd, nOperand int, meta string) []reg {
	return k.parseBase(i, nd, nOperand, 0, meta)
}

// copyKernel hedef (td, adımlı) ← kaynak (ts, adımlı); ekle doğruysa hedef += ölçek·kaynak
// (toplam float32'de yapılır, 16 bitlik hedefe bir kez yuvarlanır).
func copyKernel(nd int, ts, td tensor.DType, add bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("kopya_%d_%s_%s_%v", nd, ts, td, add))
		src := k.pointer("kaynak")
		dst := k.written("hedef")
		n := k.u32p("n")
		scaleF := k.f32p("olcek")
		k.arr("meta", 3*nd)
		i := k.globalIndex()
		last := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, i, n)
		k.jumpIf(p, last)
		ofs := k.parse(i, nd, 2, "meta")
		v := k.load(ts, k.addressS(src, ofs[0], dtypeSize(ts)))
		ha := k.addressS(dst, ofs[1], dtypeSize(td))
		if add {
			old := k.load(td, ha)
			k.e("fma.rn.f32", v, v, scaleF, old)
		}
		k.store(td, ha, v)
		k.put(last)
		return k
	}
}

// fillKernel hedefin n elemanını v yapar (dt türüne yuvarlanmış).
func fillKernel(dt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter("doldur_" + dt.String())
		dst := k.written("hedef")
		n := k.u32p("n")
		v := k.f32p("deger")
		i := k.globalIndex()
		last := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, i, n)
		k.jumpIf(p, last)
		k.store(dt, k.address(dst, i, dtypeSize(dt)), v)
		k.put(last)
		return k
	}
}

// copyStrided hedef görünümüne kaynak görünümünü yazar (ekle: hedef += ölçek·kaynak).
// Şekiller aynıdır; kaynak yayınlanmış olabilir (adım 0).
func (g *backend) copyStrided(target, source tensor.View, add bool, scaleF float32) {
	n := numel(target.Shape)
	if n == 0 {
		return
	}
	checkSize(n)
	hd, kd := g.storageOf(target.S), g.storageOf(source.S)
	kind := tensor.ImplicitAccumulate
	if !add {
		kind = tensor.ImplicitKindOf(hd.dtype, source, target.IsContiguous())
	}
	tensor.NoteImplicit(true, kind, n) // GPU'da süre ölçülmez (Finish gerekmez)
	sh, st := tensor.MergeAxes(target.Shape, source.Strides, target.Strides)
	// Aynı tür, iki taraf bitişik: sürücünün cihaz içi kopyası.
	if !add && kd.dtype == hd.dtype && len(sh) == 1 && st[0][0] == 1 && st[1][0] == 1 {
		g.dCopy(hd, target.Offset, kd, source.Offset, n)
		return
	}
	if len(sh) > maxAxis {
		none(messages.H4057, len(sh), maxAxis)
	}
	meta := append(append(append([]int{}, sh...), st[0]...), st[1]...)
	c := g.takeKernel(fmt.Sprintf("kopya:%d:%d:%d:%v", len(sh), kd.dtype, hd.dtype, add), copyKernel(len(sh), kd.dtype, hd.dtype, add))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().read(kd, source.Offset).write(hd, target.Offset).u32(n).f32(scaleF).arr(3*len(sh), meta...))
}

// fill depolamanın bütün elemanlarını v yapar.
func (g *backend) fill(d *storage, v float32) {
	if d.n == 0 {
		return
	}
	if d.dtype == tensor.Float32 {
		g.fill32(d, math.Float32bits(v))
		return
	}
	checkSize(d.n)
	c := g.takeKernel("doldur:"+d.dtype.String(), fillKernel(d.dtype))
	iz, bl := plain(d.n, sameBlock)
	g.launch(c, iz, bl, g.param().write(d, 0).u32(d.n).f32(v))
}

// contiguous görünümün satır öncelikli adımları.
func contiguous(shape []int) []int {
	st := make([]int, len(shape))
	s := 1
	for i := len(shape) - 1; i >= 0; i-- {
		st[i] = s
		s *= shape[i]
	}
	return st
}

func numel(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

// broadcastStride, şekli shape ve adımları st olan görünümün hedef şekle yayınlanmış adımları
// (eksik ve 1 uzunluklu eksenlerde 0).
func broadcastStride(st, shape, target []int) []int {
	out := make([]int, len(target))
	diff := len(target) - len(shape)
	for i := range shape {
		if shape[i] != 1 {
			out[diff+i] = st[i]
		}
	}
	return out
}

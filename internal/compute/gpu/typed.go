package gpu

import (
	"fmt"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Cinsli tensör (15.1): float64, TR_Int ve TR_Bool depolaması (tensor.RawBackend)
// ============================================================================
//
// Ham bayt türleri cihazda öğe başına hassasiyetin genişliğinde durur (mantık bir bayt).
// Ondalık çekirdekler bunları görmez: emitter.load/store hassasiyet hatası verir. Bu
// dilimde GPU'da yalnız adım 8'in yolları vardır (typed_ops.go); öbür işlemler 15.48'in
// hassasiyet hatasıdır.

// FromRaw ham baytlardan dt türünde depolama kurar.
func (g *backend) FromRaw(raw []byte, dt tensor.DType) tensor.Storage {
	g.enter()
	defer g.exit()
	g.noCapture(messages.H4066)
	d := g.allocate(len(raw)/dtypeSize(dt), dt, false)
	if len(raw) > 0 {
		g.loadHost(d, unsafe.Pointer(&raw[0]), len(raw))
	}
	return d
}

// ToRaw görünümün öğelerini mantıksal sırayla ham bayt olarak konağa kopyalar.
func (g *backend) ToRaw(src tensor.View) []byte {
	g.enter()
	defer g.exit()
	g.noCapture(messages.H4065)
	d := g.storageOf(src.S)
	n, size := src.Numel(), dtypeSize(d.dtype)
	out := make([]byte, n*size)
	if n == 0 {
		return out
	}
	gt := g.temp()
	defer gt.finish()
	v := gt.readNative(src)
	g.download(unsafe.Pointer(&out[0]), g.addressOf(v), n*size)
	return out
}

// FillRaw bütün öğeleri elem'in değeri yapar (konakta kurulup yüklenir; kayıtta olmaz).
func (g *backend) FillRaw(dst tensor.Storage, elem []byte) {
	g.enter()
	defer g.exit()
	d := g.storageOf(dst)
	if d.n == 0 {
		return
	}
	buf := make([]byte, d.n*len(elem))
	for i := 0; i < d.n; i++ {
		copy(buf[i*len(elem):], elem)
	}
	g.loadHost(d, unsafe.Pointer(&buf[0]), len(buf))
}

// rawCopyKernel hedef (adımlı) ← kaynak (adımlı, aynı tür): öğeler size baytlık tam sayı
// olarak taşınır (dönüşüm yok).
func rawCopyKernel(nd, size int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("ham_kopya_%d_%d", nd, size))
		src := k.pointer("kaynak")
		dst := k.written("hedef")
		n := k.u32p("n")
		k.arr("meta", 3*nd)
		i := k.globalIndex()
		last := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, i, n)
		k.jumpIf(p, last)
		ofs := k.parse(i, nd, 2, "meta")
		from, to := k.addressS(src, ofs[0], size), k.addressS(dst, ofs[1], size)
		suffix := map[int]string{1: "u8", 2: "u16", 4: "u32", 8: "u64"}[size]
		v := k.r()
		if size == 8 {
			v = k.rd()
		} else if size == 2 {
			v = k.h()
		}
		k.e("ld.global."+suffix, v, mem(from, 0))
		k.e("st.global."+suffix, mem(to, 0), v)
		k.put(last)
		return k
	}
}

// copyTyped aynı türden iki ham bayt görünümü arasında adımlı kopya (kaynak yayınlanmış
// olabilir). Türler farklıysa iç hatadır: dönüşüm typed_ops.go'dadır.
func (g *backend) copyTyped(target, source tensor.View, hd, kd *storage) {
	if hd.dtype != kd.dtype {
		panic(tensor.PrecisionError(kd.dtype, g.dev))
	}
	n := tensor.NumelOf(target.Shape)
	sh, st := tensor.MergeAxes(target.Shape, source.Strides, target.Strides)
	if len(sh) == 1 && st[0][0] == 1 && st[1][0] == 1 {
		g.dCopy(hd, target.Offset, kd, source.Offset, n)
		return
	}
	if len(sh) > maxAxis {
		none(messages.H4057, len(sh), maxAxis)
	}
	size := dtypeSize(hd.dtype)
	meta := append(append(append([]int{}, sh...), st[0]...), st[1]...)
	c := g.takeKernel(fmt.Sprintf("ham_kopya:%d:%d", len(sh), size), rawCopyKernel(len(sh), size))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().read(kd, source.Offset).write(hd, target.Offset).u32(n).arr(3*len(sh), meta...))
}

// checkFloat ondalık çekirdeğin girdisi ya da çıktısı ham bayt türündeyse hassasiyet hatası.
func (g *backend) checkFloat(d *storage) {
	if !d.dtype.IsFloat() {
		panic(tensor.PrecisionError(d.dtype, g.dev))
	}
}

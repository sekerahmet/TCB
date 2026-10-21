package gpu

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// İndeksi cihazdan okuyan işlemler (kayıt içinde: tensor.IndexReader)
// ============================================================================

// updateSliceKernel hedef [dış, Td, iç] içine kaynağı [dış, Ts, iç] eksende s'den başlayarak
// yazar; s = kırp(başlangıç[0], 0, Td−Ts) (cihazdaki float32 tam sayı). Eleman başına iş
// parçacığı; kaynak bitişik, türler farklıysa yazarken çevrilir.
func updateSliceKernel(st, dt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter("dilim_yaz_" + st.String() + "_" + dt.String())
		dst := k.written("hedef")
		src := k.pointer("kaynak")
		start := k.pointer("baslangic")
		n, Td, Ts, I := k.u32p("n"), k.u32p("td"), k.u32p("ts"), k.u32p("ic")
		e := k.globalIndex()
		end := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, e, n)
		k.jumpIf(p, end)
		s, room := k.r(), k.r()
		k.e("cvt.rzi.s32.f32", s, k.load(tensor.Float32, start))
		k.e("sub.s32", room, Td, Ts)
		k.e("add.s32", room, room, 1)
		k.clampIndex(s, room)
		tsI, o, rest, a, i := k.r(), k.r(), k.r(), k.r(), k.r()
		k.e("mul.lo.s32", tsI, Ts, I)
		k.e("div.u32", o, e, tsI)
		k.e("mul.lo.s32", rest, o, tsI)
		k.e("sub.s32", rest, e, rest)
		k.e("div.u32", a, rest, I)
		k.e("mul.lo.s32", i, a, I)
		k.e("sub.s32", i, rest, i)
		di := k.r()
		k.e("mad.lo.s32", di, o, Td, s)
		k.e("add.s32", di, di, a)
		k.e("mad.lo.s32", di, di, I, i)
		k.store(dt, k.address(dst, di, dtypeSize(dt)), k.load(st, k.address(src, e, dtypeSize(st))))
		k.put(end)
		return k
	}
}

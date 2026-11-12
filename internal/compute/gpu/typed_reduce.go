package gpu

import "github.com/sekerahmet/TCB/internal/compute/tensor"

// ============================================================================
// "int64" toplamı kartta (tensor.TypedReducer; 15.18, 15.6; T1 dilim 2b)
// ============================================================================
//
// İşlemcinin reduceInt64'ü (tensor/typed_reduce.go) ile aynı anlam: toplam 128 bitte kesindir,
// yalnız sonuç int64'e sığmalıdır. Her çıktıyı bir iş parçacığı eksen boyunca sırayla toplar;
// tam sayı toplamı birleşmeli olduğu için sonuç sıradan ve iş parçacığı sayısından bağımsızdır,
// işlemciyle bit bit aynıdır. Taşan çıktının anahtarı (düz konum << 4 | taşma) bayrağın
// yuvasına tek atom.global.min.u64 ile yazılır (en küçük düz konum kazanır, 15.6). Toplama
// çekirdeği kazanan çıktının girdilerini yuvaya sığdıramaz (n öğe): onları tek iş parçacığında
// yeniden toplar, 128 bitlik toplamın iki yarısını yuvanın 1. (alt) ve 2. (üst) sözcüğüne
// yazar; konak ElemFault'u oradan kurar (tensor.SumFaultFromSlot). Kayıtta yuva kayda girer.

// SumInt64 (tensor.TypedReducer) bitişik "int64" in'i (outer, n, inner) şeklinde orta eksen
// boyunca toplar; out bitişik, outer·inner öğe.
func (g *backend) SumInt64(out tensor.Storage, in tensor.View, outer, n, inner int) (*tensor.ElemFault, bool) {
	g.enter()
	defer g.exit()
	o := g.storageOf(out)
	if g.storageOf(in.S).dtype != tensor.Int64 || o.dtype != tensor.Int64 {
		return nil, false
	}
	m := outer * inner
	if m == 0 {
		return nil, true
	}
	checkSize(m)
	checkSize(outer * n * inner)
	slot := g.takeSlot()
	k := g.takeKernel("toplam_int64", sumInt64Kernel)
	iz, bl := plain(m, sameBlock)
	g.launch(k, iz, bl, g.param().readV(in).write(o, 0).u32(m).u32(n).u32(inner).write(slot, 0))
	gk := g.takeKernel("hata_topla_toplam_int64", sumGatherKernel)
	g.launch(gk, [3]uint32{1, 1, 1}, [3]uint32{1, 1, 1}, g.param().readV(in).u32(n).u32(inner).write(slot, 0))
	// Şekil yazılmaz: işlemcinin toplam hatası da konumsuzdur (iletide konum yok).
	site := faultSite{slot: slot, sum: true}
	if g.capturing != nil {
		return g.recordSite(site), true
	}
	return g.readSlot(site), true
}

// sum128 x'in (j / inner)·n·inner + j % inner'dan başlayıp inner adımla giden n öğesini 128
// bitte toplar: lo alt 64 bit, hi üst 64 bit (işaretli).
func (k *emitter) sum128(x, j, n, inner reg) (lo, hi reg) {
	o, i, base := k.r(), k.r(), k.r()
	k.e("div.u32", o, j, inner)
	k.e("rem.u32", i, j, inner)
	k.e("mul.lo.u32", base, o, n)
	k.e("mul.lo.u32", base, base, inner)
	k.e("add.u32", base, base, i)
	lo, hi = k.rd(), k.rd()
	k.e("mov.u64", lo, 0)
	k.e("mov.s64", hi, 0)
	step := k.r()
	k.e("mov.u32", step, 0)
	loop, done := k.label(), k.label()
	k.put(loop)
	end := k.p()
	k.e("setp.ge.u32", end, step, n)
	k.jumpIf(end, done)
	idx := k.r()
	k.e("mad.lo.u32", idx, step, inner, base)
	v := k.rawLoad(tensor.Int64, k.address(x, idx, 8))
	// (hi, lo) += (v'nin işareti, v): alt yarının eldesi üst yarıya.
	next, sign, carry := k.rd(), k.rd(), k.rd()
	k.e("add.u64", next, lo, v)
	c := k.p()
	k.e("setp.lt.u64", c, next, lo)
	k.e("selp.u64", carry, 1, 0, c)
	k.e("shr.s64", sign, v, 63)
	k.e("add.s64", hi, hi, sign)
	k.e("add.s64", hi, hi, carry)
	k.e("mov.u64", lo, next)
	k.e("add.u32", step, step, 1)
	k.jump(loop)
	k.put(done)
	return lo, hi
}

// sumInt64Kernel tek iş parçacığı bir çıktı: 128 bitlik toplam sığarsa yazılır, sığmazsa
// anahtar bayrağa (çıktı yazılmaz).
func sumInt64Kernel() *emitter {
	k := newEmitter("toplam_int64")
	x := k.pointer("girdi")
	dst := k.written("cikti")
	m, n, inner := k.u32p("m"), k.u32p("n"), k.u32p("ic")
	flag := k.written("bayrak")
	j := k.globalIndex()
	last := k.bounded(j, m)
	lo, hi := k.sum128(x, j, n, inner)
	top, bad := k.rd(), k.p()
	k.e("shr.s64", top, lo, 63)
	k.e("setp.ne.s64", bad, hi, top)
	kind := k.r()
	k.e("mov.u32", kind, int(tensor.FaultOverflow))
	k.storeOrFault(bad, kind, lo, k.address(dst, j, 8), j, flag)
	k.put(last)
	return k
}

// sumGatherKernel tek iş parçacığı: yuvanın anahtarı doluysa kazanan çıktıyı yeniden toplar,
// 128 bitlik toplamın alt yarısını yuvanın 1., üst yarısını 2. sözcüğüne yazar.
func sumGatherKernel() *emitter {
	k := newEmitter("hata_topla_toplam_int64")
	x := k.pointer("girdi")
	n, inner := k.u32p("n"), k.u32p("ic")
	slot := k.written("yuva")
	key := k.rd()
	k.e("ld.global.u64", key, mem(slot, 0))
	empty := k.p()
	k.e("setp.eq.u64", empty, key, int64(-1))
	end := k.label()
	k.jumpIf(empty, end)
	k.e("shr.u64", key, key, 4)
	j := k.r()
	k.e("cvt.u32.u64", j, key)
	lo, hi := k.sum128(x, j, n, inner)
	k.e("st.global.u64", mem(slot, 8), lo)
	k.e("st.global.u64", mem(slot, 16), hi)
	k.put(end)
	return k
}

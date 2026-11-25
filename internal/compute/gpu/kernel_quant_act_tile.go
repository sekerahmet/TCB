package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Paylaşımlı bellek karolu dp4a çarpımı (M > 1'de sm_80 altı ve q4; M = 1'de paylaşımlı belleğe sığmayan K): blok 128 iş parçacığı, 64 satır
// × 64 sütun. Her 256'lık dilimde (q4_k ve q6_k bir blok, q4 sekiz blok) etkinleştirmenin kodları ve ağırlığın
// açılmış sözcükleri paylaşımlı belleğe bir kez yüklenir; iş parçacığı 8 satır × 4 sütun hesaplar (satır rg + 8i,
// sütun cg + 16j). Tamsayı toplamlar kesindir; blok formülü her çıktıda CPU'nun sırasıyla döner (dotQ4Q8Go,
// dotQ4KQ8Go, dotQ6KQ8Go). T4'te (sm_75) eski float yolundan hızlıdır (defter 9; deneme 2).
const (
	tileRows    = 8 // iş parçacığının satırı
	tileCols    = 4 // iş parçacığının sütunu
	tileThreads = 128
	tileStride  = 65 // 64 sözcük + 1 (bank çakışmasız)
	tileScale   = 17 // 16 ölçek + 1
)

// quantActTileKernel türün karolu çekirdeğidir; ızgara (N/64, M/64). dp yanlışsa (sm_61 altı) dp4a yerine dört
// tamsayı çarpma-toplama.
func quantActTileKernel(dt tensor.DType, dp bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_etk_karo_%s_%v", dt.Name(), dp))
		if dp {
			k.target = 61
		}
		t := &dp4aTile{k: k, dt: dt}
		t.setup()
		var acc [tileRows][tileCols]reg
		for i := range acc {
			for j := range acc[i] {
				acc[i][j] = k.constant(0)
			}
		}
		chunks := t.nb
		if dt == tensor.Q4 {
			chunks = k.r()
			k.e("add.s32", chunks, t.nb, 7)
			k.e("shr.u32", chunks, chunks, 3)
		}
		b := k.whole(0)
		k.whileLess("u32", b, chunks, func() {
			t.loadActs(b)
			t.loadWeights(b)
			k.barrier()
			switch dt {
			case tensor.Q4K:
				t.computeQ4K(&acc)
			case tensor.Q6K:
				t.computeQ6K(&acc)
			default:
				t.computeQ4(&acc, b)
			}
			k.barrier()
			k.e("add.s32", b, b, 1)
		})
		t.store(&acc)
		return k
	}
}

// dp4aTile karolu çekirdeğin ortak yazmaçlarıdır.
type dp4aTile struct {
	k                        *emitter
	dt                       tensor.DType
	pq, pd, pbs, pw, pc      reg
	pfin                     reg
	M, N, nb, K, rowBytes    reg
	row0, col0, rg, cg       reg
	half, mine               reg // tid & 1, tid >> 1 (yüklemede satır ya da sütun)
	sA, sW, sSc, sD, sDm, sB reg
	sD8                      reg
}

// perChunk dilimdeki ölçek sayısıdır: q4'te 8 blok, öbürlerinde 1.
func (t *dp4aTile) perChunk() int {
	if t.dt == tensor.Q4 {
		return 8
	}
	return 1
}

func (t *dp4aTile) setup() {
	k := t.k
	t.pq, t.pd, t.pbs, t.pw = k.pointer("q"), k.pointer("d"), k.pointer("bs"), k.pointer("w")
	t.pc, t.pfin = k.written("c"), k.pointer("fin")
	t.M, t.N, t.nb = k.u32p("m"), k.u32p("n"), k.u32p("bloklar")
	elems, size := t.dt.QuantBlock()
	t.K, t.rowBytes = k.r(), k.r()
	k.e("shl.b32", t.K, t.nb, log2(elems))
	k.e("mul.lo.s32", t.rowBytes, t.nb, size)
	tid := k.special("%tid.x")
	t.cg, t.rg, t.half, t.mine = k.r(), k.r(), k.r(), k.r()
	k.e("and.b32", t.cg, tid, 15)
	k.e("shr.u32", t.rg, tid, 4)
	k.e("and.b32", t.half, tid, 1)
	k.e("shr.u32", t.mine, tid, 1)
	t.row0, t.col0 = k.r(), k.r()
	k.e("shl.b32", t.row0, k.special("%ctaid.y"), 6)
	k.e("shl.b32", t.col0, k.special("%ctaid.x"), 6)
	n := t.perChunk()
	t.sA = k.shared("karo_a", 4*64*tileStride)
	t.sW = k.shared("karo_w", 4*64*tileStride)
	t.sD = k.shared("karo_d", 4*64*n)
	t.sD8 = k.shared("karo_d8", 4*64*n)
	if t.dt != tensor.Q4 {
		t.sSc = k.shared("karo_olcek", 4*64*tileScale)
	}
	if t.dt == tensor.Q4K {
		t.sDm = k.shared("karo_dmin", 4*64)
		t.sB = k.shared("karo_bs", 4*64*8)
	}
}

// clamp v + o'yu n − 1'e kırpar.
func (t *dp4aTile) clamp(v, o, n reg) reg {
	k := t.k
	r, last := k.r(), k.r()
	k.e("add.s32", r, v, o)
	k.e("sub.s32", last, n, 1)
	k.e("min.u32", r, r, last)
	return r
}

// at sm + 4·(idx·stride + base) adresidir (32 bit).
func (t *dp4aTile) at(sm, idx reg, stride int, base reg) reg {
	k := t.k
	r := k.r()
	k.e("mad.lo.s32", r, idx, stride, base)
	return k.sharedAt(sm, r)
}

// loadActs satır mine'ın (64 satırın biri) b. diliminin yarısını (128 bayt) yükler; yarı 0 d8'i (q4'te sekiz
// bloğunkini), q4_k'da yarı 1 16'lık toplamların çiftlerini yazar. q4'te K 256'nın katı olmayabilir: son dilimin
// dışına düşen kodlar sonraki satırdan ya da tamponun 256 baytlık dolgusundan okunur, hesapta kullanılmaz.
func (t *dp4aTile) loadActs(b reg) {
	k := t.k
	row := t.clamp(t.row0, t.mine, t.M)
	e := k.r()
	k.e("mul.lo.s32", e, b, actBlk)
	k.e("mad.lo.s32", e, t.half, 128, e)
	at := k.r()
	k.e("mad.lo.s32", at, row, t.K, e)
	src := k.address(t.pq, at, 1)
	off := k.r()
	k.e("shl.b32", off, t.half, 5)
	dst := t.at(t.sA, t.mine, tileStride, off)
	for v := range 8 {
		w := k.r4(src, 16*v)
		for j, x := range w {
			k.e("st.shared.u32", mem(dst, 4*(4*v+j)), x)
		}
	}
	n := t.perChunk()
	zero := k.p()
	k.e("setp.eq.u32", zero, t.half, 0)
	skip := k.label()
	k.jumpUnless(zero, skip)
	dd := t.at(t.sD8, t.mine, n, k.whole(0))
	last := k.r()
	k.e("sub.s32", last, t.nb, 1)
	for s := range n {
		di := k.r()
		k.e("mad.lo.s32", di, b, n, k.whole(s))
		k.e("min.u32", di, di, last)
		k.e("mad.lo.s32", di, row, t.nb, di)
		d8 := k.f()
		k.e("ld.global.f32", d8, mem(k.address(t.pd, di, 4), 0))
		k.e("st.shared.f32", mem(dd, 4*s), d8)
	}
	k.put(skip)
	if t.dt != tensor.Q4K {
		return
	}
	done := k.label()
	k.jumpIf(zero, done)
	bi := k.r()
	k.e("mad.lo.s32", bi, row, t.nb, b)
	k.e("shl.b32", bi, bi, 4)
	ba := k.address(t.pbs, bi, 4)
	bd := t.at(t.sB, t.mine, 8, k.whole(0))
	for v := range 4 {
		w := k.r4(ba, 16*v)
		for p := range 2 {
			s := k.r()
			k.e("add.s32", s, w[2*p], w[2*p+1])
			k.e("st.shared.u32", mem(bd, 4*(2*v+p)), s)
		}
	}
	k.put(done)
}

// loadWeights sütun mine'ın b. dilimini açar: q4_k'da yarı qs'nin 64 baytını (iki alt blok çifti), q6_k'da bir
// 128'liği, q4'te dört bloğu. Yarı 0 ölçekleri yazar (q4'te her yarı kendi bloklarınınkini).
func (t *dp4aTile) loadWeights(b reg) {
	k := t.k
	col := t.clamp(t.col0, t.mine, t.N)
	row := k.rd()
	k.e("mul.wide.u32", row, col, t.rowBytes)
	k.e("add.s64", row, row, t.pw)
	off := k.r()
	k.e("shl.b32", off, t.half, 5)
	dst := t.at(t.sW, t.mine, tileStride, off)
	if t.dt == tensor.Q4 {
		t.unpackQ4(row, b, col, dst)
		return
	}
	_, size := t.dt.QuantBlock()
	block := k.address(row, b, size)
	if t.dt == tensor.Q4K {
		t.unpackQ4K(block, dst)
	} else {
		t.unpackQ6K(block, dst)
	}
	zero := k.p()
	k.e("setp.eq.u32", zero, t.half, 0)
	skip := k.label()
	k.jumpUnless(zero, skip)
	sc := t.at(t.sSc, t.mine, tileScale, k.whole(0))
	dd := k.sharedAt(t.sD, t.mine)
	if t.dt == tensor.Q4K {
		for j := range 8 {
			s, m := k.q4kScale(block, k.whole(j))
			k.e("st.shared.u32", mem(sc, 4*j), s)
			k.e("st.shared.u32", mem(sc, 4*(8+j)), m)
		}
		k.e("st.shared.f32", mem(dd, 0), k.half(block, 0))
		k.e("st.shared.f32", mem(k.sharedAt(t.sDm, t.mine), 0), k.half(block, 2))
	} else {
		for w := range 4 {
			v := k.words16(block, 192+4*w)
			for j := range 4 {
				k.e("st.shared.u32", mem(sc, 4*(4*w+j)), k.byteS(v, j))
			}
		}
		k.e("st.shared.f32", mem(dd, 0), k.half(block, 208))
	}
	k.put(skip)
}

// unpackQ4K: ham qs sözcüğü r = yarı·16 + x (g = r/8, w = r%8) alt nibble'da 2g. alt bloğun, üstte
// 2g+1. alt bloğun w. sözcüğüdür; açılmış dizide 32 öğelik alt blok 8 sözcüktür.
func (t *dp4aTile) unpackQ4K(block, dst reg) {
	k := t.k
	src := k.rd()
	o := k.r()
	k.e("shl.b32", o, t.half, 6)
	k.e("cvt.u64.u32", src, o)
	k.e("add.s64", src, src, block)
	for v := range 4 {
		w := k.r4(src, 16+16*v)
		for j, x := range w {
			idx := 4*v + j
			base := (idx/8)*16 + idx%8
			lo, hi := k.r(), k.r()
			k.e("and.b32", lo, x, 0x0F0F0F0F)
			k.e("shr.u32", hi, x, 4)
			k.e("and.b32", hi, hi, 0x0F0F0F0F)
			k.e("st.shared.u32", mem(dst, 4*base), lo)
			k.e("st.shared.u32", mem(dst, 4*(base+8)), hi)
		}
	}
}

// unpackQ6K: yarı n bir 128'liktir; çeyrek q, sözcük lw (l = 4·lw) açılmış dizide n·32 + q·8 + lw'dir
// (16'lık alt blok s = n·8 + q·2 + lw/4, sözcüğü lw%4). Kod q6 − 32, işaretli.
func (t *dp4aTile) unpackQ6K(block, dst reg) {
	k := t.k
	qlO, qhO := k.r(), k.r()
	k.e("shl.b32", qlO, t.half, 6)
	k.e("shl.b32", qhO, t.half, 5)
	k.e("add.s32", qhO, qhO, 128)
	ql, qh := k.rd(), k.rd()
	k.e("cvt.u64.u32", ql, qlO)
	k.e("add.s64", ql, ql, block)
	k.e("cvt.u64.u32", qh, qhO)
	k.e("add.s64", qh, qh, block)
	for lw := range 8 {
		l0, l1, h := k.words16(ql, 4*lw), k.words16(ql, 32+4*lw), k.words16(qh, 4*lw)
		for q := range 4 {
			src := l0
			if q%2 == 1 {
				src = l1
			}
			c, x := k.r(), k.r()
			k.e("shr.u32", c, src, 4*(q/2))
			k.e("and.b32", c, c, 0x0F0F0F0F)
			k.e("shr.u32", x, h, 2*q)
			k.e("and.b32", x, x, 0x03030303)
			k.e("shl.b32", x, x, 4)
			k.e("or.b32", c, c, x)
			k.signedFromBias(c, 0x20202020)
			k.e("st.shared.u32", mem(dst, 4*(q*8+lw)), c)
		}
	}
}

// unpackQ4: yarı h dilimin 4h..4h+3. bloklarını açar (18 bayt: fp16 ölçek, 16 bayt kod); bloğun j. baytının alt
// yarısı j., üst yarısı j+16. öğedir (dequantQ4), kod − 8 işaretli. Ölçek sD[sütun·8 + blok]'a.
func (t *dp4aTile) unpackQ4(row, b, col, dst reg) {
	k := t.k
	last := k.r()
	k.e("sub.s32", last, t.nb, 1)
	dd := t.at(t.sD, t.mine, 8, k.whole(0))
	h4 := k.r()
	k.e("shl.b32", h4, t.half, 2)
	for bb := range 4 {
		bi := k.r()
		k.e("shl.b32", bi, b, 3)
		k.e("add.s32", bi, bi, h4)
		k.e("add.s32", bi, bi, bb)
		k.e("min.u32", bi, bi, last)
		block := k.address(row, bi, 18)
		slot := k.r()
		k.e("add.s32", slot, h4, bb)
		k.e("st.shared.f32", mem(k.sharedAt(dd, slot), 0), k.half(block, 0))
		for w := range 4 {
			raw := k.words16(block, 2+4*w)
			lo, hi := k.r(), k.r()
			k.e("and.b32", lo, raw, 0x0F0F0F0F)
			k.e("shr.u32", hi, raw, 4)
			k.e("and.b32", hi, hi, 0x0F0F0F0F)
			k.signedFromBias(lo, 0x08080808)
			k.signedFromBias(hi, 0x08080808)
			k.e("st.shared.u32", mem(dst, 4*(bb*8+w)), lo)
			k.e("st.shared.u32", mem(dst, 4*(bb*8+4+w)), hi)
		}
	}
}

// signedFromBias sözcüğün baytlarından bias'ı bayt bayt çıkarır: ((c | 0x80) − bias) ^ 0x80 ödünç almaz
// (bayt değeri 0..63, sonuç işaretli bayt).
func (k *emitter) signedFromBias(c reg, bias int) {
	k.e("or.b32", c, c, -2139062144) // 0x80808080
	k.e("sub.s32", c, c, bias)
	k.e("xor.b32", c, c, -2139062144)
}

// rowAddr, colAddr iş parçacığının i. satırının ve j. sütununun karo dizisindeki adresidir.
func (t *dp4aTile) rowAddr(sm reg, i, stride int) reg {
	k := t.k
	r := k.r()
	k.e("add.s32", r, t.rg, 8*i)
	return t.at(sm, r, stride, k.whole(0))
}

func (t *dp4aTile) colAddr(sm reg, j, stride int) reg {
	k := t.k
	c := k.r()
	k.e("add.s32", c, t.cg, 16*j)
	return t.at(sm, c, stride, k.whole(0))
}

func (k *emitter) lds(addr reg, word int) reg {
	r := k.r()
	k.e("ld.shared.u32", r, mem(addr, 4*word))
	return r
}

func (k *emitter) ldsF(addr reg, word int) reg {
	r := k.f()
	k.e("ld.shared.f32", r, mem(addr, 4*word))
	return r
}

// subDots sb. alt bloğun (n sözcük) her (satır, sütun) için tamsayı nokta çarpımıdır.
func (t *dp4aTile) subDots(aRow [tileRows]reg, wCol [tileCols]reg, sb, n int, each func(i, j int, s reg)) {
	k := t.k
	w := make([][]reg, tileCols)
	for j := range w {
		w[j] = make([]reg, n)
		for x := range n {
			w[j][x] = k.lds(wCol[j], n*sb+x)
		}
	}
	for i := range aRow {
		a := make([]reg, n)
		for x := range a {
			a[x] = k.lds(aRow[i], n*sb+x)
		}
		for j := range w {
			s := k.whole(0)
			for x := range a {
				k.dot4(s, w[j][x], a[x])
			}
			each(i, j, s)
		}
	}
}

// tileAddrs iş parçacığının satırlarının ve sütunlarının kod adresleridir.
func (t *dp4aTile) tileAddrs() (aRow [tileRows]reg, wCol [tileCols]reg) {
	for i := range aRow {
		aRow[i] = t.rowAddr(t.sA, i, tileStride)
	}
	for j := range wCol {
		wCol[j] = t.colAddr(t.sW, j, tileStride)
	}
	return aRow, wCol
}

// computeQ4K: isum = Σ_j sc_j·s_j, msum = Σ_j m_j·bs_j; t = d·isum − dmin·msum, acc += t·d8.
func (t *dp4aTile) computeQ4K(acc *[tileRows][tileCols]reg) {
	k := t.k
	aRow, wCol := t.tileAddrs()
	var sCol [tileCols]reg
	for j := range sCol {
		sCol[j] = t.colAddr(t.sSc, j, tileScale)
	}
	var isum, msum [tileRows][tileCols]reg
	for i := range isum {
		for j := range isum[i] {
			isum[i][j], msum[i][j] = k.whole(0), k.whole(0)
		}
	}
	for sb := range 8 {
		var sc [tileCols]reg
		for j := range sc {
			sc[j] = k.lds(sCol[j], sb)
		}
		t.subDots(aRow, wCol, sb, 8, func(i, j int, s reg) {
			k.e("mad.lo.s32", isum[i][j], sc[j], s, isum[i][j])
		})
	}
	for sb := range 8 {
		var m [tileCols]reg
		for j := range m {
			m[j] = k.lds(sCol[j], 8+sb)
		}
		for i := range aRow {
			bs := k.lds(t.rowAddr(t.sB, i, 8), sb)
			for j := range m {
				k.e("mad.lo.s32", msum[i][j], m[j], bs, msum[i][j])
			}
		}
	}
	for j := range wCol {
		d := k.ldsF(t.colAddr(t.sD, j, 1), 0)
		dm := k.ldsF(t.colAddr(t.sDm, j, 1), 0)
		for i := range aRow {
			d8 := k.ldsF(t.rowAddr(t.sD8, i, 1), 0)
			x, y, v := k.f(), k.f(), k.f()
			k.e("mul.rn.f32", x, d, k.toFloat(isum[i][j]))
			k.e("mul.rn.f32", y, dm, k.toFloat(msum[i][j]))
			k.e("sub.rn.f32", v, x, y)
			k.e("mul.rn.f32", v, v, d8)
			k.e("add.rn.f32", acc[i][j], acc[i][j], v)
		}
	}
}

// computeQ6K: I = Σ_s sc_s·Σ (q6 − 32)·q8; t = d·I, acc += t·d8.
func (t *dp4aTile) computeQ6K(acc *[tileRows][tileCols]reg) {
	k := t.k
	aRow, wCol := t.tileAddrs()
	var sCol [tileCols]reg
	for j := range sCol {
		sCol[j] = t.colAddr(t.sSc, j, tileScale)
	}
	var I [tileRows][tileCols]reg
	for i := range I {
		for j := range I[i] {
			I[i][j] = k.whole(0)
		}
	}
	for sb := range 16 {
		var sc [tileCols]reg
		for j := range sc {
			sc[j] = k.lds(sCol[j], sb)
		}
		t.subDots(aRow, wCol, sb, 4, func(i, j int, s reg) {
			k.e("mad.lo.s32", I[i][j], sc[j], s, I[i][j])
		})
	}
	for j := range wCol {
		d := k.ldsF(t.colAddr(t.sD, j, 1), 0)
		for i := range aRow {
			d8 := k.ldsF(t.rowAddr(t.sD8, i, 1), 0)
			v := k.f()
			k.e("mul.rn.f32", v, d, k.toFloat(I[i][j]))
			k.e("mul.rn.f32", v, v, d8)
			k.e("add.rn.f32", acc[i][j], acc[i][j], v)
		}
	}
}

// computeQ4: dilimin sekiz bloğu sırayla (K'nın dışına düşen bloklar atlanır, bütün blokta aynı dal):
// isum = Σ (kod − 8)·q8, t = d_w·d_x, acc += t·isum (dotQ4Q8Go'nun sırası).
func (t *dp4aTile) computeQ4(acc *[tileRows][tileCols]reg, b reg) {
	k := t.k
	aRow, wCol := t.tileAddrs()
	end := k.label()
	for sb := range 8 {
		bi, p := k.r(), k.p()
		k.e("mad.lo.s32", bi, b, 8, k.whole(sb))
		k.e("setp.ge.u32", p, bi, t.nb)
		k.jumpIf(p, end)
		var dw [tileCols]reg
		for j := range dw {
			dw[j] = k.ldsF(t.colAddr(t.sD, j, 8), sb)
		}
		var dx [tileRows]reg
		for i := range dx {
			dx[i] = k.ldsF(t.rowAddr(t.sD8, i, 8), sb)
		}
		t.subDots(aRow, wCol, sb, 8, func(i, j int, s reg) {
			v := k.f()
			k.e("mul.rn.f32", v, dw[j], dx[i])
			k.e("mul.rn.f32", v, v, k.toFloat(s))
			k.e("add.rn.f32", acc[i][j], acc[i][j], v)
		})
	}
	k.put(end)
}

// store c[row, col] = c + acc (satır ve sütun içerideyse, satır sonluysa; sonlu olmayan satır float yolundadır).
func (t *dp4aTile) store(acc *[tileRows][tileCols]reg) {
	k := t.k
	for i := range acc {
		row := k.r()
		k.e("add.s32", row, t.row0, t.rg)
		k.e("add.s32", row, row, 8*i)
		in, finite := k.p(), k.p()
		k.e("setp.lt.u32", in, row, t.M)
		f := k.r()
		k.e("ld.global.u32", f, mem(k.address(t.pfin, t.clamp(row, k.whole(0), t.M), 4), 0))
		k.e("setp.eq.u32", finite, f, 0)
		k.e("and.pred", in, in, finite)
		for j := range acc[i] {
			col := k.r()
			k.e("add.s32", col, t.col0, t.cg)
			k.e("add.s32", col, col, 16*j)
			p := k.p()
			k.e("setp.lt.u32", p, col, t.N)
			k.e("and.pred", p, p, in)
			skip := k.label()
			k.jumpUnless(p, skip)
			idx := k.r()
			k.e("mad.lo.s32", idx, row, t.N, col)
			ad := k.address(t.pc, idx, 4)
			old := k.f()
			k.e("ld.global.f32", old, mem(ad, 0))
			k.e("add.rn.f32", old, old, acc[i][j])
			k.e("st.global.f32", mem(ad, 0), old)
			k.put(skip)
		}
	}
}

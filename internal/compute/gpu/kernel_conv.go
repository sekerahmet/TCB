package gpu

import "github.com/sekerahmet/TCB/internal/compute/tensor"

// ============================================================================
// Bir boyutlu evrişimin açması ve katlaması (15.23c, 15.41)
// ============================================================================
//
// Açma girdinin pencerelerini [g, B · çıkış, Cin / g · K] satırlarına kopyalar, dolgu sıfırdır;
// katlama açmanın türevidir: her girdi öğesi kendine düşen satır öğelerini çıkış sırasıyla
// (t artan) toplar. İkisi de tensör paketinin openRows ve foldRows'uyla aynı sıradadır, atomik
// toplama yoktur: bitler işlemciyle ve koşudan koşuya aynıdır. Eleman başına iş parçacığı.

const convBlock = 256

// convParams çekirdeklerin ortak boy parametreleridir (sıra launch'ınkiyle aynı).
type convParams struct{ n, batch, cin, l, k, frames, stride, pad, dil, groups reg }

// convHead parametreleri bildirir; e ≥ n olan iş parçacığı bitişe atlar.
func (k *emitter) convHead(end string) (e reg, c convParams) {
	c = convParams{k.u32p("n"), k.u32p("b"), k.u32p("cin"), k.u32p("l"), k.u32p("k"), k.u32p("cikis"),
		k.u32p("adim"), k.u32p("dolgu"), k.u32p("genisleme"), k.u32p("grup")}
	e = k.globalIndex()
	p := k.p()
	k.e("setp.ge.u32", p, e, c.n)
	k.jumpIf(p, end)
	return e, c
}

// divmod q = a / b, r = a % b (işaretsiz).
func (k *emitter) divmod(a, b reg) (q, r reg) {
	q, r = k.r(), k.r()
	k.e("div.u32", q, a, b)
	k.e("rem.u32", r, a, b)
	return q, r
}

// unfoldKernel satırlar[e] = x[b, gi·cg + c, t·adım − dolgu + j·genişleme] (dışarıda 0);
// e = ((gi·B + b)·çıkış + t)·(cg·K) + c·K + j.
func unfoldKernel() *emitter {
	k := newEmitter("acma_1b")
	x := k.pointer("x")
	cols := k.written("satirlar")
	end := k.label()
	e, c := k.convHead(end)
	cg, width, bf := k.r(), k.r(), k.r()
	k.e("div.u32", cg, c.cin, c.groups)
	k.e("mul.lo.s32", width, cg, c.k)
	k.e("mul.lo.s32", bf, c.batch, c.frames)
	u, r := k.divmod(e, width)
	ch, j := k.divmod(r, c.k)
	gi, rest := k.divmod(u, bf)
	b, t := k.divmod(rest, c.frames)
	pos := k.r()
	k.e("mul.lo.s32", pos, t, c.stride)
	k.e("sub.s32", pos, pos, c.pad)
	k.e("mad.lo.s32", pos, j, c.dil, pos)
	v := k.constant(0)
	write := k.label()
	p := k.p()
	k.e("setp.lt.s32", p, pos, 0)
	k.jumpIf(p, write)
	k.e("setp.ge.s32", p, pos, c.l)
	k.jumpIf(p, write)
	row := k.r()
	k.e("mad.lo.s32", row, gi, cg, ch)
	k.e("mad.lo.s32", row, b, c.cin, row)
	k.e("mad.lo.s32", row, row, c.l, pos)
	k.e("ld.global.f32", v, mem(k.address(x, row, 4), 0))
	k.put(write)
	k.store(tensor.Float32, k.address(cols, e, 4), v)
	k.put(end)
	return k
}

// foldKernel dx[b, ci, p] = Σ_t g[(gi·B + b)·çıkış + t, c·K + j] (t artan; t·adım + j·genişleme =
// p + dolgu, 0 ≤ j < K). t yalnız j'nin aralığa düştüğü [t0, t1]'de dolaşılır: t0 = ⌈(p + dolgu −
// (K−1)·genişleme) / adım⌉ (en az 0), t1 = ⌊(p + dolgu) / adım⌋ (en çok çıkış − 1).
func foldKernel() *emitter {
	k := newEmitter("katlama_1b")
	g := k.pointer("g")
	dx := k.written("dx")
	end := k.label()
	e, c := k.convHead(end)
	cg, width := k.r(), k.r()
	k.e("div.u32", cg, c.cin, c.groups)
	k.e("mul.lo.s32", width, cg, c.k)
	r, pos := k.divmod(e, c.l)
	b, ci := k.divmod(r, c.cin)
	gi, ch := k.divmod(ci, cg)
	q, span, lo, hi, last := k.r(), k.r(), k.r(), k.r(), k.r()
	k.e("add.s32", q, pos, c.pad)
	k.e("add.s32", span, c.k, -1)
	k.e("mul.lo.s32", span, span, c.dil)
	k.e("sub.s32", lo, q, span)
	k.e("max.s32", lo, lo, 0)
	k.e("add.s32", lo, lo, c.stride)
	k.e("add.s32", lo, lo, -1)
	k.e("div.u32", lo, lo, c.stride)
	k.e("div.u32", hi, q, c.stride)
	k.e("add.s32", last, c.frames, -1)
	k.e("min.u32", hi, hi, last)
	k.e("add.s32", hi, hi, 1)
	base, colOff := k.r(), k.r()
	k.e("mul.lo.s32", base, gi, c.batch)
	k.e("add.s32", base, base, b)
	k.e("mul.lo.s32", base, base, c.frames)
	k.e("mul.lo.s32", colOff, ch, c.k)
	acc := k.constant(0)
	t := k.r()
	k.e("mov.u32", t, lo)
	k.whileLess("u32", t, hi, func() {
		rem, jq, jr := k.r(), k.r(), k.r()
		k.e("mul.lo.s32", rem, t, c.stride)
		k.e("sub.s32", rem, q, rem)
		k.e("div.u32", jq, rem, c.dil)
		k.e("rem.u32", jr, rem, c.dil)
		next := k.label()
		p := k.p()
		k.e("setp.ne.u32", p, jr, 0)
		k.jumpIf(p, next)
		at := k.r()
		k.e("add.s32", at, base, t)
		k.e("mad.lo.s32", at, at, width, colOff)
		k.e("add.s32", at, at, jq)
		k.e("add.rn.f32", acc, acc, k.load(tensor.Float32, k.address(g, at, 4)))
		k.put(next)
		k.e("add.s32", t, t, 1)
	})
	k.store(tensor.Float32, k.address(dx, e, 4), acc)
	k.put(end)
	return k
}

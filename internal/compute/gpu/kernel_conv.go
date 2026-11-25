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
//
// Konum (t·adım − dolgu + j·genişleme) dolgulu uzunluk 2³¹'in altındayken 32 bitte hesaplanır; üstünde
// aynı gövdenin 64 bitlik eşi koşar (acma_1b_64, katlama_1b_64; adım, dolgu ve genişleme 64 bitlik
// parametredir). Öğe numaraları ve kare sayısı iki hesapta da 32 bittedir.

const convBlock = 256

// convParams çekirdeklerin ortak boy parametreleridir (sıra launch'ınkiyle aynı); stride, pad ve dil
// konumun genişliğindedir.
type convParams struct{ n, batch, cin, l, k, frames, stride, pad, dil, groups reg }

// convHead parametreleri bildirir; e ≥ n olan iş parçacığı bitişe atlar.
func (k *emitter) convHead(end string, wide bool) (e reg, c convParams) {
	c = convParams{n: k.u32p("n"), batch: k.u32p("b"), cin: k.u32p("cin"), l: k.u32p("l"), k: k.u32p("k"), frames: k.u32p("cikis")}
	setting := k.u32p
	if wide {
		setting = k.u64p
	}
	c.stride, c.pad, c.dil = setting("adim"), setting("dolgu"), setting("genisleme")
	c.groups = k.u32p("grup")
	e = k.globalIndex()
	p := k.p()
	k.e("setp.ge.u32", p, e, c.n)
	k.jumpIf(p, end)
	return e, c
}

// convWidth konum hesabının genişliğidir: 32 bit ya da (convWide, backend_conv.go) 64 bit.
type convWidth struct {
	k    *emitter
	wide bool
}

func (w convWidth) reg() reg {
	if w.wide {
		return w.k.rd()
	}
	return w.k.r()
}

func (w convWidth) bits(sign string) string {
	if w.wide {
		return sign + "64"
	}
	return sign + "32"
}

func (w convWidth) s() string { return w.bits("s") }
func (w convWidth) u() string { return w.bits("u") }

// up 32 bitlik işaretsiz değeri konum genişliğine çevirir (dar hesapta değer kendisidir).
func (w convWidth) up(r reg) reg {
	if !w.wide {
		return r
	}
	x := w.k.rd()
	w.k.e("cvt.u64.u32", x, r)
	return x
}

// down 32 bite sığan konumu 32 bite çevirir (dar hesapta değer kendisidir).
func (w convWidth) down(r reg) reg {
	if !w.wide {
		return r
	}
	x := w.k.r()
	w.k.e("cvt.u32.u64", x, r)
	return x
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
func unfoldKernel(wide bool) *emitter {
	k := newEmitter("acma_1b")
	w := convWidth{k, wide}
	x := k.pointer("x")
	cols := k.written("satirlar")
	end := k.label()
	e, c := k.convHead(end, wide)
	cg, width, bf := k.r(), k.r(), k.r()
	k.e("div.u32", cg, c.cin, c.groups)
	k.e("mul.lo.s32", width, cg, c.k)
	k.e("mul.lo.s32", bf, c.batch, c.frames)
	u, r := k.divmod(e, width)
	ch, j := k.divmod(r, c.k)
	gi, rest := k.divmod(u, bf)
	b, t := k.divmod(rest, c.frames)
	pos, t, j, l := w.reg(), w.up(t), w.up(j), w.up(c.l)
	k.e("mul.lo."+w.s(), pos, t, c.stride)
	k.e("sub."+w.s(), pos, pos, c.pad)
	k.e("mad.lo."+w.s(), pos, j, c.dil, pos)
	v := k.constant(0)
	write := k.label()
	p := k.p()
	k.e("setp.lt."+w.s(), p, pos, 0)
	k.jumpIf(p, write)
	k.e("setp.ge."+w.s(), p, pos, l)
	k.jumpIf(p, write)
	row := k.r()
	k.e("mad.lo.s32", row, gi, cg, ch)
	k.e("mad.lo.s32", row, b, c.cin, row)
	k.e("mad.lo.s32", row, row, c.l, w.down(pos))
	k.e("ld.global.f32", v, mem(k.address(x, row, 4), 0))
	k.put(write)
	k.store(tensor.Float32, k.address(cols, e, 4), v)
	k.put(end)
	return k
}

// foldKernel dx[b, ci, p] = Σ_t g[(gi·B + b)·çıkış + t, c·K + j] (t artan; t·adım + j·genişleme =
// p + dolgu, 0 ≤ j < K). t yalnız j'nin aralığa düştüğü [t0, t1]'de dolaşılır: t0 = ⌈(p + dolgu −
// (K−1)·genişleme) / adım⌉ (en az 0), t1 = ⌊(p + dolgu) / adım⌋ (en çok çıkış − 1).
func foldKernel(wide bool) *emitter {
	k := newEmitter("katlama_1b")
	w := convWidth{k, wide}
	g := k.pointer("g")
	dx := k.written("dx")
	end := k.label()
	e, c := k.convHead(end, wide)
	cg, width := k.r(), k.r()
	k.e("div.u32", cg, c.cin, c.groups)
	k.e("mul.lo.s32", width, cg, c.k)
	r, pos := k.divmod(e, c.l)
	b, ci := k.divmod(r, c.cin)
	gi, ch := k.divmod(ci, cg)
	q, span, loW, hiW, last := w.reg(), w.reg(), w.reg(), w.reg(), w.reg()
	k.e("add."+w.s(), q, w.up(pos), c.pad)
	k.e("add."+w.s(), span, w.up(c.k), -1)
	k.e("mul.lo."+w.s(), span, span, c.dil)
	k.e("sub."+w.s(), loW, q, span)
	k.e("max."+w.s(), loW, loW, 0)
	k.e("add."+w.s(), loW, loW, c.stride)
	k.e("add."+w.s(), loW, loW, -1)
	k.e("div."+w.u(), loW, loW, c.stride)
	k.e("div."+w.u(), hiW, q, c.stride)
	k.e("add."+w.s(), last, w.up(c.frames), -1)
	k.e("min."+w.u(), hiW, hiW, last)
	k.e("add."+w.s(), hiW, hiW, 1)
	lo, hi := w.down(loW), w.down(hiW)
	base, colOff := k.r(), k.r()
	k.e("mul.lo.s32", base, gi, c.batch)
	k.e("add.s32", base, base, b)
	k.e("mul.lo.s32", base, base, c.frames)
	k.e("mul.lo.s32", colOff, ch, c.k)
	acc := k.constant(0)
	t := k.r()
	k.e("mov.u32", t, lo)
	k.whileLess("u32", t, hi, func() {
		rem, jq, jr := w.reg(), w.reg(), w.reg()
		k.e("mul.lo."+w.s(), rem, w.up(t), c.stride)
		k.e("sub."+w.s(), rem, q, rem)
		k.e("div."+w.u(), jq, rem, c.dil)
		k.e("rem."+w.u(), jr, rem, c.dil)
		next := k.label()
		p := k.p()
		k.e("setp.ne."+w.u(), p, jr, 0)
		k.jumpIf(p, next)
		at := k.r()
		k.e("add.s32", at, base, t)
		k.e("mad.lo.s32", at, at, width, colOff)
		k.e("add.s32", at, at, w.down(jq))
		k.e("add.rn.f32", acc, acc, k.load(tensor.Float32, k.address(g, at, 4)))
		k.put(next)
		k.e("add.s32", t, t, 1)
	})
	k.store(tensor.Float32, k.address(dx, e, 4), acc)
	k.put(end)
	return k
}

package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Dikkat: matris çarpımı yolu (birleşik çekirdeğin paylaşımlı belleğine sığmayan baş
// boyları için; birleşik yol cekirdek_flash*.go ve arkauc_dikkat.go'da)
// ============================================================================
//
// GQA'da (H = G·Hkv) bir anahtar/değer başını paylaşan G sorgu başı bitişiktir: q,
// [Batch·Hkv, G·Tq, Dh] matrisleri olarak görülür ve her (b, kv) için tek çarpım yapılır.
//
//	S = q·kᵀ            (toplu çarpım, puanlar [Batch·H·Tq, Tk])
//	P = softmax(S·ölçek + maske), lse = m + log Σ  (satır çekirdeği; yerinde)
//	O = P·v             (toplu çarpım)
//
// Geri yönde P, lse'den yeniden hesaplanır: dV += Pᵀ·dO, dP = dO·vᵀ,
// dS = P·(dP − Σ dO·O)·ölçek, dQ += dS·k, dK += dSᵀ·q. Hiçbir anahtara bakamayan satırda
// P = 0 ve lse = +sonsuz'dur (çıktı ve gradyan 0). Bellek Tq·Tk ile büyür.

// dikkatSinirlari sorgu satırı i için bakılabilen anahtar aralığı [lo, hi) (CPU
// AttentionParams.Limits ile aynı kural); nedensel ve pencere çalışma anı parametresidir.
func (k *yazici) dikkatSinirlari(i, Tq, Tk, nedensel, pencere yazmac) (lo, hi yazmac) {
	c := k.r()
	k.e("add.s32", c, i, Tk)
	k.e("sub.s32", c, c, Tq)
	lo, hi = k.tam(0), k.r()
	k.e("mov.u32", hi, Tk)
	nd, pv := k.p(), k.p()
	k.e("setp.ne.u32", nd, nedensel, 0)
	k.e("setp.gt.s32", pv, pencere, 0)
	c1 := k.r()
	k.e("add.s32", c1, c, 1)
	h1 := k.r()
	k.e("min.s32", h1, hi, c1)
	k.e("selp.b32", hi, h1, hi, nd)
	l1 := k.r()
	k.e("sub.s32", l1, c1, pencere)
	k.e("max.s32", l1, lo, l1)
	k.e("selp.b32", lo, l1, lo, pv)
	// Nedensel olmayan pencere iki yönlüdür: hi = min(hi, c + W).
	cw := k.r()
	k.e("add.s32", cw, c, pencere)
	k.e("min.s32", cw, hi, cw)
	ikiYon := k.p()
	k.e("not.pred", nd, nd)
	k.e("and.pred", ikiYon, pv, nd)
	k.e("selp.b32", hi, cw, hi, ikiYon)
	k.e("max.s32", hi, hi, lo)
	return lo, hi
}

// dikkatGorunur anahtar jj'nin görünürlüğü (sınırlar ve ek maske).
func (k *yazici) dikkatGorunur(jj, lo, hi, maskeTaban yazmac, md, msK yazmac, maske bool) yazmac {
	p1, p2 := k.p(), k.p()
	k.e("setp.ge.s32", p1, jj, lo)
	k.e("setp.lt.s32", p2, jj, hi)
	k.e("and.pred", p1, p1, p2)
	if maske {
		o := k.r()
		k.e("mad.lo.s32", o, jj, msK, maskeTaban)
		mv := k.f()
		k.e("mov.f32", mv, float32(0))
		k.eger(p1, "ld.global.f32", mv, bel(k.adresS(md, o, 4), 0))
		p3 := k.p()
		k.e("setp.ne.f32", p3, mv, float32(0))
		k.e("and.pred", p1, p1, p3)
	}
	return p1
}

// dikkatSatir puan satırı çekirdeğinin ortak yazmaçları.
type dikkatSatir struct {
	k                  *yazici
	s, lse, md         yazmac
	tid, R, taban, bir yazmac
	lo, hi, mt, msK    yazmac
	olcek              yazmac
	B                  int
	maske              bool
}

// puan S·ölçek.
func (d *dikkatSatir) puan(ofs yazmac) yazmac {
	v := d.k.yukle(tensor.Float32, d.k.satirAdresi(d.s, ofs))
	d.k.e("mul.rn.f32", v, v, d.olcek)
	return v
}

func (d *dikkatSatir) gorunur(jj yazmac) yazmac {
	return d.k.dikkatGorunur(jj, d.lo, d.hi, d.mt, d.md, d.msK, d.maske)
}

// dikkatSatirCekirdegi puan satırları üzerinde yerinde çalışır. ileri: P = softmax ve
// lse yazılır; değilse (geri) P = görünür ? e^{S·ölçek − lse} : 0.
func dikkatSatirCekirdegi(B int, maske, ileri bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("dikkat_satir_%d_%v_%v", B, maske, ileri))
		d := &dikkatSatir{k: k, B: B, maske: maske}
		d.s = k.yazilan("s")
		if ileri {
			d.lse = k.yazilan("lse")
		} else {
			d.lse = k.gosterici("lse")
		}
		d.md = k.gosterici("maske")
		mo := k.gosterici("maske_ofset")
		Tq, Tk := k.u32p("tq"), k.u32p("tk")
		ned, pen := k.u32p("nedensel"), k.u32p("pencere")
		msQ := k.u32p("msq")
		d.msK = k.u32p("msk")
		d.olcek = k.f32p("olcek")
		var sm yazmac
		if ileri {
			sm = k.ortak("sm", 4*B)
		}
		d.tid, d.R = k.ozel("%tid.x"), k.ozel("%ctaid.x")
		i, bh := k.r(), k.r()
		k.e("rem.u32", i, d.R, Tq)
		k.e("div.u32", bh, d.R, Tq)
		d.lo, d.hi = k.dikkatSinirlari(i, Tq, Tk, ned, pen)
		d.mt = k.tam(0)
		if maske {
			k.e("ld.global.u32", d.mt, bel(k.adres(mo, bh, 4), 0))
			k.e("mad.lo.s32", d.mt, i, msQ, d.mt)
		}
		d.taban = k.r()
		k.e("mul.lo.s32", d.taban, d.R, Tk)
		d.bir = k.tam(1)
		if ileri {
			d.ileri(Tk, sm)
		} else {
			d.geri(Tk)
		}
		return k
	}
}

// geri P = görünür ? e^{S·ölçek − lse} : 0.
func (d *dikkatSatir) geri(Tk yazmac) {
	k := d.k
	l := k.yukle(tensor.Float32, k.adres(d.lse, d.R, 4))
	k.satirDongu(d.tid, Tk, d.bir, d.taban, d.B, func(ofs, jj yazmac) {
		v := d.puan(ofs)
		k.e("sub.rn.f32", v, v, l)
		e := k.ustel(v)
		k.e("selp.f32", e, e, float32(0), d.gorunur(jj))
		k.sakla(tensor.Float32, k.satirAdresi(d.s, ofs), e)
	})
}

// ileri maskeli softmax ve lse (boş satırda P = 0, lse = +sonsuz).
func (d *dikkatSatir) ileri(Tk, sm yazmac) {
	k := d.k
	eksi := float32(math.Inf(-1))
	m := k.sabit(eksi)
	k.satirDongu(d.tid, Tk, d.bir, d.taban, d.B, func(ofs, jj yazmac) {
		v := d.puan(ofs)
		k.e("selp.f32", v, v, eksi, d.gorunur(jj))
		b := k.p()
		k.e("setp.gt.f32", b, v, m)
		k.e("selp.f32", m, v, m, b)
	})
	m = k.blokIndirge(m, sm, d.tid, d.B, k.enBuyuk)
	top := k.sabit(0)
	k.satirDongu(d.tid, Tk, d.bir, d.taban, d.B, func(ofs, jj yazmac) {
		v := d.puan(ofs)
		k.e("sub.rn.f32", v, v, m)
		e := k.ustel(v)
		k.e("selp.f32", e, e, float32(0), d.gorunur(jj))
		k.e("add.rn.f32", top, top, e)
		k.sakla(tensor.Float32, k.satirAdresi(d.s, ofs), e)
	})
	top = k.blokIndirge(top, sm, d.tid, d.B, k.topla)
	bos := k.p()
	k.e("setp.eq.f32", bos, m, eksi)
	ters := k.f()
	k.e("rcp.rn.f32", ters, top)
	k.e("selp.f32", ters, float32(0), ters, bos)
	k.satirDongu(d.tid, Tk, d.bir, d.taban, d.B, func(ofs, _ yazmac) {
		ad := k.satirAdresi(d.s, ofs)
		v := k.yukle(tensor.Float32, ad)
		k.e("mul.rn.f32", v, v, ters)
		k.sakla(tensor.Float32, ad, v)
	})
	son := k.etiket()
	k.gitEger(k.tidSifirDegil(d.tid), son)
	l := k.logaritma(top)
	k.e("add.rn.f32", l, l, m)
	k.e("selp.f32", l, float32(math.Inf(1)), l, bos)
	k.sakla(tensor.Float32, k.adres(d.lse, d.R, 4), l)
	k.koy(son)
}

// dikkatGeriCekirdegi dS = P·(dP − Σ_d dO·O)·ölçek; dS, dP tamponuna yerinde yazılır.
func dikkatGeriCekirdegi(B int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("dikkat_geri_%d", B))
		dp := k.yazilan("dp")
		pp := k.gosterici("p")
		do := k.gosterici("do")
		po := k.gosterici("o")
		Tk, Dv := k.u32p("tk"), k.u32p("dv")
		olcek := k.f32p("olcek")
		sm := k.ortak("sm", 4*B)
		tid := k.ozel("%tid.x")
		R := k.ozel("%ctaid.x")
		bir := k.tam(1)
		tv := k.r()
		k.e("mul.lo.s32", tv, R, Dv)
		d := k.sabit(0)
		k.satirDongu(tid, Dv, bir, tv, B, func(ofs, _ yazmac) {
			a := k.yukle(tensor.Float32, k.satirAdresi(do, ofs))
			b := k.yukle(tensor.Float32, k.satirAdresi(po, ofs))
			k.e("fma.rn.f32", d, a, b, d)
		})
		d = k.blokIndirge(d, sm, tid, B, k.topla)
		ts := k.r()
		k.e("mul.lo.s32", ts, R, Tk)
		k.satirDongu(tid, Tk, bir, ts, B, func(ofs, _ yazmac) {
			ad := k.satirAdresi(dp, ofs)
			v := k.yukle(tensor.Float32, ad)
			k.e("sub.rn.f32", v, v, d)
			k.e("mul.rn.f32", v, v, k.yukle(tensor.Float32, k.satirAdresi(pp, ofs)))
			k.e("mul.rn.f32", v, v, olcek)
			k.sakla(tensor.Float32, ad, v)
		})
		return k
	}
}

// dikkatSatirlari puan satırı çekirdeğini başlatır.
func (g *arkaUc) dikkatSatirlari(s, lse *depo, maske *depo, mo *depo, p tensor.AttentionParams, ileri bool) {
	R := p.Batch * p.H * p.Tq
	B := ikiKuvvet(p.Tk, 32, 256)
	c := g.cekirdekAl(fmt.Sprintf("dikkat_satir:%d:%v:%v", B, maske != nil, ileri), dikkatSatirCekirdegi(B, maske != nil, ileri))
	pr := g.param().yaz(s, 0)
	if ileri {
		pr.yaz(lse, 0)
	} else {
		pr.oku(lse, 0)
	}
	ned := 0
	if p.Causal {
		ned = 1
	}
	pr.okuYa(maske).okuYa(mo).u32(p.Tq).u32(p.Tk).u32(ned).u32(p.Window).u32(p.MaskStrideQ).u32(p.MaskStrideK).f32(p.Scale)
	g.baslat(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(B), 1, 1}, pr)
}

// dikkatHazirla ortak denetimler ve maske ofsetlerinin yüklenmesi.
func (g *arkaUc) dikkatHazirla(gt *gecici, mask tensor.Storage, p tensor.AttentionParams) (md, mo *depo) {
	if p.H%max(p.Hkv, 1) != 0 {
		panic(fmt.Sprintf("tensor: GPU: dikkat baş sayıları uyumsuz (%d, %d)", p.H, p.Hkv))
	}
	if mask != nil {
		md = gt.okuD(mask)
		boyDenetle(md.n) // maske dizini 32 bit
		mo = gt.yukleTam(p.MaskOffsets)
	}
	return md, mo
}

// dikkatMatris dikkatin matris çarpımı yolu (ileri); lse'yi doldurur.
func (g *arkaUc) dikkatMatris(gt *gecici, od, lse *depo, q, k, v tensor.View, md, mo *depo, p tensor.AttentionParams) {
	dikkatBoyDenetle(p, false)
	R := p.Batch * p.H * p.Tq
	G, Z := p.H/p.Hkv, p.Batch*p.Hkv
	qv, kv, vv := gt.oku(q), gt.oku(k), gt.oku(v)
	s := gt.tampon(R*p.Tk, false)
	sv := tensor.View{S: s}
	g.gemm(Z, G*p.Tq, p.Tk, p.Dh, qv, false, kv, true, s, 0, true)
	g.dikkatSatirlari(s, lse, md, mo, p, true)
	g.gemm(Z, G*p.Tq, p.Dv, p.Tk, sv, false, vv, false, od, 0, true)
}

// dikkatMatrisGeri dikkatin matris çarpımı yolu (geri); gradyanlar eklenir.
func (g *arkaUc) dikkatMatrisGeri(gt *gecici, gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, md, mo *depo, o, lse tensor.Storage, p tensor.AttentionParams) {
	dikkatBoyDenetle(p, false)
	R := p.Batch * p.H * p.Tq
	Z, Mq := p.Batch*p.Hkv, p.H/p.Hkv*p.Tq
	qv, kv, vv := gt.oku(q), gt.oku(k), gt.oku(v)
	dO := tensor.View{S: gt.okuD(gs)}
	s := gt.tampon(R*p.Tk, false)
	g.gemm(Z, Mq, p.Tk, p.Dh, qv, false, kv, true, s, 0, true)
	g.dikkatSatirlari(s, g.depoOf(lse), md, mo, p, false)
	if gv != nil {
		dv, son := gt.yaz(gv, true)
		g.gemm(Z, p.Tk, p.Dv, Mq, tensor.View{S: s}, true, dO, false, dv, 0, false)
		son()
	}
	if gq == nil && gk == nil {
		return
	}
	dp := gt.tampon(R*p.Tk, false)
	g.gemm(Z, Mq, p.Tk, p.Dv, dO, false, vv, true, dp, 0, true)
	B := ikiKuvvet(max(p.Tk, p.Dv), 32, 256)
	od := gt.okuD(o) // zincirden önce: 16 bitlik o açılırken kopya çekirdeği başlatılır
	c := g.cekirdekAl(fmt.Sprintf("dikkat_geri:%d", B), dikkatGeriCekirdegi(B))
	g.baslat(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(B), 1, 1},
		g.param().yaz(dp, 0).oku(s, 0).okuV(dO).oku(od, 0).u32(p.Tk).u32(p.Dv).f32(p.Scale))
	dsv := tensor.View{S: dp}
	if gq != nil {
		dq, son := gt.yaz(gq, true)
		g.gemm(Z, Mq, p.Dh, p.Tk, dsv, false, kv, false, dq, 0, false)
		son()
	}
	if gk != nil {
		dk, son := gt.yaz(gk, true)
		g.gemm(Z, p.Tk, p.Dh, Mq, dsv, true, qv, false, dk, 0, false)
		son()
	}
}

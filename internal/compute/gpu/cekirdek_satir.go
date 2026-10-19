package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Satır çekirdekleri: softmax, log_softmax (çapraz entropi: cekirdek_ce.go)
// ============================================================================
//
// Her satıra bir blok düşer; iş parçacıkları satırın elemanlarını B adımla gezer, satır
// toplamları ve en büyükleri blok içinde ağaç biçiminde (sabit sıra) indirgenir.
// Satır (dış, iç) çiftidir: eleman (o, l, i) bitişik düzende (o·L + l)·I + i'dedir; son
// eksen için I = 1.

// enBuyuk birlestir işlevi: max.f32 (NaN'ı yok sayar; satır en büyükleri NaN olmaz).
func (k *yazici) enBuyuk(a, b yazmac) yazmac {
	r := k.f()
	k.e("max.f32", r, a, b)
	return r
}

// satirDongu l = tid; l < L; l += B döngüsünü kurar; govde, elemanın bayt ofsetini
// ((taban + l·I)·4) alır ve kendi göstericisine ekler (satirAdresi).
func (k *yazici) satirDongu(tid, L, I, taban yazmac, B int, govde func(adr yazmac, l yazmac)) {
	l := k.r()
	k.e("mov.u32", l, tid)
	bas, son := k.etiket(), k.etiket()
	k.koy(bas)
	p := k.p()
	k.e("setp.ge.u32", p, l, L)
	k.gitEger(p, son)
	o := k.r()
	k.e("mad.lo.s32", o, l, I, taban)
	b := k.rd()
	k.e("mul.wide.u32", b, o, 4)
	govde(b, l)
	k.e("add.s32", l, l, B)
	k.git(bas)
	k.koy(son)
}

// satirAdresi göstericiye satirDongu'nun verdiği bayt ofsetini ekler.
func (k *yazici) satirAdresi(ptr, ofs yazmac) yazmac {
	r := k.rd()
	k.e("add.s64", r, ptr, ofs)
	return r
}

// satirBasi blok başına satırın taban eleman ofsetini verir: o·L·I + i.
func (k *yazici) satirBasi(L, I yazmac) yazmac {
	row := k.ozel("%ctaid.x")
	o, i := k.r(), k.r()
	k.e("div.u32", o, row, I)
	k.e("rem.u32", i, row, I)
	t := k.r()
	k.e("mul.lo.s32", t, o, L)
	k.e("mad.lo.s32", t, t, I, i)
	return t
}

func softmaxCekirdegi(B int, logKip bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("softmax_%d_%v", B, logKip))
		x := k.gosterici("x")
		y := k.yazilan("y")
		L := k.u32p("l")
		I := k.u32p("i")
		sm := k.ortak("sm", 4*B)
		tid := k.ozel("%tid.x")
		taban := k.satirBasi(L, I)
		m := k.sabit(float32(math.Inf(-1)))
		k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
			v := k.yukle(tensor.Float32, k.satirAdresi(x, ofs))
			b := k.p()
			k.e("setp.gt.f32", b, v, m)
			k.e("selp.f32", m, v, m, b)
		})
		m = k.blokIndirge(m, sm, tid, B, k.enBuyuk)
		son, dolu := k.etiket(), k.etiket()
		bos := k.p()
		k.e("setp.ne.f32", bos, m, float32(math.Inf(-1)))
		k.gitEger(bos, dolu)
		// Tamamen −sonsuz satır: softmax 0, log_softmax −sonsuz.
		dolgu := float32(0)
		if logKip {
			dolgu = float32(math.Inf(-1))
		}
		dv := k.sabit(dolgu)
		k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
			k.sakla(tensor.Float32, k.satirAdresi(y, ofs), dv)
		})
		k.git(son)
		k.koy(dolu)
		s := k.sabit(0)
		k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
			v := k.yukle(tensor.Float32, k.satirAdresi(x, ofs))
			k.e("sub.rn.f32", v, v, m)
			k.e("add.rn.f32", s, s, k.ustel(v))
		})
		s = k.blokIndirge(s, sm, tid, B, k.topla)
		carpan := k.f()
		if logKip {
			carpan = k.logaritma(s)
		} else {
			k.e("rcp.rn.f32", carpan, s)
		}
		k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
			v := k.yukle(tensor.Float32, k.satirAdresi(x, ofs))
			k.e("sub.rn.f32", v, v, m)
			r := k.f()
			if logKip {
				k.e("sub.rn.f32", r, v, carpan)
			} else {
				k.e("mul.rn.f32", r, k.ustel(v), carpan)
			}
			k.sakla(tensor.Float32, k.satirAdresi(y, ofs), r)
		})
		k.koy(son)
		return k
	}
}

func softmaxTurevCekirdegi(B int, logKip bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("softmax_turev_%d_%v", B, logKip))
		gx := k.yazilan("gx")
		g := k.gosterici("g")
		y := k.gosterici("y")
		L := k.u32p("l")
		I := k.u32p("i")
		sm := k.ortak("sm", 4*B)
		tid := k.ozel("%tid.x")
		taban := k.satirBasi(L, I)
		son := k.etiket()
		if logKip {
			// Tamamen maskeli satır (bütün çıktılar −sonsuz): gradyan 0.
			dolu := k.sabit(0)
			k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
				v := k.yukle(tensor.Float32, k.satirAdresi(y, ofs))
				b := k.p()
				k.e("setp.ne.f32", b, v, float32(math.Inf(-1)))
				k.e("selp.f32", dolu, float32(1), dolu, b)
			})
			dolu = k.blokIndirge(dolu, sm, tid, B, k.enBuyuk)
			b := k.p()
			k.e("setp.eq.f32", b, dolu, float32(0))
			k.gitEger(b, son)
		}
		s := k.sabit(0)
		k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
			gv := k.yukle(tensor.Float32, k.satirAdresi(g, ofs))
			if logKip {
				k.e("add.rn.f32", s, s, gv)
				return
			}
			yv := k.yukle(tensor.Float32, k.satirAdresi(y, ofs))
			k.e("fma.rn.f32", s, gv, yv, s)
		})
		s = k.blokIndirge(s, sm, tid, B, k.topla)
		k.satirDongu(tid, L, I, taban, B, func(ofs, _ yazmac) {
			gv := k.yukle(tensor.Float32, k.satirAdresi(g, ofs))
			yv := k.yukle(tensor.Float32, k.satirAdresi(y, ofs))
			t := k.f()
			if logKip {
				k.e("mul.rn.f32", t, k.ustel(yv), s)
				k.e("sub.rn.f32", t, gv, t)
			} else {
				k.e("sub.rn.f32", t, gv, s)
				k.e("mul.rn.f32", t, yv, t)
			}
			ha := k.satirAdresi(gx, ofs)
			eski := k.yukle(tensor.Float32, ha)
			k.e("add.rn.f32", eski, eski, t)
			k.sakla(tensor.Float32, ha, eski)
		})
		k.koy(son)
		return k
	}
}

// tidSifirDegil tid ≠ 0 yüklemi.
func (k *yazici) tidSifirDegil(tid yazmac) yazmac {
	p := k.p()
	k.e("setp.ne.u32", p, tid, 0)
	return p
}

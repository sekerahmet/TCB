package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Katman normu ve RMS normu (ileri ve geri)
// ============================================================================
//
// Satır başına bir blok. İleri (CPU lnIleriBlokGo ile aynı formül):
//
//	mu = Σx / D (RMS: 0),  v = Σ(x − mu)² / D,  rs = 1 / √(v + eps),
//	y  = ((x − mu)·rs)·γ + β.
//
// Geri: h = (x − mu)·rs, dh = g·γ, m1 = Σdh / D (RMS: 0), m2 = Σdh·h / D;
// gx += rs·((dh − m1) − h·m2). γ'nın gradyanı için satır başına g·h terimi geçici bir
// tampona yazılır ve satırlar boyunca (belirlenimci) indirgenir; β'nınki g'nin satır
// toplamıdır.

// normAyar norm çekirdeğinin üretim ayarları.
type normAyar struct {
	B                   int
	katman, gamma, beta bool
	xGrad, gammaGrad    bool // geri yön
}

func (a normAyar) anahtar(ad string) string {
	return fmt.Sprintf("%s:%d:%v:%v:%v:%v:%v", ad, a.B, a.katman, a.gamma, a.beta, a.xGrad, a.gammaGrad)
}

func normIleriCekirdegi(a normAyar) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("norm_ileri_%d_%v_%v_%v", a.B, a.katman, a.gamma, a.beta))
		x := k.gosterici("x")
		gm := k.gosterici("gamma")
		bt := k.gosterici("beta")
		y := k.yazilan("y")
		ort := k.yazilan("ortalama")
		rstd := k.yazilan("rstd")
		D := k.u32p("d")
		eps := k.f32p("eps")
		sm := k.ortak("sm", 4*a.B)
		tid := k.ozel("%tid.x")
		r := k.ozel("%ctaid.x")
		bir := k.tam(1)
		taban := k.r()
		k.e("mul.lo.s32", taban, r, D)
		df := k.f()
		k.e("cvt.rn.f32.u32", df, D)
		mu := k.sabit(0)
		if a.katman {
			k.satirDongu(tid, D, bir, taban, a.B, func(ofs, _ yazmac) {
				k.e("add.rn.f32", mu, mu, k.yukle(tensor.Float32, k.satirAdresi(x, ofs)))
			})
			mu = k.blokIndirge(mu, sm, tid, a.B, k.topla)
			k.e("div.rn.f32", mu, mu, df)
		}
		v := k.sabit(0)
		k.satirDongu(tid, D, bir, taban, a.B, func(ofs, _ yazmac) {
			d := k.yukle(tensor.Float32, k.satirAdresi(x, ofs))
			k.e("sub.rn.f32", d, d, mu)
			k.e("fma.rn.f32", v, d, d, v)
		})
		v = k.blokIndirge(v, sm, tid, a.B, k.topla)
		k.e("div.rn.f32", v, v, df)
		k.e("add.rn.f32", v, v, eps)
		k.e("sqrt.rn.f32", v, v)
		rs := k.f()
		k.e("rcp.rn.f32", rs, v)
		k.satirDongu(tid, D, bir, taban, a.B, func(ofs, j yazmac) {
			h := k.yukle(tensor.Float32, k.satirAdresi(x, ofs))
			k.e("sub.rn.f32", h, h, mu)
			k.e("mul.rn.f32", h, h, rs)
			if a.gamma {
				k.e("mul.rn.f32", h, h, k.yukle(tensor.Float32, k.adres(gm, j, 4)))
			}
			if a.beta {
				k.e("add.rn.f32", h, h, k.yukle(tensor.Float32, k.adres(bt, j, 4)))
			}
			k.sakla(tensor.Float32, k.satirAdresi(y, ofs), h)
		})
		son := k.etiket()
		k.gitEger(k.tidSifirDegil(tid), son)
		if a.katman {
			k.sakla(tensor.Float32, k.adres(ort, r, 4), mu)
		}
		k.sakla(tensor.Float32, k.adres(rstd, r, 4), rs)
		k.koy(son)
		return k
	}
}

func normGeriCekirdegi(a normAyar) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("norm_geri_%d_%v_%v_%v_%v", a.B, a.katman, a.gamma, a.xGrad, a.gammaGrad))
		gx := k.yazilan("gx")
		terim := k.yazilan("terim")
		g := k.gosterici("g")
		x := k.gosterici("x")
		gm := k.gosterici("gamma")
		ort := k.gosterici("ortalama")
		rstd := k.gosterici("rstd")
		D := k.u32p("d")
		sm := k.ortak("sm", 4*a.B)
		tid := k.ozel("%tid.x")
		r := k.ozel("%ctaid.x")
		bir := k.tam(1)
		taban := k.r()
		k.e("mul.lo.s32", taban, r, D)
		df := k.f()
		k.e("cvt.rn.f32.u32", df, D)
		m := k.sabit(0)
		if a.katman {
			m = k.yukle(tensor.Float32, k.adres(ort, r, 4))
		}
		rs := k.yukle(tensor.Float32, k.adres(rstd, r, 4))
		h := func(ofs yazmac) yazmac {
			v := k.yukle(tensor.Float32, k.satirAdresi(x, ofs))
			k.e("sub.rn.f32", v, v, m)
			k.e("mul.rn.f32", v, v, rs)
			return v
		}
		dh := func(ofs, j yazmac) yazmac {
			v := k.yukle(tensor.Float32, k.satirAdresi(g, ofs))
			if a.gamma {
				k.e("mul.rn.f32", v, v, k.yukle(tensor.Float32, k.adres(gm, j, 4)))
			}
			return v
		}
		if a.gammaGrad {
			k.satirDongu(tid, D, bir, taban, a.B, func(ofs, _ yazmac) {
				t := k.yukle(tensor.Float32, k.satirAdresi(g, ofs))
				k.e("mul.rn.f32", t, t, h(ofs))
				k.sakla(tensor.Float32, k.satirAdresi(terim, ofs), t)
			})
		}
		if a.xGrad {
			a1, a2 := k.sabit(0), k.sabit(0)
			k.satirDongu(tid, D, bir, taban, a.B, func(ofs, j yazmac) {
				d := dh(ofs, j)
				k.e("add.rn.f32", a1, a1, d)
				k.e("fma.rn.f32", a2, d, h(ofs), a2)
			})
			m1 := k.sabit(0)
			if a.katman {
				m1 = k.blokIndirge(a1, sm, tid, a.B, k.topla)
				k.e("div.rn.f32", m1, m1, df)
			}
			m2 := k.blokIndirge(a2, sm, tid, a.B, k.topla)
			k.e("div.rn.f32", m2, m2, df)
			k.satirDongu(tid, D, bir, taban, a.B, func(ofs, j yazmac) {
				t := dh(ofs, j)
				k.e("sub.rn.f32", t, t, m1)
				hm := k.f()
				k.e("mul.rn.f32", hm, h(ofs), m2)
				k.e("sub.rn.f32", t, t, hm)
				k.e("mul.rn.f32", t, t, rs)
				ha := k.satirAdresi(gx, ofs)
				eski := k.yukle(tensor.Float32, ha)
				k.e("add.rn.f32", eski, eski, t)
				k.sakla(tensor.Float32, ha, eski)
			})
		}
		return k
	}
}

package gpu

import (
	"math"

	"tr_dili/internal/hesap/tensor"
)

// Arka uç: dikkat. Baş boyları birleşik çekirdeğin paylaşımlı belleğine sığıyorsa (Dh, Dv ≤
// 128 civarı; bkz. flashSec) birleşik yol, değilse matris çarpımı yolu (dikkat.go).

// flashUygun birleşik yolun bu tanım için kullanılabildiği ve döşeme ayarı.
func flashUygun(p tensor.AttentionParams, maske, geri bool) (flashAyar, bool) {
	if p.Dh == 0 || p.Dv == 0 || p.H > 65535 || p.Batch > 65535 {
		return flashAyar{}, false
	}
	return flashSec(p.Dh, p.Dv, maske, geri)
}

func (g *arkaUc) Attention(out tensor.Storage, q, k, v tensor.View, mask tensor.Storage, p tensor.AttentionParams) tensor.Storage {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	R := p.Batch * p.H * p.Tq
	lse := g.ayir(R, tensor.Float32, false)
	od, son := gt.yaz(out, false)
	defer son()
	if R == 0 {
		return lse
	}
	if p.Tk == 0 {
		g.doldur(od, 0)
		g.doldur(lse, float32(math.Inf(1)))
		return lse
	}
	md, mo := g.dikkatHazirla(gt, mask, p)
	a, ok := flashUygun(p, md != nil, false)
	if !ok {
		g.dikkatMatris(gt, od, lse, q, k, v, md, mo, p)
		return lse
	}
	qv, kv, vv := gt.oku(q), gt.oku(k), gt.oku(v)
	c := g.cekirdekAl(a.anahtar("dikkat_ileri"), flashIleriCekirdegi(a))
	pr := g.param().okuV(qv).okuV(kv).okuV(vv).yaz(od, 0).yaz(lse, 0)
	g.dikkatOrtak(pr, md, mo, p)
	g.baslat(c, [3]uint32{bloklar(p.Tq, a.Br), uint32(p.H), uint32(p.Batch)}, [3]uint32{flashIs, 1, 1}, pr)
	return lse
}

// dikkatOrtak çekirdeklerin ortak parametreleri (flashOrtak.ortakParametreler sırası).
func (g *arkaUc) dikkatOrtak(pr *paramYazici, md, mo *depo, p tensor.AttentionParams) {
	ned := 0
	if p.Causal {
		ned = 1
	}
	pr.okuYa(md).okuYa(mo).u32(p.Tq).u32(p.Tk).u32(p.H).u32(p.H / p.Hkv).u32(ned).u32(p.Window).
		u32(p.MaskStrideQ).u32(p.MaskStrideK).f32(p.Scale)
}

func (g *arkaUc) AttentionGrad(gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, mask tensor.Storage, o, lse tensor.Storage, p tensor.AttentionParams) {
	g.gir()
	defer g.cik()
	g.dikkatGeri(gq, gk, gv, gs, q, k, v, mask, o, lse, p, [3]bool{})
}

// AttentionGradSet (tensor.AttentionGradSetter) taze[i] doğru olan gradyanların (gq, gk, gv)
// üzerine yazar; diğerlerine ekler.
func (g *arkaUc) AttentionGradSet(gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, mask tensor.Storage, o, lse tensor.Storage, p tensor.AttentionParams, taze [3]bool) {
	g.gir()
	defer g.cik()
	g.dikkatGeri(gq, gk, gv, gs, q, k, v, mask, o, lse, p, taze)
}

func (g *arkaUc) dikkatGeri(gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, mask tensor.Storage, o, lse tensor.Storage, p tensor.AttentionParams, taze [3]bool) {
	gt := g.gecici()
	defer gt.bitir()
	R := p.Batch * p.H * p.Tq
	md, mo := g.dikkatHazirla(gt, mask, p)
	a, ok := flashUygun(p, md != nil, true)
	if !ok || R == 0 || p.Tk == 0 {
		// Taze gradyanlar sıfırlanmamıştır: önce sıfırlanır, sonra eklenir.
		for i, s := range []tensor.Storage{gq, gk, gv} {
			if s != nil && taze[i] {
				g.doldur(g.depoOf(s), 0)
			}
		}
		if R > 0 && p.Tk > 0 && (gq != nil || gk != nil || gv != nil) {
			g.dikkatMatrisGeri(gt, gq, gk, gv, gs, q, k, v, md, mo, o, lse, p)
		}
		return
	}
	kip := func(i int, s tensor.Storage) int {
		switch {
		case s == nil:
			return gradYok
		case taze[i]:
			return gradUzerine
		}
		return gradEkle
	}
	kq, kk, kv := kip(0, gq), kip(1, gk), kip(2, gv)
	qv, kvw, vv := gt.oku(q), gt.oku(k), gt.oku(v)
	dO := gt.okuD(gs)
	od := gt.okuD(o)
	ld := gt.okuD(lse)
	dq, sonQ := gt.yaz(gq, kq == gradEkle)
	dk, sonK := gt.yaz(gk, kk == gradEkle)
	dv, sonV := gt.yaz(gv, kv == gradEkle)
	// D = satır başına Σ dO·O.
	D := gt.tampon(R, false)
	cd := g.cekirdekAl("dikkat_d", flashDCekirdegi)
	iz, bl := duz(R, esBlok)
	g.baslat(cd, iz, bl, g.param().oku(dO, 0).oku(od, 0).yaz(D, 0).u32(R).u32(p.Dv))
	if kq != gradYok {
		c := g.cekirdekAl(a.anahtar("dikkat_geri_q")+string(rune('0'+kq)), flashGeriQCekirdegi(a, kq))
		pr := g.param().okuV(qv).okuV(kvw).okuV(vv).oku(dO, 0).oku(ld, 0).oku(D, 0).yazYa(dq)
		g.dikkatOrtak(pr, md, mo, p)
		g.baslat(c, [3]uint32{bloklar(p.Tq, a.Br), uint32(p.H), uint32(p.Batch)}, [3]uint32{flashIs, 1, 1}, pr)
	}
	if kk != gradYok || kv != gradYok {
		c := g.cekirdekAl(a.anahtar("dikkat_geri_kv")+string(rune('0'+kk))+string(rune('0'+kv)), flashGeriKVCekirdegi(a, kk, kv))
		pr := g.param().okuV(qv).okuV(kvw).okuV(vv).oku(dO, 0).oku(ld, 0).oku(D, 0).yazYa(dk).yazYa(dv)
		g.dikkatOrtak(pr, md, mo, p)
		g.baslat(c, [3]uint32{bloklar(p.Tk, a.Bc), uint32(p.Hkv), uint32(p.Batch)}, [3]uint32{flashIs, 1, 1}, pr)
	}
	sonQ()
	sonK()
	sonV()
}

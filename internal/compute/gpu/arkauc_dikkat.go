package gpu

import (
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: dikkat. Baş boyları birleşik çekirdeğin paylaşımlı belleğine sığıyorsa (Dh, Dv ≤
// 128 civarı; bkz. flashSec) birleşik yol, değilse matris çarpımı yolu (dikkat.go).

// flashUygun birleşik yolun bu tanım için kullanılabildiği ve döşeme ayarı (yol
// TR_GPU_DIKKAT ile zorlanmışsa ona göre).
func (g *arkaUc) flashUygun(p tensor.AttentionParams, maske, geri bool) (flashAyar, bool) {
	if g.dikkatYolu == "carpim" || p.Dh == 0 || p.Dv == 0 || p.H > 65535 || p.Batch > 65535 {
		return flashAyar{}, false
	}
	a, ok := flashSec(p.Dh, p.Dv, maske, geri, g.sr.paylasimSiniri())
	if ok {
		dikkatBoyDenetle(p, true)
	}
	return a, ok
}

// dikkatBoyDenetle çekirdeklerin 32 bitlik dizinlerinin sınırlarını denetler. Birleşik yolda
// puan matrisi oluşmaz: q/çıktı (B·H·Tq·D) ve k/v (B·Hkv·Tk·D) dizinleri sınırlıdır. Matris
// yolunda puan matrisi (B·H·Tq·Tk) de tek depolamadır.
func dikkatBoyDenetle(p tensor.AttentionParams, birlesik bool) {
	D := max(p.Dh, p.Dv)
	boyDenetle(p.Batch * p.H * p.Tq * D)
	boyDenetle(p.Batch * p.Hkv * p.Tk * D)
	if !birlesik {
		boyDenetle(p.Batch * p.H * p.Tq * max(p.Tk, 1))
	}
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
	a, ok := g.flashUygun(p, md != nil, false)
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
	a, ok := g.flashUygun(p, md != nil, true)
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
	var cik [3]*depo
	var kip [3]int
	var sonlar []func()
	for i, s := range []tensor.Storage{gq, gk, gv} {
		cik[i], kip[i], sonlar = g.dikkatCikisi(gt, s, taze[i], cik[:i], []tensor.Storage{gq, gk, gv}[:i], sonlar)
	}
	dq, dk, dv := cik[0], cik[1], cik[2]
	kq, kk, kv := kip[0], kip[1], kip[2]
	qv, kvw, vv := gt.oku(q), gt.oku(k), gt.oku(v)
	dO := gt.okuD(gs)
	od := gt.okuD(o)
	ld := gt.okuD(lse)
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
	for _, son := range sonlar {
		son()
	}
}

// dikkatCikisi bir gradyan çıktısının yazılacağı float32 depolamayı ve kipini verir. Önceki
// bir çıktıyla aynı depolamaya düşen gradyan (q, k, v aynı tensörse) aynı (geçici) tamponu
// kullanır ve ona ekler: çekirdekler Q, K, V sırasıyla yazdığından ilk yazan üzerine yazar
// ya da ekler, sonrakiler ekler; 16 bitlik gradyanın geçici tamponu bir kez yüklenir ve bir
// kez geri yazılır (ayrı tamponlar birbirinin sonucunu ezerdi).
func (g *arkaUc) dikkatCikisi(gt *gecici, s tensor.Storage, taze bool, onceki []*depo, oncekiS []tensor.Storage, sonlar []func()) (*depo, int, []func()) {
	if s == nil {
		return nil, gradYok, sonlar
	}
	asil := g.depoOf(s)
	for j, o := range oncekiS {
		if o != nil && g.depoOf(o) == asil {
			return onceki[j], gradEkle, sonlar
		}
	}
	kip := gradEkle
	if taze {
		kip = gradUzerine
	}
	d, son := gt.yaz(s, kip == gradEkle)
	return d, kip, append(sonlar, son)
}

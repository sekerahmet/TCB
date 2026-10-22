package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: dikkat. 16 bitlik girdilerde (sm_80+) tensör çekirdekli birleşik yol
// (kernel_flash_mma.go); öbür durumlarda bakış boyları birleşik çekirdeğin paylaşımlı belleğine
// sığıyorsa (Dh, Dv ≤ 128 civarı; bkz. pickFlash) SIMT birleşik yol, değilse matris çarpımı
// yolu (attention.go).

// flashFits birleşik yolun bu tanım için kullanılabildiği ve döşeme ayarı (yol
// TR_GPU_DIKKAT ile zorlanmışsa ona göre).
func (g *backend) flashFits(p tensor.AttentionParams, mask2, backward bool) (flashConfig, bool) {
	if g.attnPath == "carpim" || p.Dh == 0 || p.Dv == 0 || p.H > 65535 || p.Batch > 65535 {
		return flashConfig{}, false
	}
	a, ok := pickFlash(p.Dh, p.Dv, mask2, backward, g.sr.sharedMemLimit())
	if ok {
		checkAttnSize(p, true)
	}
	return a, ok
}

// checkAttnSize çekirdeklerin 32 bitlik dizinlerinin sınırlarını denetler. Birleşik yolda
// puan matrisi oluşmaz: q/çıktı (B·H·Tq·D) ve k/v (B·Hkv·Tk·D) dizinleri sınırlıdır. Matris
// yolunda puan matrisi (B·H·Tq·Tk) de tek depolamadır.
func checkAttnSize(p tensor.AttentionParams, fused bool) {
	D := max(p.Dh, p.Dv)
	checkSize(p.Batch * p.H * p.Tq * D)
	checkSize(p.Batch * p.Hkv * p.Tk * D)
	if !fused {
		checkSize(p.Batch * p.H * p.Tq * max(p.Tk, 1))
	}
}

func (g *backend) Attention(out tensor.Storage, q, k, v tensor.View, mask tensor.Storage, p tensor.AttentionParams) tensor.Storage {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	R := p.Batch * p.H * p.Tq
	lse := g.allocate(R, tensor.Float32, false)
	od, last := gt.write(out, false)
	defer last()
	if R == 0 {
		return lse
	}
	if p.Tk == 0 {
		g.fill(od, 0)
		g.fill(lse, float32(math.Inf(1)))
		return lse
	}
	md, mo := g.prepareAttn(gt, mask, p)
	if !p.SeqMajor && p.H > 1 {
		// Birleşik çekirdekler dilin düzenini ([toplu, T, bakış, D]) okur; bakış önde verilen
		// tanım (tensor.AttentionOut yalnızca AttentionSeqMajor yanlışsa verir) matris yolundadır.
		g.attnMatrix(gt, od, lse, q, k, v, md, mo, p)
		return lse
	}
	if tc, ok := g.mmaFits(q, k, v, md != nil, p); ok {
		c := g.takeKernel(tc.key("dikkat_ileri_tc"), flashMmaForwardKernel(tc))
		pr := g.param().readV(q).readV(k).readV(v).write(od, 0).write(lse, 0)
		g.attnShared(pr, md, mo, p)
		g.launch(c, [3]uint32{blocks(p.Tq, mmaBr), uint32(p.H), uint32(p.Batch)}, [3]uint32{flashJob, 1, 1}, pr)
		return lse
	}
	a, ok := g.flashFits(p, md != nil, false)
	if !ok {
		g.attnMatrix(gt, od, lse, q, k, v, md, mo, p)
		return lse
	}
	qv, kv, vv := gt.read(q), gt.read(k), gt.read(v)
	c := g.takeKernel(a.key("dikkat_ileri"), flashForwardKernel(a))
	pr := g.param().readV(qv).readV(kv).readV(vv).write(od, 0).write(lse, 0)
	g.attnShared(pr, md, mo, p)
	g.launch(c, [3]uint32{blocks(p.Tq, a.Br), uint32(p.H), uint32(p.Batch)}, [3]uint32{flashJob, 1, 1}, pr)
	return lse
}

// AttentionSeqMajor (tensor.SeqMajorAttender) dikkatin dilin düzeninde birleşik çekirdeklerle
// (ileri ve geri) hesaplanabildiği: tensör çekirdekli ya da SIMT birleşik yol uyuyorsa doğru.
// Uymuyorsa (bakış boyları büyük, TR_GPU_DIKKAT=carpim) tensör katmanı girdileri bakış önde
// görünümlere çevirir ve matris yolu çalışır.
func (g *backend) AttentionSeqMajor(q, k, v tensor.View, mask bool, p tensor.AttentionParams) bool {
	g.enter()
	defer g.exit()
	tc, tcOK := g.mmaFits(q, k, v, mask, p)
	_, fw := g.flashFits(p, mask, false)
	_, bw := g.flashFits(p, mask, true)
	return (tcOK || fw) && (tcOK && tc.Dh <= mmaBackMax && tc.Dv <= mmaBackMax || bw)
}

// attnShared çekirdeklerin ortak parametreleri (flashOrtak.ortakParametreler sırası).
func (g *backend) attnShared(pr *paramWriter, md, mo *storage, p tensor.AttentionParams) {
	reason2 := 0
	if p.Causal {
		reason2 = 1
	}
	pr.readMask(md, p).readFrom(mo).u32(p.Tq).u32(p.Tk).u32(p.H).u32(p.H / p.Hkv).u32(reason2).u32(p.Window).
		u32(p.MaskStrideQ).u32(p.MaskStrideK).f32(p.Scale)
}

func (g *backend) AttentionGrad(gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, mask tensor.Storage, o, lse tensor.Storage, p tensor.AttentionParams) {
	g.enter()
	defer g.exit()
	g.attnBackward(gq, gk, gv, gs, q, k, v, mask, o, lse, p, [3]bool{})
}

// AttentionGradSet (tensor.AttentionGradSetter) taze[i] doğru olan gradyanların (gq, gk, gv)
// üzerine yazar; diğerlerine ekler.
func (g *backend) AttentionGradSet(gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, mask tensor.Storage, o, lse tensor.Storage, p tensor.AttentionParams, fresh [3]bool) {
	g.enter()
	defer g.exit()
	g.attnBackward(gq, gk, gv, gs, q, k, v, mask, o, lse, p, fresh)
}

func (g *backend) attnBackward(gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, mask tensor.Storage, o, lse tensor.Storage, p tensor.AttentionParams, fresh [3]bool) {
	gt := g.temp()
	defer gt.finish()
	R := p.Batch * p.H * p.Tq
	md, mo := g.prepareAttn(gt, mask, p)
	tc, tcOK := g.mmaFits(q, k, v, md != nil, p)
	tcOK = tcOK && R > 0 && p.Tk > 0 && tc.Dh <= mmaBackMax && tc.Dv <= mmaBackMax
	a, ok := g.flashFits(p, md != nil, true)
	if !p.SeqMajor && p.H > 1 {
		tcOK, ok = false, false // bakış önde: matris yolu (bkz. Attention)
	}
	if !tcOK && (!ok || R == 0 || p.Tk == 0) {
		// Taze gradyanlar sıfırlanmamıştır: önce sıfırlanır, sonra eklenir.
		for i, s := range []tensor.Storage{gq, gk, gv} {
			if s != nil && fresh[i] {
				g.fill(g.storageOf(s), 0)
			}
		}
		if R > 0 && p.Tk > 0 && (gq != nil || gk != nil || gv != nil) {
			g.attnMatrixBackward(gt, gq, gk, gv, gs, q, k, v, md, mo, o, lse, p)
		}
		return
	}
	var exit [3]*storage
	var mode2 [3]int
	var lasts []func()
	for i, s := range []tensor.Storage{gq, gk, gv} {
		exit[i], mode2[i], lasts = g.attnOutput(gt, s, fresh[i], exit[:i], []tensor.Storage{gq, gk, gv}[:i], lasts)
	}
	dq, dk, dv := exit[0], exit[1], exit[2]
	kq, kk, kv := mode2[0], mode2[1], mode2[2]
	dO := gt.readD(gs)
	od := gt.readD(o)
	ld := gt.readD(lse)
	// D = satır başına Σ dO·O.
	D := gt.buffer(R, false)
	cd := g.takeKernel("dikkat_d", flashDKernel)
	iz, bl := plain(R, sameBlock)
	g.launch(cd, iz, bl, g.param().read(dO, 0).read(od, 0).write(D, 0).u32(R).u32(p.Dv).u32(p.H).u32(p.Tq))
	if tcOK {
		g.attnBackwardTC(tc, q, k, v, dO, ld, D, md, mo, dq, dk, dv, kq, kk, kv, p)
		for _, last := range lasts {
			last()
		}
		return
	}
	qv, kvw, vv := gt.read(q), gt.read(k), gt.read(v)
	if kq != noGrad {
		c := g.takeKernel(a.key("dikkat_geri_q")+string(rune('0'+kq)), flashBackwardQKernel(a, kq))
		pr := g.param().readV(qv).readV(kvw).readV(vv).read(dO, 0).read(ld, 0).read(D, 0).writeTo(dq)
		g.attnShared(pr, md, mo, p)
		g.launch(c, [3]uint32{blocks(p.Tq, a.Br), uint32(p.H), uint32(p.Batch)}, [3]uint32{flashJob, 1, 1}, pr)
	}
	if kk != noGrad || kv != noGrad {
		c := g.takeKernel(a.key("dikkat_geri_kv")+string(rune('0'+kk))+string(rune('0'+kv)), flashBackwardKVKernel(a, kk, kv))
		pr := g.param().readV(qv).readV(kvw).readV(vv).read(dO, 0).read(ld, 0).read(D, 0).writeTo(dk).writeTo(dv)
		g.attnShared(pr, md, mo, p)
		g.launch(c, [3]uint32{blocks(p.Tk, a.Bc), uint32(p.Hkv), uint32(p.Batch)}, [3]uint32{flashJob, 1, 1}, pr)
	}
	for _, last := range lasts {
		last()
	}
}

// attnOutput bir gradyan çıktısının yazılacağı float32 depolamayı ve kipini verir. Önceki
// bir çıktıyla aynı depolamaya düşen gradyan (q, k, v aynı tensörse) aynı (geçici) tamponu
// kullanır ve ona ekler: çekirdekler Q, K, V sırasıyla yazdığından ilk yazan üzerine yazar
// ya da ekler, sonrakiler ekler; 16 bitlik gradyanın geçici tamponu bir kez yüklenir ve bir
// kez geri yazılır (ayrı tamponlar birbirinin sonucunu ezerdi).
func (g *backend) attnOutput(gt *temp, s tensor.Storage, fresh bool, prev []*storage, prevS []tensor.Storage, lasts []func()) (*storage, int, []func()) {
	if s == nil {
		return nil, noGrad, lasts
	}
	base := g.storageOf(s)
	for j, o := range prevS {
		if o != nil && g.storageOf(o) == base {
			return prev[j], addGrad, lasts
		}
	}
	mode2 := addGrad
	if fresh {
		mode2 = gradOver
	}
	d, last := gt.write(s, mode2 == addGrad)
	return d, mode2, append(lasts, last)
}

// attnBackwardTC tensör çekirdekli geri yön: 16 bitlik q, k, v doğrudan okunur.
func (g *backend) attnBackwardTC(tc flashMma, q, k, v tensor.View, dO, ld, D, md, mo, dq, dk, dv *storage, kq, kk, kv int, p tensor.AttentionParams) {
	if kq != noGrad {
		c := g.takeKernel(fmt.Sprintf("%s_%d", tc.key("dikkat_geri_q_tc"), kq), flashMmaBackwardQKernel(tc, kq))
		pr := g.param().readV(q).readV(k).readV(v).read(dO, 0).read(ld, 0).read(D, 0).writeTo(dq)
		g.attnShared(pr, md, mo, p)
		g.launch(c, [3]uint32{blocks(p.Tq, mmaBr), uint32(p.H), uint32(p.Batch)}, [3]uint32{flashJob, 1, 1}, pr)
	}
	if kk != noGrad || kv != noGrad {
		c := g.takeKernel(fmt.Sprintf("%s_%d_%d", tc.key("dikkat_geri_kv_tc"), kk, kv), flashMmaBackwardKVKernel(tc, kk, kv))
		pr := g.param().readV(q).readV(k).readV(v).read(dO, 0).read(ld, 0).read(D, 0).writeTo(dk).writeTo(dv)
		g.attnShared(pr, md, mo, p)
		g.launch(c, [3]uint32{blocks(p.Tk, mmaKeys), uint32(p.Hkv), uint32(p.Batch)}, [3]uint32{flashJob, 1, 1}, pr)
	}
}

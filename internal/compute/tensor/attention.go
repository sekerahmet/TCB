package tensor

import (
	"fmt"
	"math"
)

// Birleşik ölçekli nokta çarpımı dikkati (cihazdan bağımsız katman).
//
//	çıktı = softmax(q·kᵀ·ölçek + maske) · v
//
// tek bir otomatik türev düğümüdür. Arka uç ileri geçişi döşemeli yapar:
// sorgular satır blokları, anahtarlar anahtar blokları hâlinde gezilir ve
// softmax çevrimiçi (satır başına koşan en büyük değer ve payda) hesaplanır;
// Tq×Tk puan matrisi hiçbir zaman bütünüyle tutulmaz. Geri yayılım için
// yalnızca satır başına logsumexp saklanır; puanlar geri geçişte yeniden
// hesaplanır. Bellek bu yüzden T ile doğrusal büyür. Blok boyutları arka ucun
// ayarıdır (bkz. AttentionTiler).

// AttentionOptions dikkatin seçenekleridir.
type AttentionOptions struct {
	// Causal, sorgu i'nin yalnızca Tk-Tq+i'ye kadar olan anahtarlara bakmasını
	// sağlar (Tq < Tk iken nedensellik sondan hizalıdır: önbellekli üretim).
	Causal bool
	// Window > 0 ise sorgu yalnızca son Window konuma bakar (kendisi dahil).
	// Nedensel değilse pencere iki yönlüdür: |Tk-Tq+i - j| < Window.
	Window int
	// Mask isteğe bağlı 0/1 maskesidir (sıfırdan farklı = bakılır); şekli
	// [..., H, Tq, Tk]'ye yayınlanabilir olmalıdır. Türev izlenmez.
	Mask *Tensor
	// Scale puanların ölçeğidir; 0 ise 1/√dh.
	Scale float32
}

// AttentionParams arka uca verilen, şekilleri çözülmüş dikkat tanımıdır. q, k,
// v görünümleri bitişiktir: q [Batch, H, Tq, Dh], k [Batch, Hkv, Tk, Dh],
// v [Batch, Hkv, Tk, Dv]. H, Hkv'nin katıdır; baş h, k/v başı h/(H/Hkv)'yi
// kullanır (GQA: k ve v kopyalanmaz).
type AttentionParams struct {
	Batch, H, Hkv, Tq, Tk, Dh, Dv int
	Causal                        bool
	Window                        int
	Scale                         float32
	// Maske (varsa) float32 bitişik bir depolamadır; (b, h) çifti için ilk
	// eleman MaskOffsets[b*H+h], sorgu ve anahtar adımları MaskStrideQ,
	// MaskStrideK'dır (yayınlanan eksenlerde 0).
	MaskOffsets              []int
	MaskStrideQ, MaskStrideK int
}

// Limits sorgu satırı i'nin bakabileceği anahtar aralığını [lo, hi) verir
// (nedensellik ve pencereye göre; ek maske hariç). lo ve hi i ile azalmaz.
func (p *AttentionParams) Limits(i int) (lo, hi int) {
	c := i + p.Tk - p.Tq
	lo, hi = 0, p.Tk
	if p.Causal {
		hi = min(hi, c+1)
	}
	if p.Window > 0 {
		lo = max(lo, c-p.Window+1)
		if !p.Causal {
			hi = min(hi, c+p.Window)
		}
	}
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

// AttentionTiler döşemeli dikkatin blok boyutlarını ayarlanabilir kılan arka
// uç arayüzüdür (sorgu satır bloğu ve anahtar bloğu). Her arka uç kendi
// varsayılanını kendi belleğine (CPU'da önbellek, GPU'da paylaşımlı bellek)
// göre seçer.
type AttentionTiler interface {
	AttentionBlocks() (q, k int)
	SetAttentionBlocks(q, k int)
}

// SetAttentionBlocks cihazın arka ucundaki dikkat blok boyutlarını ayarlar
// (0 verilen boyut varsayılana döner). Arka uç ayarlanabilir değilse panik.
func SetAttentionBlocks(dev Device, q, k int) {
	t, ok := BackendFor(dev).(AttentionTiler)
	if !ok {
		panic(fmt.Sprintf("tensor: %s arka ucunun dikkat blok boyutları ayarlanamaz", dev))
	}
	t.SetAttentionBlocks(q, k)
}

// AttentionBlocks cihazın arka ucundaki dikkat blok boyutlarını verir.
func AttentionBlocks(dev Device) (q, k int) {
	if t, ok := BackendFor(dev).(AttentionTiler); ok {
		return t.AttentionBlocks()
	}
	return 0, 0
}

// Attention birleşik dikkat; çıktı türü girdilerin türüdür (türler farklıysa
// float32).
func Attention(q, k, v *Tensor, opt AttentionOptions) *Tensor {
	return AttentionOut(q, k, v, opt, resultDType(q.dtyp, resultDType(k.dtyp, v.dtyp)))
}

// AttentionOut çıktı türü açıkça verilen birleşik dikkattir (karışık
// hassasiyet: 16 bitlik girdiler, float32 biriktirme ve çıktı).
//
// Şekiller: q [..., H, Tq, dh], k [..., Hkv, Tk, dh], v [..., Hkv, Tk, dv];
// öndeki eksenler eşit olmalıdır. 2 boyutlu girdiler tek başlı sayılır.
// Sonuç [..., H, Tq, dv] şeklindedir. Hiçbir anahtara bakamayan sorgu
// satırının çıktısı 0'dır (gradyanı da 0).
func AttentionOut(q, k, v *Tensor, opt AttentionOptions, outType DType) *Tensor {
	be := sameDevice("attention", q, k, v)
	sy := counterStart(be, OpKindAttention).start()
	p := attentionParams(q, k, v, opt)
	outShape := copyInts(q.shape)
	outShape[len(outShape)-1] = p.Dv

	var maskVal Storage
	if opt.Mask != nil {
		if opt.Mask.Device() != q.Device() {
			panic(fmt.Sprintf("tensor: Attention maskesi farklı cihazda (%s, %s)", opt.Mask.Device(), q.Device()))
		}
		maskVal = prepareMask(be, opt.Mask, q.shape, &p)
	}
	qv, kv, vv := q.contigView(), k.contigView(), v.contigView()
	out := emptyOf(isAttention, be, outShape, outType)
	lse := be.Attention(out.storage, qv, kv, vv, maskVal, p)
	r := track(out, "attention", []*Tensor{q, k, v}, func() {
		as, writer := be.(AttentionGradSetter)
		var g [3]Storage
		var fresh [3]bool
		for i, t := range []*Tensor{q, k, v} {
			if t.requiresGrad {
				g[i], fresh[i] = t.freshGrad(writer)
			}
		}
		if writer {
			as.AttentionGradSet(g[0], g[1], g[2], out.grad, qv, kv, vv, maskVal, out.storage, lse, p, fresh)
			return
		}
		be.AttentionGrad(g[0], g[1], g[2], out.grad, qv, kv, vv, maskVal, out.storage, lse, p)
	}, q, k, v)
	sy.finish()
	return r
}

// attentionParams şekilleri denetler ve arka uç tanımını kurar.
func attentionParams(q, k, v *Tensor, opt AttentionOptions) AttentionParams {
	nd := len(q.shape)
	if nd < 2 || len(k.shape) != nd || len(v.shape) != nd {
		panic(fmt.Sprintf("tensor: Attention q, k, v aynı sayıda (en az 2) eksen ister: %v, %v, %v", q.shape, k.shape, v.shape))
	}
	lead := 0
	if nd > 3 {
		lead = nd - 3
	}
	if !sameShape(q.shape[:lead], k.shape[:lead]) || !sameShape(q.shape[:lead], v.shape[:lead]) {
		panic(fmt.Sprintf("tensor: Attention öndeki eksenler eşit olmalı: %v, %v, %v", q.shape, k.shape, v.shape))
	}
	p := AttentionParams{Batch: numelOf(q.shape[:lead]), H: 1, Hkv: 1,
		Tq: q.shape[nd-2], Dh: q.shape[nd-1], Tk: k.shape[nd-2], Dv: v.shape[nd-1],
		Causal: opt.Causal, Window: opt.Window, Scale: opt.Scale}
	if nd >= 3 {
		p.H, p.Hkv = q.shape[nd-3], k.shape[nd-3]
		if v.shape[nd-3] != p.Hkv {
			panic(fmt.Sprintf("tensor: Attention k ve v baş sayıları farklı: %v, %v", k.shape, v.shape))
		}
	}
	if k.shape[nd-1] != p.Dh {
		panic(fmt.Sprintf("tensor: Attention q ve k'nin son ekseni eşit olmalı: %v, %v", q.shape, k.shape))
	}
	if v.shape[nd-2] != p.Tk {
		panic(fmt.Sprintf("tensor: Attention k ve v'nin anahtar sayısı eşit olmalı: %v, %v", k.shape, v.shape))
	}
	if p.Hkv == 0 || p.H%p.Hkv != 0 {
		panic(fmt.Sprintf("tensor: Attention: sorgu baş sayısı (%d), anahtar/değer baş sayısının (%d) katı olmalı", p.H, p.Hkv))
	}
	if p.Window < 0 {
		panic(fmt.Sprintf("tensor: Attention penceresi negatif olamaz (%d)", p.Window))
	}
	if p.Scale == 0 {
		p.Scale = float32(1 / math.Sqrt(float64(max(p.Dh, 1))))
	}
	return p
}

// prepareMask maskeyi float32 bitişik depolamaya çevirir ve [lead..., H, Tq,
// Tk]'ye yayınlanmış adımlarını p'ye yazar.
func prepareMask(be Backend, m *Tensor, qShape []int, p *AttentionParams) Storage {
	whole := append(copyInts(qShape[:len(qShape)-1]), p.Tk) // [..., H, Tq, Tk]
	if len(qShape) == 2 {
		whole = []int{p.Tq, p.Tk}
	}
	if len(m.shape) > len(whole) {
		panic(fmt.Sprintf("tensor: Attention maskesi %v, %v şekline yayınlanamaz", m.shape, whole))
	}
	for i := 1; i <= len(m.shape); i++ {
		d := m.shape[len(m.shape)-i]
		if d != 1 && d != whole[len(whole)-i] {
			panic(fmt.Sprintf("tensor: Attention maskesi %v, %v şekline yayınlanamaz", m.shape, whole))
		}
	}
	s := allocUninit(be, m.Numel(), Float32)
	be.Copy(s, 0, m.view())
	st := broadcastStrides(contiguousStrides(m.shape), m.shape, whole)
	n := len(whole)
	p.MaskStrideQ, p.MaskStrideK = st[n-2], st[n-1]
	p.MaskOffsets = make([]int, p.Batch*p.H)
	// Öndeki eksenler (lead) ve baş ekseni satır öncelikli gezilir.
	on := whole[:n-2]
	preSt := st[:n-2]
	idx := make([]int, len(on))
	for bh := range p.MaskOffsets {
		o := 0
		for d := range idx {
			o += idx[d] * preSt[d]
		}
		p.MaskOffsets[bh] = o
		for d := len(idx) - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < on[d] {
				break
			}
			idx[d] = 0
		}
	}
	return s
}

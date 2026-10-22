package tensor

import (
	"math"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	// [..., H, Tq, Tk]'ye yayınlanabilir olmalıdır (girdilerin düzeninden bağımsız; JAX'ta da
	// maske BNTS'dir). Türev izlenmez.
	Mask *Tensor
	// Scale puanların ölçeğidir; 0 ise 1/√dh.
	Scale float32
}

// AttentionParams arka uca verilen, şekilleri çözülmüş dikkat tanımıdır. q, k,
// v görünümleri bitişiktir. SeqMajor ise düzen dilinkidir (dil tanımı 15.17; JAX
// dot_product_attention ve FlashAttention gibi): q [Batch, Tq, H, Dh], k [Batch, Tk, Hkv, Dh],
// v [Batch, Tk, Hkv, Dv], çıktı ve gradyanlar da öyle. Değilse bakış öndedir: q [Batch, H, Tq,
// Dh] ... (arka ucun yeni düzeni desteklemediği durum; AttentionOut girdileri görünüm olarak
// çevirir). H, Hkv'nin katıdır; bakış h, k/v bakışı h/(H/Hkv)'yi kullanır (GQA: k ve v
// kopyalanmaz). lse her iki düzende [Batch, H, Tq]'dur.
type AttentionParams struct {
	Batch, H, Hkv, Tq, Tk, Dh, Dv int
	SeqMajor                      bool
	Causal                        bool
	Window                        int
	Scale                         float32
	// Maske (varsa) float32 bitişik bir depolamadır; (b, h) çifti için ilk
	// eleman MaskBase + MaskOffsets[b*H+h], sorgu ve anahtar adımları MaskStrideQ,
	// MaskStrideK'dır (yayınlanan eksenlerde 0). MaskBase görünümün depolamadaki
	// başlangıcıdır; ofset tablosu ona göreli olduğundan aynı şekilli maskelerde aynıdır
	// (GPU onu bir kez yükler, tekrar_oynat kaydında da bulur).
	MaskBase                 int
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
		panic(messages.New(messages.H3001, dev))
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
// Şekiller: q [..., Tq, H, dh], k [..., Tk, Hkv, dh], v [..., Tk, Hkv, dv];
// öndeki eksenler eşit olmalıdır. 2 boyutlu girdiler tek bakışlı sayılır.
// Sonuç [..., Tq, H, dv] şeklindedir. Hiçbir anahtara bakamayan sorgu
// satırının çıktısı 0'dır (gradyanı da 0). Bakışlar doğrusal katmanın
// çıktısını şekillendirmekle ayrılır, eksen sıralama ve kopya gerekmez.
func AttentionOut(q, k, v *Tensor, opt AttentionOptions, outType DType) *Tensor {
	be := sameDevice("attention", q, k, v)
	p := attentionParams(q, k, v, opt)
	if opt.Mask != nil && opt.Mask.Device() != q.Device() {
		panic(messages.New(messages.H3002, opt.Mask.Device(), q.Device()))
	}
	whole := []int{p.Tq, p.Tk}
	if nd := len(q.shape); nd >= 3 {
		whole = append(copyInts(q.shape[:nd-3]), p.H, p.Tq, p.Tk)
	}
	if len(q.shape) >= 3 && !seqMajorNative(be, q, k, v, opt.Mask != nil, p) {
		// Arka uç bu tanımı bakış önde hesaplar: girdiler görünüm olarak çevrilir, sonuç
		// geri çevrilmiş bir görünümdür; geri yön eksen sıralamanın türeviyle gelir.
		out := copyInts(q.shape)
		out[len(out)-1] = p.Dv
		checkAllocOn(be, isAttention, out, outType)
		h := swapHeadAxis(len(q.shape))
		return attend(be, q.Permute(h...), k.Permute(h...), v.Permute(h...), opt, outType, p, whole).Permute(h...)
	}
	p.SeqMajor = true
	return attend(be, q, k, v, opt, outType, p, whole)
}

// SeqMajorAttender (isteğe bağlı) dikkati dilin düzeninde (SeqMajor) kopyasız hesaplayabilen
// arka uç: bitişik görünümlerle bu tanım için ileri ve geri yolu varsa doğru verir.
type SeqMajorAttender interface {
	AttentionSeqMajor(q, k, v View, mask bool, p AttentionParams) bool
}

// seqMajorNative arka ucun bu dikkati dilin düzeninde hesaplayıp hesaplayamayacağı (görünümler
// bitişik olacakları biçimde sorulur: bitişik değilse attend kopyalar).
func seqMajorNative(be Backend, q, k, v *Tensor, mask bool, p AttentionParams) bool {
	a, ok := be.(SeqMajorAttender)
	if !ok {
		return false
	}
	view := func(t *Tensor) View {
		if t.IsContiguous() {
			return t.view()
		}
		return View{S: t.storage, Shape: t.shape, Strides: contiguousStrides(t.shape)}
	}
	p.SeqMajor = true
	return a.AttentionSeqMajor(view(q), view(k), view(v), mask, p)
}

// swapHeadAxis T ile bakış eksenlerini (sondan üçüncü ve ikinci) değiştiren eksen sırası
// (kendisinin tersidir).
func swapHeadAxis(nd int) []int {
	ax := make([]int, nd)
	for i := range ax {
		ax[i] = i
	}
	ax[nd-3], ax[nd-2] = nd-2, nd-3
	return ax
}

// attend dikkat düğümünü kurar: q, k, v p'nin düzenindedir (SeqMajor ya da bakış önde),
// whole maskenin yayınlandığı [..., H, Tq, Tk] şeklidir.
func attend(be Backend, q, k, v *Tensor, opt AttentionOptions, outType DType, p AttentionParams, whole []int) *Tensor {
	sy := counterStart(be, OpKindAttention).start()
	outShape := copyInts(q.shape)
	outShape[len(outShape)-1] = p.Dv

	var maskVal Storage
	rm := func() {}
	if opt.Mask != nil {
		maskVal = prepareMask(be, opt.Mask, whole, &p)
		if maskVal != opt.Mask.storage {
			be, s := be, maskVal
			rm = func() { giveBack(be, s) }
		}
	}
	qv, kv, vv := q.contigView(), k.contigView(), v.contigView()
	// Bitişik kopyalar (maskeninki de) geri yönde de okunur; geri yön bitince (türev
	// izlenmiyorsa hemen) iade edilir. Kopyalanmadan okunan maske çizgeye kaydedilir: geri
	// yöne kadar tutulur ve yerinde değişirse sürüm denetimi yakalar.
	rq, rk, rv := copyRelease(q, qv), copyRelease(k, kv), copyRelease(v, vv)
	release := func() { rq(); rk(); rv(); rm() }
	out := emptyOf(isAttention, be, outShape, outType)
	lse := be.Attention(out.storage, qv, kv, vv, maskVal, p)
	os := out.storage
	r := track(out, "attention", []*Tensor{q, k, v}, func(o *node) {
		defer release()
		as, writer := be.(AttentionGradSetter)
		var g [3]Storage
		var fresh [3]bool
		for i, n := range o.parents {
			if n.wants() {
				g[i], fresh[i] = n.freshGrad(writer)
			}
		}
		if writer {
			as.AttentionGradSet(g[0], g[1], g[2], o.grad, qv, kv, vv, maskVal, os, lse, p, fresh)
			return
		}
		be.AttentionGrad(g[0], g[1], g[2], o.grad, qv, kv, vv, maskVal, os, lse, p)
		// Geri yön ileri çıktıyı da okur (D = Σ dO·O): çıktı yerinde değişirse sürüm
		// denetimi yakalar.
	}, q, k, v, out, savedMask(opt.Mask, maskVal))
	if r.IsLeaf() {
		release()
	}
	sy.finish()
	return r
}

// savedMask kopyalanmadan okunan maskeyi (geri yön onun depolamasını okur) verir; maske yoksa
// ya da kopyalandıysa nil.
func savedMask(m *Tensor, s Storage) *Tensor {
	if m == nil || s != m.storage {
		return nil
	}
	return m
}

// attentionParams şekilleri denetler ve arka uç tanımını kurar.
func attentionParams(q, k, v *Tensor, opt AttentionOptions) AttentionParams {
	nd := len(q.shape)
	if nd < 2 || len(k.shape) != nd || len(v.shape) != nd {
		panic(messages.New(messages.H3003, q.shape, k.shape, v.shape))
	}
	lead := 0
	if nd > 3 {
		lead = nd - 3
	}
	if !sameShape(q.shape[:lead], k.shape[:lead]) || !sameShape(q.shape[:lead], v.shape[:lead]) {
		panic(messages.New(messages.H3004, q.shape, k.shape, v.shape))
	}
	// Düzen [..., T, bakış, D]: T sondan üçüncü (2 boyutluda ikinci), bakış sondan ikinci.
	tAxis := nd - 2
	if nd >= 3 {
		tAxis = nd - 3
	}
	p := AttentionParams{Batch: numelOf(q.shape[:lead]), H: 1, Hkv: 1,
		Tq: q.shape[tAxis], Dh: q.shape[nd-1], Tk: k.shape[tAxis], Dv: v.shape[nd-1],
		Causal: opt.Causal, Window: opt.Window, Scale: opt.Scale}
	if nd >= 3 {
		p.H, p.Hkv = q.shape[nd-2], k.shape[nd-2]
		if v.shape[nd-2] != p.Hkv {
			panic(messages.New(messages.H3005, k.shape, v.shape))
		}
	}
	if k.shape[nd-1] != p.Dh {
		panic(messages.New(messages.H3006, q.shape, k.shape))
	}
	if v.shape[tAxis] != p.Tk {
		panic(messages.New(messages.H3007, k.shape, v.shape))
	}
	if p.Hkv == 0 || p.H%p.Hkv != 0 {
		panic(messages.New(messages.H3008, p.H, p.Hkv))
	}
	if p.Window < 0 {
		panic(messages.New(messages.H3009, p.Window))
	}
	if p.Scale == 0 {
		p.Scale = float32(1 / math.Sqrt(float64(max(p.Dh, 1))))
	}
	return p
}

// prepareMask maskeyi float32 bitişik depolamaya çevirir (zaten öyleyse kopyalamaz; ofsetler
// görünümün başlangıcını içerir) ve whole'a ([lead..., H, Tq, Tk]) yayınlanmış adımlarını p'ye
// yazar.
func prepareMask(be Backend, m *Tensor, whole []int, p *AttentionParams) Storage {
	if len(m.shape) > len(whole) {
		panic(messages.New(messages.H3010, m.shape, whole))
	}
	for i := 1; i <= len(m.shape); i++ {
		d := m.shape[len(m.shape)-i]
		if d != 1 && d != whole[len(whole)-i] {
			panic(messages.New(messages.H3010, m.shape, whole))
		}
	}
	s, base := m.storage, m.offset
	if m.dtyp != Float32 || !m.IsContiguous() {
		s, base = allocUninit(be, m.Numel(), Float32), 0
		be.Copy(s, 0, m.view())
	}
	st := broadcastStrides(contiguousStrides(m.shape), m.shape, whole)
	n := len(whole)
	p.MaskStrideQ, p.MaskStrideK = st[n-2], st[n-1]
	p.MaskBase = base
	p.MaskOffsets = make([]int, p.Batch*p.H)
	// Öndeki eksenler (lead) ve bakış ekseni satır öncelikli gezilir.
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

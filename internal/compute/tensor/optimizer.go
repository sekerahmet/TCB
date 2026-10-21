package tensor

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İyileştiriciler (cihazdan bağımsız katman). Durum tamponları (momentum,
// moment tahminleri) parametrenin cihazında tutulur; güncelleme arka uçta
// yapılır.

// Optimizer parametreleri gradyanlarına göre güncelleyen iyileştirici.
type Optimizer interface {
	Step()
	ZeroGrad()
}

func checkParams(params []*Tensor) {
	for _, p := range params {
		if !p.RequiresGrad() {
			panic(messages.New(messages.H3054))
		}
		if !p.IsContiguous() || p.offset != 0 || p.storage.Len() != p.Numel() {
			panic(messages.New(messages.H3055, p.shape))
		}
	}
}

// masterWeights 16 bitlik parametreler için float32 ana ağırlık kopyalarını
// yönetir. Güncelleme ana kopyada yapılır ve parametreye yuvarlanarak yazılır;
// böylece parametrenin 16 bitlik çözünürlüğünden küçük adımlar kaybolmaz.
// Parametre iyileştirici dışında değiştirilirse (sürümü son yazımdan farklıysa)
// ana kopya parametreden yeniden alınır.
type masterWeights struct {
	closed  bool
	master  []Storage // parametre başına float32 kopya (gerekmiyorsa nil)
	version []uint64  // son yazımdan sonraki parametre sürümü
}

func newMasterWeights(n int) masterWeights {
	return masterWeights{master: make([]Storage, n), version: make([]uint64, n)}
}

// target k. parametrenin güncelleneceği depolama.
func (a *masterWeights) target(k int, p *Tensor) Storage {
	if a.closed || p.dtyp == Float32 {
		return p.storage
	}
	if a.master[k] == nil || p.storage.Version() != a.version[k] {
		if a.master[k] == nil {
			a.master[k] = p.be.Alloc(p.Numel(), Float32)
		}
		p.be.Copy(a.master[k], 0, p.view())
	}
	return a.master[k]
}

// write güncellenmiş hedefi (ana kopyaysa yuvarlayarak) parametreye yazar ve
// parametrenin sürümünü artırır (yerinde değişiklik).
func (a *masterWeights) write(k int, p *Tensor, h Storage) {
	if h != p.storage {
		p.be.Copy(p.storage, 0, View{S: h, Shape: p.shape, Strides: contiguousStrides(p.shape)})
	}
	p.storage.BumpVersion()
	a.version[k] = p.storage.Version()
}

// configure ana kopya kullanımını açar/kapatır.
func (a *masterWeights) configure(open bool) {
	a.closed = !open
	if !open {
		clear(a.master)
	}
}

// SGD momentumlu (isteğe bağlı) olasılıksal gradyan inişi. WeightDecay,
// gradyana L2 cezası olarak eklenir.
type SGD struct {
	Params      []*Tensor
	LR          float32
	Momentum    float32
	WeightDecay float32
	speed       []Storage
	master      masterWeights
}

// NewSGD yeni bir SGD iyileştirici kurar.
func NewSGD(params []*Tensor, lr, momentum float32) *SGD {
	checkParams(params)
	return &SGD{Params: params, LR: lr, Momentum: momentum, speed: make([]Storage, len(params)),
		master: newMasterWeights(len(params))}
}

// SetMasterWeights 16 bitlik parametreler için float32 ana ağırlık kopyasını
// açar ya da kapatır (varsayılan açık). Kapalıyken güncelleme doğrudan 16
// bitlik parametre üzerinde (float32'de hesaplanıp yuvarlanarak) yapılır.
func (o *SGD) SetMasterWeights(on bool) { o.master.configure(on) }

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır.
// Parametreler yerinde değiştiği için sürümleri artar.
func (o *SGD) Step() {
	sy := stepCounter(o.Params, OpKindSGD).start()
	defer sy.finish()
	sorted := repeated(o.Params)
	var steps []goodStep
	for k, p := range o.Params {
		if p.gradStorage() == nil {
			continue
		}
		if o.Momentum != 0 && o.speed[k] == nil {
			o.speed[k] = p.be.Alloc(p.Numel(), Float32)
		}
		if sorted {
			h := o.master.target(k, p)
			p.be.SGDStep(h, p.gradStorage(), o.speed[k], o.LR, o.Momentum, o.WeightDecay)
			o.master.write(k, p, h)
			continue
		}
		steps = append(steps, goodStep{k: k, p: p, h: o.master.target(k, p), m: o.speed[k]})
	}
	multiApply(steps, func(be MultiTensorOps, a []goodStep) {
		h, g, v := a[0].cols(a)
		be.SGDStepAll(h, g, v, o.LR, o.Momentum, o.WeightDecay)
	}, func(a goodStep) {
		a.p.be.SGDStep(a.h, a.p.gradStorage(), a.m, o.LR, o.Momentum, o.WeightDecay)
	})
	for _, a := range steps {
		o.master.write(a.k, a.p, a.h)
	}
}

// stepCounter iyileştirici adımının profil sayacını başlatır: bir adım (bütün parametreler,
// çok tensörlü yolda tek arka uç çağrısı) bir çağrı sayılır. Cihaz ilk parametreninkidir.
func stepCounter(params []*Tensor, is OpKind) counterMeasure {
	be := Backend(cpuBackend{})
	if len(params) > 0 {
		be = params[0].be
	}
	return counterStart(be, is)
}

// goodStep bir parametrenin bu adımdaki güncellemesi: güncellenecek depolama (h; ana kopya
// olabilir), durum depolamaları (m, v) ve sapma düzeltmeleri.
type goodStep struct {
	k        int
	p        *Tensor
	h, m, v  Storage
	bc1, bc2 float32
}

// cols adımların h, gradyan ve m depolamaları.
func (goodStep) cols(a []goodStep) (h, g, m []Storage) {
	for _, x := range a {
		h, g, m = append(h, x.h), append(g, x.p.gradStorage()), append(m, x.m)
	}
	return h, g, m
}

// multiApply ardışık, aynı arka uçtaki ve çok tensörlü işi destekleyen adımları tek çağrıda
// (coklu), kalanları tek tek (tekli) uygular; sıra korunur.
//
// Aynı parametre listede iki kez geçerse (aynı depolama) grup orada bölünür: tek başlatmada iki
// blok aynı belleği aynı anda güncellerdi; bölünce güncellemeler tek tek yoldaki gibi sırayla
// yapılır.
func multiApply(steps []goodStep, multi func(MultiTensorOps, []goodStep), unaryArg func(goodStep)) {
	for i := 0; i < len(steps); {
		mt, ok := steps[i].p.be.(MultiTensorOps)
		if !ok {
			unaryArg(steps[i])
			i++
			continue
		}
		wasSeen := map[Storage]bool{steps[i].p.gradStorage(): true}
		j := i + 1
		for j < len(steps) && steps[j].p.be == steps[i].p.be && !wasSeen[steps[j].p.gradStorage()] {
			wasSeen[steps[j].p.gradStorage()] = true
			j++
		}
		multi(mt, steps[i:j])
		i = j
	}
}

// repeated listede aynı parametrenin (depolamanın) birden çok kez geçip geçmediği. Öyleyse
// iyileştiriciler adımı parametre parametre (hedef, güncelleme, geri yazma sırasıyla) yapar:
// ikinci geçiş birincinin sonucunu görür.
func repeated(params []*Tensor) bool {
	wasSeen := make(map[Storage]bool, len(params))
	for _, p := range params {
		if wasSeen[p.storage] {
			return true
		}
		wasSeen[p.storage] = true
	}
	return false
}

// ZeroGrad tüm parametre gradyanlarını sıfırlar.
func (o *SGD) ZeroGrad() { ZeroGrads(o.Params) }

// AdamW ayrık ağırlık çürümeli uyarlanır moment tahmini iyileştiricisi:
//
//	p ← p - lr·wd·p
//	m ← β1·m + (1-β1)·g,  v ← β2·v + (1-β2)·g²
//	p ← p - lr · m̂ / (√v̂ + eps)   (m̂, v̂ sapma düzeltmeli)
type AdamW struct {
	Params      []*Tensor
	LR          float32
	Beta1       float32
	Beta2       float32
	Eps         float32
	WeightDecay float32
	m, v        []Storage
	step        int   // Step çağrısı sayısı
	steps       []int // parametre başına güncelleme sayısı (sapma düzeltmesi bunu kullanır)
	master      masterWeights
}

// NewAdamW varsayılan β1=0.9, β2=0.999, eps=1e-8 ile AdamW kurar.
func NewAdamW(params []*Tensor, lr, weightDecay float32) *AdamW {
	checkParams(params)
	return &AdamW{
		Params: params, LR: lr, Beta1: 0.9, Beta2: 0.999, Eps: 1e-8, WeightDecay: weightDecay,
		m: make([]Storage, len(params)), v: make([]Storage, len(params)), steps: make([]int, len(params)),
		master: newMasterWeights(len(params)),
	}
}

// SetMasterWeights 16 bitlik parametreler için float32 ana ağırlık kopyasını
// açar ya da kapatır (varsayılan açık). Moment tahminleri her zaman float32'dir.
func (o *AdamW) SetMasterWeights(on bool) { o.master.configure(on) }

// StepCount şimdiye kadar yapılan adım sayısı.
func (o *AdamW) StepCount() int { return o.step }

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır. Sapma
// düzeltmesi parametrenin kendi güncelleme sayısıyla yapılır: bazı adımlarda gradyan
// almayan bir parametre (ör. o adımda kullanılmayan gömme satırları değil, hiç gradyan
// akmayan bir parametre) ilk güncellemesinde yeni bir iyileştiricideki gibi davranır.
func (o *AdamW) Step() {
	sy := stepCounter(o.Params, OpKindAdamW).start()
	defer sy.finish()
	o.step++
	sorted := repeated(o.Params)
	var steps []goodStep
	for k, p := range o.Params {
		if p.gradStorage() == nil {
			continue
		}
		o.steps[k]++
		t := float64(o.steps[k])
		if o.m[k] == nil {
			o.m[k] = p.be.Alloc(p.Numel(), Float32)
			o.v[k] = p.be.Alloc(p.Numel(), Float32)
		}
		a := goodStep{k: k, p: p, m: o.m[k], v: o.v[k],
			bc1: float32(1 - math.Pow(float64(o.Beta1), t)), bc2: float32(1 - math.Pow(float64(o.Beta2), t))}
		a.h = o.master.target(k, p)
		if sorted {
			p.be.AdamWStep(a.h, p.gradStorage(), a.m, a.v, o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, a.bc1, a.bc2)
			o.master.write(k, p, a.h)
			continue
		}
		steps = append(steps, a)
	}
	multiApply(steps, func(be MultiTensorOps, a []goodStep) {
		h, g, m := a[0].cols(a)
		v, bc1, bc2 := make([]Storage, len(a)), make([]float32, len(a)), make([]float32, len(a))
		for i, x := range a {
			v[i], bc1[i], bc2[i] = x.v, x.bc1, x.bc2
		}
		be.AdamWStepAll(h, g, m, v, o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, bc1, bc2)
	}, func(a goodStep) {
		a.p.be.AdamWStep(a.h, a.p.gradStorage(), a.m, a.v, o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, a.bc1, a.bc2)
	})
	for _, a := range steps {
		o.master.write(a.k, a.p, a.h)
	}
}

// ZeroGrad tüm parametre gradyanlarını sıfırlar.
func (o *AdamW) ZeroGrad() { ZeroGrads(o.Params) }

// ClipGradNorm parametre gradyanlarının toplam L2 normunu en fazla maxNorm
// olacak şekilde ölçekler ve kırpma öncesi normu döndürür.
func ClipGradNorm(params []*Tensor, maxNorm float64) float64 {
	params = distinctGrads(params)
	if len(params) == 0 {
		return 0
	}
	sy := counterStart(params[0].be, OpKindClipGrad).start()
	defer sy.finish()
	// Norm float32'de taşarsa (ör. [3e38, 3e38]) sonsuzdur: ölçek 0, sonlu gradyanlar 0 olur
	// (PyTorch 2.14 float32 normla aynı; GPU çekirdeği de normu float32'ye yuvarlayıp böler).
	norm := float64(float32(math.Sqrt(gradSquareSum(params))))
	var steps []goodStep
	for _, p := range params {
		if p.gradStorage() != nil {
			steps = append(steps, goodStep{p: p})
		}
	}
	if norm > maxNorm && norm > 0 {
		k := float32(maxNorm / norm)
		multiApply(steps, func(be MultiTensorOps, a []goodStep) {
			_, g, _ := a[0].cols(a)
			be.ScaleAll(g, k)
		}, func(a goodStep) { a.p.be.Scale(a.p.gradStorage(), k) })
	}
	// Kırpma gradyanları yerinde yazar sayılır (PyTorch clip_grad_norm_ ölçek 1 olsa da
	// gradyanı yerinde çarpar ve sürümü artırır); GPU yolu ölçeği cihazda seçtiği için aynı
	// kuralla iki cihaz aynı davranır. Sınırsız yoklama (en_fazla +∞, sonlu_değilse_hata)
	// gradyanlara dokunmaz.
	if !math.IsInf(maxNorm, 1) {
		for _, a := range steps {
			a.p.gradStorage().BumpVersion()
		}
	}
	return norm
}

// GradClipper gradyanların toplam L2 normunu ve kırpmayı cihazda yapan arka uçların isteğe
// bağlı arayüzüdür: konak beklemez (GPU'da norm ve ölçek de sıradaki çekirdeklerce
// hesaplanır). Dönen tek elemanlı float32 depolama kırpma öncesi normdur; ok yanlışsa
// (ör. 16 bitlik gradyan) arka uç işi yapmamıştır.
type GradClipper interface {
	ClipGradAll(grads []Storage, maxNorm float32) (norm Storage, ok bool)
}

// ClipGradNormTensor ClipGradNorm'dur, kırpma öncesi normu 0 boyutlu tensör olarak verir.
// Bütün gradyanlar GradClipper olan tek arka uçtaysa norm ve kırpma cihazda yapılır ve
// konak beklemez; norm ancak okunduğunda (.değer, yazdırma) beklenir. Değilse norm konakta
// hesaplanır (ClipGradNorm) ve CPU tensörü olarak döner.
func ClipGradNormTensor(params []*Tensor, maxNorm float64) *Tensor {
	params = distinctGrads(params)
	var be Backend
	var grads []Storage
	single := true
	for _, p := range params {
		if p.gradStorage() == nil {
			continue
		}
		if be == nil {
			be = p.be
		} else if p.be != be {
			single = false
		}
		grads = append(grads, p.gradStorage())
	}
	if gc, ok := be.(GradClipper); ok && single {
		sy := counterStart(be, OpKindClipGrad).start()
		norm, ok := gc.ClipGradAll(grads, float32(maxNorm))
		sy.finish()
		if ok {
			if !math.IsInf(maxNorm, 1) { // sınırsız yoklama gradyanlara dokunmaz (ClipGradNorm)
				for _, g := range grads {
					g.BumpVersion()
				}
			}
			return fromStorage(be, norm, nil)
		}
	}
	return NewOn(CPU, []float32{float32(ClipGradNorm(params, maxNorm))})
}

// distinctGrads gradyanı olan parametreleri her gradyan deposu bir kez olacak biçimde (ilk
// görülme sırasıyla) verir: listede iki kez geçen (bağlı, paylaşılan) bir parametrenin
// gradyanı norma iki kez girmez, kırpmada iki kez ölçeklenmez (k² olurdu) ve GPU'da aynı
// depolama aynı çekirdeğe iki kez verilmez.
func distinctGrads(params []*Tensor) []*Tensor {
	seen := make(map[Storage]bool, len(params))
	out := make([]*Tensor, 0, len(params))
	for _, p := range params {
		if p.gradStorage() == nil || seen[p.gradStorage()] {
			continue
		}
		seen[p.gradStorage()] = true
		out = append(out, p)
	}
	return out
}

// gradSquareSum gradyanların kareler toplamını verir. Toplu kareler toplamı veren
// (SquareSummer) arka uçlarda aynı arka uçtaki ardışık gradyanlar tek çağrıda
// toplanır (GPU'da tek eşzamanlama); toplama sırası parametre sırasıdır.
func gradSquareSum(params []*Tensor) float64 {
	var s float64
	for i := 0; i < len(params); {
		p := params[i]
		if p.gradStorage() == nil {
			i++
			continue
		}
		ss, ok := p.be.(SquareSummer)
		if !ok {
			s += p.be.SumSquares(p.gradStorage())
			i++
			continue
		}
		var group []Storage
		for ; i < len(params) && (params[i].gradStorage() == nil || params[i].be == p.be); i++ {
			if params[i].gradStorage() != nil {
				group = append(group, params[i].gradStorage())
			}
		}
		s += ss.SumSquaresAll(group)
	}
	return s
}

// ----------------------------------------------------------------------------
// İyileştirici durumu (sürdürme)
// ----------------------------------------------------------------------------
//
// State iyileştiricinin bütün durumunu adlandırılmış tensörler olarak (kopya) verir;
// LoadState aynı parametre listesiyle kurulmuş yeni bir iyileştiriciye yükler. Böylece
// kontrol noktasından sürdürülen eğitim, kesintisiz eğitimle bit düzeyinde aynı devam eder.
// Anahtarlar parametrenin listedeki sırasını taşır: "m.k", "v.k" (AdamW momentleri),
// "hiz.k" (SGD momentumu), "adim.k" ve "adim" (adım sayıları), "ana.k" (16 bitlik
// parametrelerin float32 ana kopyası). Sözlük doğrudan dosyaya yazılabilir. Öğrenme oranı
// gibi ayarlar duruma dahil değildir (kurulumda verilir).

// counterTensor n'yi 2^24'e bölünmüş iki float32 parça olarak kayıpsız taşır.
func counterTensor(n int) *Tensor {
	return New([]float32{float32(n >> 24), float32(n & (1<<24 - 1))}, 2)
}

// stateTensor s'yi p'nin şeklinde tensör olarak verir: kopya ise bitişik bir kopyası
// (State), değilse depolamanın kendisinin görünümü (DurumGorunumu).
func stateTensor(p *Tensor, s Storage, dup bool) *Tensor {
	if !dup {
		return fromStorage(p.be, s, p.shape)
	}
	return stateCopy(p, s)
}

// stateCopy depolamanın bitişik float32 kopyasını tensör olarak verir.
func stateCopy(p *Tensor, s Storage) *Tensor {
	c := p.be.Alloc(s.Len(), s.DType())
	p.be.Copy(c, 0, View{S: s, Shape: p.shape, Strides: contiguousStrides(p.shape)})
	return fromStorage(p.be, c, p.shape)
}

// anaDurum ana kopyaları duruma ekler.
func (a *masterWeights) state(st map[string]*Tensor, params []*Tensor, dup bool) {
	for k, p := range params {
		if a.master[k] != nil && !a.closed {
			st[fmt.Sprintf("ana.%d", k)] = stateTensor(p, a.master[k], dup)
		}
	}
}

// anaYukle ana kopyaları durumdan yükler; parametre o andaki sürümüyle eşleşir (ana kopya
// parametreden yeniden alınmaz).
func (a *masterWeights) loadFn(st map[string]*Tensor, params []*Tensor) {
	for k, p := range params {
		a.master[k] = nil
		if c := loadState(st, fmt.Sprintf("ana.%d", k), p); c != nil && !a.closed && p.dtyp != Float32 {
			a.master[k] = c
			a.version[k] = p.storage.Version()
		}
	}
}

// State AdamW'nin durumunu verir (bkz. yukarıdaki açıklama).
func (o *AdamW) State() map[string]*Tensor { return o.state(true) }

// StateView State gibidir ama kopya almaz: moment ve ana kopya tensörleri
// iyileştiricinin kendi depolarının görünümleridir (adım sayaçları yeni tensördür). Yalnızca
// okunmalı ve bir sonraki Step'ten önce kullanılmalıdır. Kontrol noktası durumu cihazda
// ikinci bir kopya ayırmadan konağa indirmek için kullanır (büyük modelde GPU belleği).
func (o *AdamW) StateView() map[string]*Tensor { return o.state(false) }

func (o *AdamW) state(dup bool) map[string]*Tensor {
	st := map[string]*Tensor{"adim": counterTensor(o.step)}
	for k, p := range o.Params {
		if o.m[k] != nil {
			st[fmt.Sprintf("m.%d", k)] = stateTensor(p, o.m[k], dup)
			st[fmt.Sprintf("v.%d", k)] = stateTensor(p, o.v[k], dup)
		}
		if o.steps[k] != 0 {
			st[fmt.Sprintf("adim.%d", k)] = counterTensor(o.steps[k])
		}
	}
	o.master.state(st, o.Params, dup)
	return st
}

// LoadState State ile alınmış durumu yükler; parametre sayısı ve şekilleri aynı olmalıdır.
func (o *AdamW) LoadState(st map[string]*Tensor) {
	o.CheckState(st)
	checkStateDevices(st, o.Params, "m", "v", "ana")
	o.step = 0
	if t, ok := st["adim"]; ok {
		o.step = readCounter("adim", t)
	}
	for k, p := range o.Params {
		o.m[k] = loadState(st, fmt.Sprintf("m.%d", k), p)
		o.v[k] = loadState(st, fmt.Sprintf("v.%d", k), p)
		if (o.m[k] == nil) != (o.v[k] == nil) {
			panic(messages.New(messages.H3061, k))
		}
		o.steps[k] = 0
		if t, ok := st[fmt.Sprintf("adim.%d", k)]; ok {
			o.steps[k] = readCounter(fmt.Sprintf("adim.%d", k), t)
		}
		// Momentler sapma düzeltmesini adım sayısıyla birlikte anlamlıdır: momentli ama
		// sayaçsız durum tutarsızdır (momentler ilk adımın momentleri sanılırdı).
		if o.m[k] != nil && o.steps[k] == 0 {
			panic(messages.New(messages.H3062, k))
		}
	}
	o.master.loadFn(st, o.Params)
}

// State SGD'nin durumunu (momentum hızları ve ana kopyalar) verir.
func (o *SGD) State() map[string]*Tensor { return o.state(true) }

// StateView State gibidir ama kopya almaz (bkz. AdamW.DurumGorunumu).
func (o *SGD) StateView() map[string]*Tensor { return o.state(false) }

func (o *SGD) state(dup bool) map[string]*Tensor {
	st := map[string]*Tensor{}
	for k, p := range o.Params {
		if o.speed[k] != nil {
			st[fmt.Sprintf("hiz.%d", k)] = stateTensor(p, o.speed[k], dup)
		}
	}
	o.master.state(st, o.Params, dup)
	return st
}

// LoadState State ile alınmış durumu yükler; parametre sayısı ve şekilleri aynı olmalıdır.
func (o *SGD) LoadState(st map[string]*Tensor) {
	o.CheckState(st)
	checkStateDevices(st, o.Params, "hiz", "ana")
	for k, p := range o.Params {
		o.speed[k] = loadState(st, fmt.Sprintf("hiz.%d", k), p)
	}
	o.master.loadFn(st, o.Params)
}

package tensor

import (
	"fmt"
	"math"
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
		if !p.requiresGrad {
			panic("tensor: iyileştirici parametreleri gradyan istemeli (SetRequiresGrad(true))")
		}
		if !p.IsContiguous() || p.offset != 0 || p.storage.Len() != p.Numel() {
			panic(fmt.Sprintf("tensor: iyileştirici parametresi kendi depolamasına sahip bitişik bir tensör olmalı (şekil %v)", p.shape))
		}
	}
}

// anaAgirliklar 16 bitlik parametreler için float32 ana ağırlık kopyalarını
// yönetir. Güncelleme ana kopyada yapılır ve parametreye yuvarlanarak yazılır;
// böylece parametrenin 16 bitlik çözünürlüğünden küçük adımlar kaybolmaz.
// Parametre iyileştirici dışında değiştirilirse (sürümü son yazımdan farklıysa)
// ana kopya parametreden yeniden alınır.
type anaAgirliklar struct {
	kapali bool
	ana    []Storage // parametre başına float32 kopya (gerekmiyorsa nil)
	surum  []uint64  // son yazımdan sonraki parametre sürümü
}

func yeniAnaAgirliklar(n int) anaAgirliklar {
	return anaAgirliklar{ana: make([]Storage, n), surum: make([]uint64, n)}
}

// hedef k. parametrenin güncelleneceği depolama.
func (a *anaAgirliklar) hedef(k int, p *Tensor) Storage {
	if a.kapali || p.tur == Float32 {
		return p.storage
	}
	if a.ana[k] == nil || p.storage.Version() != a.surum[k] {
		if a.ana[k] == nil {
			a.ana[k] = p.be.Alloc(p.Numel(), Float32)
		}
		p.be.Copy(a.ana[k], 0, p.view())
	}
	return a.ana[k]
}

// yaz güncellenmiş hedefi (ana kopyaysa yuvarlayarak) parametreye yazar ve
// parametrenin sürümünü artırır (yerinde değişiklik).
func (a *anaAgirliklar) yaz(k int, p *Tensor, h Storage) {
	if h != p.storage {
		p.be.Copy(p.storage, 0, View{S: h, Shape: p.shape, Strides: contiguousStrides(p.shape)})
	}
	p.storage.BumpVersion()
	a.surum[k] = p.storage.Version()
}

// ayarla ana kopya kullanımını açar/kapatır.
func (a *anaAgirliklar) ayarla(acik bool) {
	a.kapali = !acik
	if !acik {
		clear(a.ana)
	}
}

// SGD momentumlu (isteğe bağlı) olasılıksal gradyan inişi. WeightDecay,
// gradyana L2 cezası olarak eklenir.
type SGD struct {
	Params      []*Tensor
	LR          float32
	Momentum    float32
	WeightDecay float32
	hiz         []Storage
	ana         anaAgirliklar
}

// NewSGD yeni bir SGD iyileştirici kurar.
func NewSGD(params []*Tensor, lr, momentum float32) *SGD {
	checkParams(params)
	return &SGD{Params: params, LR: lr, Momentum: momentum, hiz: make([]Storage, len(params)),
		ana: yeniAnaAgirliklar(len(params))}
}

// SetMasterWeights 16 bitlik parametreler için float32 ana ağırlık kopyasını
// açar ya da kapatır (varsayılan açık). Kapalıyken güncelleme doğrudan 16
// bitlik parametre üzerinde (float32'de hesaplanıp yuvarlanarak) yapılır.
func (o *SGD) SetMasterWeights(on bool) { o.ana.ayarla(on) }

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır.
// Parametreler yerinde değiştiği için sürümleri artar.
func (o *SGD) Step() {
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		if o.Momentum != 0 && o.hiz[k] == nil {
			o.hiz[k] = p.be.Alloc(p.Numel(), Float32)
		}
		h := o.ana.hedef(k, p)
		p.be.SGDStep(h, p.grad, o.hiz[k], o.LR, o.Momentum, o.WeightDecay)
		o.ana.yaz(k, p, h)
	}
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
	adim        int
	ana         anaAgirliklar
}

// NewAdamW varsayılan β1=0.9, β2=0.999, eps=1e-8 ile AdamW kurar.
func NewAdamW(params []*Tensor, lr, weightDecay float32) *AdamW {
	checkParams(params)
	return &AdamW{
		Params: params, LR: lr, Beta1: 0.9, Beta2: 0.999, Eps: 1e-8, WeightDecay: weightDecay,
		m: make([]Storage, len(params)), v: make([]Storage, len(params)),
		ana: yeniAnaAgirliklar(len(params)),
	}
}

// SetMasterWeights 16 bitlik parametreler için float32 ana ağırlık kopyasını
// açar ya da kapatır (varsayılan açık). Moment tahminleri her zaman float32'dir.
func (o *AdamW) SetMasterWeights(on bool) { o.ana.ayarla(on) }

// StepCount şimdiye kadar yapılan adım sayısı.
func (o *AdamW) StepCount() int { return o.adim }

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır.
func (o *AdamW) Step() {
	o.adim++
	bc1 := float32(1 - math.Pow(float64(o.Beta1), float64(o.adim)))
	bc2 := float32(1 - math.Pow(float64(o.Beta2), float64(o.adim)))
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		if o.m[k] == nil {
			o.m[k] = p.be.Alloc(p.Numel(), Float32)
			o.v[k] = p.be.Alloc(p.Numel(), Float32)
		}
		h := o.ana.hedef(k, p)
		p.be.AdamWStep(h, p.grad, o.m[k], o.v[k], o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, bc1, bc2)
		o.ana.yaz(k, p, h)
	}
}

// ZeroGrad tüm parametre gradyanlarını sıfırlar.
func (o *AdamW) ZeroGrad() { ZeroGrads(o.Params) }

// ClipGradNorm parametre gradyanlarının toplam L2 normunu en fazla maxNorm
// olacak şekilde ölçekler ve kırpma öncesi normu döndürür.
func ClipGradNorm(params []*Tensor, maxNorm float64) float64 {
	var s float64
	for _, p := range params {
		if p.grad != nil {
			s += p.be.SumSquares(p.grad)
		}
	}
	norm := math.Sqrt(s)
	if norm > maxNorm && norm > 0 {
		k := float32(maxNorm / norm)
		for _, p := range params {
			if p.grad != nil {
				p.be.Scale(p.grad, k)
				p.grad.BumpVersion()
			}
		}
	}
	return norm
}

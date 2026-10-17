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

// SGD momentumlu (isteğe bağlı) olasılıksal gradyan inişi. WeightDecay,
// gradyana L2 cezası olarak eklenir.
type SGD struct {
	Params      []*Tensor
	LR          float32
	Momentum    float32
	WeightDecay float32
	hiz         []Storage
}

// NewSGD yeni bir SGD iyileştirici kurar.
func NewSGD(params []*Tensor, lr, momentum float32) *SGD {
	checkParams(params)
	return &SGD{Params: params, LR: lr, Momentum: momentum, hiz: make([]Storage, len(params))}
}

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır.
func (o *SGD) Step() {
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		if o.Momentum != 0 && o.hiz[k] == nil {
			o.hiz[k] = p.be.Alloc(p.Numel())
		}
		p.be.SGDStep(p.storage, p.grad, o.hiz[k], o.LR, o.Momentum, o.WeightDecay)
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
}

// NewAdamW varsayılan β1=0.9, β2=0.999, eps=1e-8 ile AdamW kurar.
func NewAdamW(params []*Tensor, lr, weightDecay float32) *AdamW {
	checkParams(params)
	return &AdamW{
		Params: params, LR: lr, Beta1: 0.9, Beta2: 0.999, Eps: 1e-8, WeightDecay: weightDecay,
		m: make([]Storage, len(params)), v: make([]Storage, len(params)),
	}
}

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
			o.m[k] = p.be.Alloc(p.Numel())
			o.v[k] = p.be.Alloc(p.Numel())
		}
		p.be.AdamWStep(p.storage, p.grad, o.m[k], o.v[k], o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, bc1, bc2)
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
			}
		}
	}
	return norm
}

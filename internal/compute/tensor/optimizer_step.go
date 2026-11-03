package tensor

import (
	"math"
	"unsafe"
)

// İyileştiricinin adımı tek çekirdektir (dil tanımı 10.2, S-kilit). Adım bütün parametreleri
// günceller. Kilit bırakılarak çalışırsa izinlerin hepsi birlikte alınır: parametreler, ana
// kopyalar ve durum depoları yazma, gradyanlar okuma izniyle. Öbür görevler adım bitene dek hiçbir
// parametreye dokunamaz (kapı bekletir); yarım güncellenmiş parametre görmez (DENETİM istek 64,
// (4a)). Hazırlık yinelenebilir: yalnız durum depolarını ayırır, depoları ve izinleri toplar.
// Sayaçlar, sürümler ve oranlar çekirdeğin başında (begin) kilit altında okunur ve yazılır;
// gövde yalnız onların kopyasıyla ve hazırlıkta alınan depolarla çalışır.

// stepEntry bir parametrenin bu adımdaki güncellemesidir: param parametrenin görünümü (ana kopya
// ondan tazelenir, ona geri yazılır); fresh ana kopyanın tazeleneceğidir (begin'de belirlenir).
type stepEntry struct {
	goodStep
	p     *Tensor
	param View
	fresh bool
}

// stepHyper adımın oranlarıdır; begin'de iyileştiriciden kopyalanır (gövde sürerken başka görev
// oranı değiştirebilir).
type stepHyper struct {
	sgd                           bool
	lr, momentum, b1, b2, eps, wd float32
}

// stepPlan bir adımın gradyanı olan parametreleri (sırasıyla; aynı parametre birden çok kez
// geçebilir), izinleri ve oranlarıdır.
type stepPlan struct {
	entries       []stepEntry
	sorted        bool
	writes, reads []Storage
	hp            stepHyper
}

// planStep adımın planını kurar: state, k. parametrenin durum depolarıdır (gerekiyorsa ayırır).
func planStep(params []*Tensor, master *masterWeights, state func(k int, p *Tensor) (m, v Storage)) *stepPlan {
	pl := &stepPlan{sorted: repeated(params)}
	add := func(ss []Storage, s Storage) []Storage {
		if s == nil || containsStorage(ss, s) {
			return ss
		}
		return append(ss, s)
	}
	for k, p := range params {
		g := p.gradStorage()
		if g == nil || !p.GradAdded() {
			continue // gradyanı eklenmemiş parametre atlanır: ağırlık, moment, sayaç değişmez (15.34)
		}
		m, v := state(k, p)
		e := stepEntry{goodStep: goodStep{k: k, be: p.be, g: g, h: master.hold(k, p), m: m, v: v}, p: p,
			param: View{S: p.storage, Shape: p.shape, Strides: ContiguousStrides(p.shape)}}
		pl.entries = append(pl.entries, e)
		for _, s := range []Storage{p.storage, e.h, m, v} {
			pl.writes = add(pl.writes, s)
		}
		pl.reads = add(pl.reads, g)
	}
	return pl
}

// mark adımın başında (begin) her girdinin ana kopyasının tazelenip tazelenmeyeceğini belirler,
// parametrenin sürümünü artırır (yerinde değişiklik) ve "gradyan eklendi" bilgisini by adına
// tüketir (15.34); sıra korunur: aynı parametrenin ikinci geçişi birincinin sonucunu görür.
func (pl *stepPlan) mark(master *masterWeights, by Optimizer) {
	for i := range pl.entries {
		e := &pl.entries[i]
		e.p.n.added, e.p.n.stepper = false, by
		e.fresh = master.stale(e.k, e.p, e.h)
		e.p.storage.BumpVersion()
		master.version[e.k] = e.p.storage.Version()
	}
}

// kernel adımın çekirdeğidir. Gövde her girdinin ana kopyasını tazeler, günceller ve
// parametreye geri yazar; ardışık, çok tensörlü işi destekleyen girdiler tek çağrıda güncellenir
// (multiApply). begin oranları ve sayaçları kurar.
func (pl *stepPlan) kernel(o unsafe.Pointer, params []*Tensor, is OpKind, begin func()) *Kernel {
	be := Backend(cpuBackend{})
	if len(params) > 0 {
		be = params[0].be
	}
	var sy counterMeasure
	hp := &pl.hp
	k := newKernel(be, pl.writes, pl.reads, params, func() {
		update := func(a goodStep) {
			if hp.sgd {
				a.be.SGDStep(a.h, a.g, a.m, hp.lr, hp.momentum, hp.wd)
			} else {
				a.be.AdamWStep(a.h, a.g, a.m, a.v, hp.lr, hp.b1, hp.b2, hp.eps, hp.wd, a.bc1, a.bc2)
			}
		}
		multi := func(mt MultiTensorOps, a []goodStep) {
			h, g, m := a[0].cols(a)
			if hp.sgd {
				mt.SGDStepAll(h, g, m, hp.lr, hp.momentum, hp.wd)
				return
			}
			v, bc1, bc2 := make([]Storage, len(a)), make([]float32, len(a)), make([]float32, len(a))
			for i, x := range a {
				v[i], bc1[i], bc2[i] = x.v, x.bc1, x.bc2
			}
			mt.AdamWStepAll(h, g, m, v, hp.lr, hp.b1, hp.b2, hp.eps, hp.wd, bc1, bc2)
		}
		refresh := func(e stepEntry) {
			if e.fresh {
				e.be.Copy(e.h, 0, e.param)
			}
		}
		writeBack := func(e stepEntry) {
			if e.h != e.param.S {
				e.be.Copy(e.param.S, 0, View{S: e.h, Shape: e.param.Shape, Strides: e.param.Strides})
			}
		}
		if pl.sorted {
			for _, e := range pl.entries {
				refresh(e)
				update(e.goodStep)
				writeBack(e)
			}
		} else {
			steps := make([]goodStep, len(pl.entries))
			for i, e := range pl.entries {
				refresh(e)
				steps[i] = e.goodStep
			}
			multiApply(steps, multi, update)
			for _, e := range pl.entries {
				writeBack(e)
			}
		}
		sy.finish()
	})
	k.begin = func() {
		sy = stepCounter(params, is).start()
		begin()
	}
	k.end = func() { optimizerStepped(o) }
	return k
}

// StepWork adımın güncelleyeceği öğe sayısıdır (gradyanı olan parametreler; kilidi bırakma
// eşiği için).
func StepWork(params []*Tensor) int {
	n := 0
	for _, p := range params {
		if p.gradStorage() != nil && p.GradAdded() {
			n += p.Numel()
		}
	}
	return n
}

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır.
// Parametreler yerinde değiştiği için sürümleri artar.
func (o *SGD) Step() { o.StepKernel().RunLocked() }

// StepKernel Step'in hazırlığıdır (kilit altında, yinelenebilir); güncelleme dönen çekirdektir.
func (o *SGD) StepKernel() *Kernel {
	pl := planStep(o.Params, &o.master, func(k int, p *Tensor) (Storage, Storage) {
		if o.Momentum != 0 && o.speed[k] == nil {
			o.speed[k] = p.be.Alloc(p.Numel(), Float32)
		}
		return o.speed[k], nil
	})
	return pl.kernel(unsafe.Pointer(o), o.Params, OpKindSGD, func() {
		pl.hp = stepHyper{sgd: true, lr: o.LR, momentum: o.Momentum, wd: o.WeightDecay}
		pl.mark(&o.master, o)
	})
}

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır. Sapma
// düzeltmesi parametrenin kendi güncelleme sayısıyla yapılır: bazı adımlarda gradyan
// almayan bir parametre (ör. o adımda kullanılmayan gömme satırları değil, hiç gradyan
// akmayan bir parametre) ilk güncellemesinde yeni bir iyileştiricideki gibi davranır.
func (o *AdamW) Step() { o.StepKernel().RunLocked() }

// StepKernel Step'in hazırlığıdır (kilit altında, yinelenebilir); güncelleme dönen çekirdektir.
func (o *AdamW) StepKernel() *Kernel {
	pl := planStep(o.Params, &o.master, func(k int, p *Tensor) (Storage, Storage) {
		if o.m[k] == nil {
			o.m[k] = p.be.Alloc(p.Numel(), Float32)
			o.v[k] = p.be.Alloc(p.Numel(), Float32)
		}
		return o.m[k], o.v[k]
	})
	return pl.kernel(unsafe.Pointer(o), o.Params, OpKindAdamW, func() {
		pl.hp = stepHyper{lr: o.LR, b1: o.Beta1, b2: o.Beta2, eps: o.Eps, wd: o.WeightDecay}
		o.step++
		for i := range pl.entries {
			e := &pl.entries[i]
			o.steps[e.k]++
			t := float64(o.steps[e.k])
			e.bc1, e.bc2 = float32(1-math.Pow(float64(o.Beta1), t)), float32(1-math.Pow(float64(o.Beta2), t))
		}
		pl.mark(&o.master, o)
	})
}

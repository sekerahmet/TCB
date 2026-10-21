package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sinir ağı katmanları için birleşik (tek düğümlü) işlemler; cihazdan bağımsız
// katman. Hesap arka uçta yapılır.

// Embedding w (V×D) tablosundan idx satırlarını seçer. shape verilirse çıktı
// şekli shape + [D] olur (ör. (B, T) indeksler için (B, T, D)); verilmezse
// (len(idx), D).
func Embedding(w *Tensor, idx []int, shape ...int) *Tensor {
	if len(w.shape) != 2 {
		panic(messages.New(messages.H3041, w.shape))
	}
	V, D := w.shape[0], w.shape[1]
	sy := counterStart(w.be, OpKindEmbedding).start()
	if len(shape) == 0 {
		shape = []int{len(idx)}
	}
	if n, overflow := elemCount(shape); overflow || n != len(idx) {
		panic(messages.New(messages.H3042, len(idx), shape))
	}
	for _, ix := range idx {
		if ix < 0 || ix >= V {
			panic(messages.New(messages.H3043, ix, V))
		}
	}
	ix := append([]int(nil), idx...)
	out := emptyOf(isEmbedding, w.be, append(copyInts(shape), D), w.dtyp)
	w.be.Embedding(out.storage, w.view(), ix, D)
	r := track(out, "embedding", []*Tensor{w}, func(o *node) {
		wn := o.parents[0]
		wn.be.EmbeddingGrad(wn.accGrad(), o.grad, ix, D)
	})
	sy.finish()
	return r
}

// CrossEntropy logits (..., C) ve hedef sınıf indeksleri (satır başına bir
// tane) için ortalama çapraz entropi kaybını verir. Negatif hedefler yok
// sayılır (dolgu için) ve ortalamaya katılmaz. Log-softmax ile birleşik ve
// sayısal kararlıdır; sonuç skalerdir.
func CrossEntropy(logits *Tensor, targets []int) *Tensor {
	if len(logits.shape) < 1 {
		panic(messages.New(messages.H3044))
	}
	sy := counterStart(logits.be, OpKindCrossEntropy).start()
	C := logits.shape[len(logits.shape)-1]
	R := logits.Numel() / max(C, 1)
	if len(targets) != R {
		panic(messages.New(messages.H3045, R, len(targets)))
	}
	valid := 0
	for _, t := range targets {
		if t >= C {
			panic(messages.New(messages.H3046, t, C))
		}
		if t >= 0 {
			valid++
		}
	}
	if valid == 0 {
		// Ortalama boş küme üzerinden alınırdı: tanımsız; sessizce 0 dönmesin.
		panic(messages.New(messages.H3047, R))
	}
	tg := append([]int(nil), targets...)
	be := logits.be
	loss, lse := be.CrossEntropy(logits.view(), tg, C)
	out := fromStorage(be, loss, nil)
	lv := logits.view()
	r := track(out, "cross_entropy", []*Tensor{logits}, func(o *node) {
		ln := o.parents[0]
		cs, writer := be.(CrossEntropyGradSetter)
		if gx, fresh := ln.freshGrad(writer); fresh {
			cs.CrossEntropyGradSet(gx, o.grad, lv, lse, tg, C)
			return
		}
		be.CrossEntropyGrad(ln.accGrad(), o.grad, lv, lse, tg, C)
	}, logits)
	sy.finish()
	return r
}

func optView(t *Tensor) *View {
	if t == nil {
		return nil
	}
	v := t.view()
	return &v
}

// optGrad düğüm gradyan istiyorsa gradyan depolamasını, aksi halde nil verir.
func optGrad(n *node) Storage {
	if !n.wants() {
		return nil
	}
	return n.accGrad()
}

// LayerNorm son eksen boyunca katman normalleştirmesi: (x-μ)/√(σ²+eps)·γ + β.
// gamma ve beta (D) şeklinde olmalı; nil verilebilirler. Çıktı girdinin türündedir.
func LayerNorm(x, gamma, beta *Tensor, eps float32) *Tensor {
	return LayerNormAs(x, gamma, beta, eps, x.dtyp)
}

// LayerNormAs LayerNorm'dur, çıktısı dt türündedir: hesap her türde float32'dir, çıktı yazılırken
// dt'ye yuvarlanır (ardından gelen çarpım için ayrı bir dönüşüm gerekmez; 15.13).
func LayerNormAs(x, gamma, beta *Tensor, eps float32, dt DType) *Tensor {
	checkNormDims("LayerNorm", x)
	D := x.shape[len(x.shape)-1]
	checkNormParam("LayerNorm", gamma, D)
	checkNormParam("LayerNorm", beta, D)
	be := sameDevice("LayerNorm", x, gamma, beta)
	sy := counterStart(be, OpKindLayerNorm).start()
	out := emptyOf(isNormalize, be, x.shape, dt)
	gv := optView(gamma)
	mean, rstd := be.LayerNorm(out.storage, x.view(), gv, optView(beta), D, eps)
	// Girdiler hep aynı konumdadır (x, gamma, beta; olmayan nil): geri hesap düğümlerini
	// konumla bulur.
	xv := x.view()
	r := track(out, "layer_norm", []*Tensor{x, gamma, beta}, func(o *node) {
		be.LayerNormGrad(optGrad(o.parents[0]), optGrad(o.parents[1]), optGrad(o.parents[2]), o.grad, xv, gv, mean, rstd, D)
	}, x, gamma)
	sy.finish()
	return r
}

// RMSNorm son eksen boyunca karekök-ortalama-kare normalleştirmesi:
// x / √(ort(x²)+eps) · γ. gamma (D) şeklinde olmalı; nil verilebilir. Çıktı girdinin türündedir.
func RMSNorm(x, gamma *Tensor, eps float32) *Tensor {
	return RMSNormAs(x, gamma, eps, x.dtyp)
}

// RMSNormAs RMSNorm'dur, çıktısı dt türündedir (bkz. LayerNormAs).
func RMSNormAs(x, gamma *Tensor, eps float32, dt DType) *Tensor {
	checkNormDims("RMSNorm", x)
	D := x.shape[len(x.shape)-1]
	checkNormParam("RMSNorm", gamma, D)
	be := sameDevice("RMSNorm", x, gamma)
	sy := counterStart(be, OpKindRMSNorm).start()
	out := emptyOf(isNormalize, be, x.shape, dt)
	gv := optView(gamma)
	rinv := be.RMSNorm(out.storage, x.view(), gv, D, eps)
	xv := x.view()
	r := track(out, "rms_norm", []*Tensor{x, gamma}, func(o *node) {
		be.RMSNormGrad(optGrad(o.parents[0]), optGrad(o.parents[1]), o.grad, xv, gv, rinv, D)
	}, x, gamma)
	sy.finish()
	return r
}

// checkNormDims normalleştirilecek girdinin en az 1 boyutlu olduğunu denetler.
func checkNormDims(op string, x *Tensor) {
	if len(x.shape) == 0 {
		panic(messages.New(messages.H3048, op))
	}
}

func checkNormParam(op string, p *Tensor, D int) {
	if p != nil && (len(p.shape) != 1 || p.shape[0] != D) {
		panic(messages.New(messages.H3049, op, D, p.shape))
	}
}

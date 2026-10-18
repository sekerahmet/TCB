package tensor

import "fmt"

// Sinir ağı katmanları için birleşik (tek düğümlü) işlemler; cihazdan bağımsız
// katman. Hesap arka uçta yapılır.

// Embedding w (V×D) tablosundan idx satırlarını seçer. shape verilirse çıktı
// şekli shape + [D] olur (ör. (B, T) indeksler için (B, T, D)); verilmezse
// (len(idx), D).
func Embedding(w *Tensor, idx []int, shape ...int) *Tensor {
	if len(w.shape) != 2 {
		panic(fmt.Sprintf("tensor: Embedding 2 boyutlu tablo ister, şekil %v", w.shape))
	}
	V, D := w.shape[0], w.shape[1]
	if len(shape) == 0 {
		shape = []int{len(idx)}
	}
	if numelOf(shape) != len(idx) {
		panic(fmt.Sprintf("tensor: Embedding: %d indeks %v şekline uymuyor", len(idx), shape))
	}
	for _, ix := range idx {
		if ix < 0 || ix >= V {
			panic(fmt.Sprintf("tensor: Embedding indeksi %d, [0, %d) dışında", ix, V))
		}
	}
	ix := append([]int(nil), idx...)
	out := emptyOf(w.be, append(copyInts(shape), D), w.tur)
	w.be.Embedding(out.storage, w.view(), ix, D)
	return track(out, "embedding", []*Tensor{w}, func() {
		w.be.EmbeddingGrad(w.accGrad(), out.grad, ix, D)
	})
}

// CrossEntropy logits (..., C) ve hedef sınıf indeksleri (satır başına bir
// tane) için ortalama çapraz entropi kaybını verir. Negatif hedefler yok
// sayılır (dolgu için) ve ortalamaya katılmaz. Log-softmax ile birleşik ve
// sayısal kararlıdır; sonuç skalerdir.
func CrossEntropy(logits *Tensor, targets []int) *Tensor {
	if len(logits.shape) < 1 {
		panic("tensor: CrossEntropy en az 1 boyutlu logit ister")
	}
	C := logits.shape[len(logits.shape)-1]
	R := logits.Numel() / max(C, 1)
	if len(targets) != R {
		panic(fmt.Sprintf("tensor: CrossEntropy: %d satır için %d hedef verildi", R, len(targets)))
	}
	gecerli := 0
	for _, t := range targets {
		if t >= C {
			panic(fmt.Sprintf("tensor: CrossEntropy hedefi %d, [0, %d) dışında", t, C))
		}
		if t >= 0 {
			gecerli++
		}
	}
	if gecerli == 0 {
		// Ortalama boş küme üzerinden alınırdı: tanımsız; sessizce 0 dönmesin.
		panic(fmt.Sprintf("tensor: CrossEntropy: geçerli hedef yok (%d satırın hepsi yok sayıldı); ortalama kayıp tanımsız", R))
	}
	tg := append([]int(nil), targets...)
	be := logits.be
	loss, lse := be.CrossEntropy(logits.view(), tg, C)
	out := fromStorage(be, loss, nil)
	return track(out, "cross_entropy", []*Tensor{logits}, func() {
		be.CrossEntropyGrad(logits.accGrad(), out.grad, logits.view(), lse, tg, C)
	}, logits)
}

func optView(t *Tensor) *View {
	if t == nil {
		return nil
	}
	v := t.view()
	return &v
}

// optGrad gradyan istiyorsa tensörün gradyan depolamasını, aksi halde nil verir.
func optGrad(t *Tensor) Storage {
	if t == nil || !t.requiresGrad {
		return nil
	}
	return t.accGrad()
}

// LayerNorm son eksen boyunca katman normalleştirmesi: (x-μ)/√(σ²+eps)·γ + β.
// gamma ve beta (D) şeklinde olmalı; nil verilebilirler.
func LayerNorm(x, gamma, beta *Tensor, eps float32) *Tensor {
	normBoyutDenetle("LayerNorm", x)
	D := x.shape[len(x.shape)-1]
	checkNormParam("LayerNorm", gamma, D)
	checkNormParam("LayerNorm", beta, D)
	be := sameDevice("LayerNorm", x, gamma, beta)
	out := emptyOf(be, x.shape, x.tur)
	gv := optView(gamma)
	mean, rstd := be.LayerNorm(out.storage, x.view(), gv, optView(beta), D, eps)
	parents := []*Tensor{x}
	if gamma != nil {
		parents = append(parents, gamma)
	}
	if beta != nil {
		parents = append(parents, beta)
	}
	return track(out, "layer_norm", parents, func() {
		be.LayerNormGrad(optGrad(x), optGrad(gamma), optGrad(beta), out.grad, x.view(), gv, mean, rstd, D)
	}, x, gamma)
}

// RMSNorm son eksen boyunca karekök-ortalama-kare normalleştirmesi:
// x / √(ort(x²)+eps) · γ. gamma (D) şeklinde olmalı; nil verilebilir.
func RMSNorm(x, gamma *Tensor, eps float32) *Tensor {
	normBoyutDenetle("RMSNorm", x)
	D := x.shape[len(x.shape)-1]
	checkNormParam("RMSNorm", gamma, D)
	be := sameDevice("RMSNorm", x, gamma)
	out := emptyOf(be, x.shape, x.tur)
	gv := optView(gamma)
	rinv := be.RMSNorm(out.storage, x.view(), gv, D, eps)
	parents := []*Tensor{x}
	if gamma != nil {
		parents = append(parents, gamma)
	}
	return track(out, "rms_norm", parents, func() {
		be.RMSNormGrad(optGrad(x), optGrad(gamma), out.grad, x.view(), gv, rinv, D)
	}, x, gamma)
}

// normBoyutDenetle normalleştirilecek girdinin en az 1 boyutlu olduğunu denetler.
func normBoyutDenetle(op string, x *Tensor) {
	if len(x.shape) == 0 {
		panic(fmt.Sprintf("tensor: %s en az 1 boyutlu girdi ister (son eksen normalleştirilir), 0 boyutlu tensör verildi", op))
	}
}

func checkNormParam(op string, p *Tensor, D int) {
	if p != nil && (len(p.shape) != 1 || p.shape[0] != D) {
		panic(fmt.Sprintf("tensor: %s parametresi (%d) şeklinde olmalı, %v verildi", op, D, p.shape))
	}
}

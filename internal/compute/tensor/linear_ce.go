package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// LinearCrossEntropyChunk, LinearCrossEntropy'nin bir dilimde hesapladığı satır sayısıdır:
// dilimin logitleri [8192, C] kadardır (hedef modelin PyTorch sürümü lm_loss'ta aynı boyu
// kullanır).
const LinearCrossEntropyChunk = 8192

// LinearCrossEntropy CrossEntropy(MatMul(x, w), targets) ile aynı kaybı ve gradyanları verir,
// ama bütün logitleri ([satır, C]) hiç tutmaz: satırlar LinearCrossEntropyChunk'lık dilimlerle
// işlenir; her dilimin logitleri hesaplanır, kaybı toplanır ve (türev izleniyorsa) gradyanı
// hemen x'e ve w'ye geri götürülüp logitler bırakılır. Bellek bir dilimin logitleri ile
// gradyanı kadardır; ek hesap yoktur (logitler geri yayılımda yeniden hesaplanmaz).
//
// Karşılaştırma: PyTorch'ta hazır işlem yoktur; hedef modelin kendi kodu dilimleri
// checkpoint ile geri yayılımda yeniden hesaplar (bir ileri çarpım fazla). Liger Kernel'in
// FusedLinearCrossEntropy'si gradyanı ileri hesapta dilim dilim çıkarır (buradaki yöntem);
// Apple'ın Cut Cross-Entropy'si logitleri hiç yazmayan özel çekirdekler kullanır. Bu yöntem
// var olan çarpım ve çapraz entropi çekirdekleriyle yazıldığından her arka uçta çalışır.
//
// x [..., D], w [D, C], targets satır başına bir sınıf (negatif: yok sayılır); kayıp geçerli
// hedefler üzerinden ortalamadır (CrossEntropy ile aynı kural). Logitler x ile w'nin ortak
// türündedir (karışık hassasiyette çağıran ikisini blok türüne çevirir). Geri yayılımda
// saklanan gradyanlar gelen gradyanla (kaybın ölçeği, ör. birikimde 1/n) çarpılır.
func LinearCrossEntropy(x, w *Tensor, targets []int) *Tensor {
	return LinearCrossEntropyChunked(x, w, targets, LinearCrossEntropyChunk)
}

// LinearCrossEntropyChunked LinearCrossEntropy'dir; dilim boyu (satır) açıkça verilir.
// Sonuç dilim boyundan bağımsızdır (yuvarlama sırası dışında); küçük dilim, birden çok dilimi
// az satırla sınamak içindir.
func LinearCrossEntropyChunked(x, w *Tensor, targets []int, chunk int) *Tensor {
	if chunk < 1 {
		panic(messages.Internal("LinearCrossEntropyChunked: chunk must be positive"))
	}
	if len(x.shape) < 1 || len(w.shape) != 2 || x.shape[len(x.shape)-1] != w.shape[0] {
		panic(messages.New(messages.H3106, x.shape, w.shape))
	}
	be := sameDevice("LinearCrossEntropy", x, w)
	D, C := w.shape[0], w.shape[1]
	R := x.Numel() / max(D, 1)
	valid := validTargets(targets, R, C)
	sy := counterStart(be, OpKindCrossEntropy).start()
	defer sy.finish()
	dt := resultDType(x.dtyp, w.dtyp)
	xv, wv := x.contigView(), w.contigView()
	rx, rw := copyRelease(x, xv), copyRelease(w, wv)
	defer rx()
	defer rw()
	wantX := GradEnabled() && x.RequiresGrad()
	wantW := GradEnabled() && w.RequiresGrad()

	loss := be.Alloc(1, Float32)
	var gx, gw Storage // dilim dilim çıkarılan gradyanlar (geri yayılımda ölçeklenir)
	if wantX {
		gx = allocUninit(be, R*D, x.n.gradDType())
	}
	if wantW {
		gw = be.Alloc(D*C, Float32)
	}
	gs, writer := be.(GemmSetter)
	for r0 := 0; r0 < R; r0 += chunk {
		n := min(chunk, R-r0)
		tg := targets[r0 : r0+n]
		vc := 0
		for _, t := range tg {
			if t >= 0 {
				vc++
			}
		}
		if vc == 0 {
			zeroRows(be, gx, r0, n, D)
			continue
		}
		xc := View{S: xv.S, Offset: xv.Offset + r0*D, Shape: []int{n, D}, Strides: []int{D, 1}}
		logits := allocUninit(be, n*C, dt)
		if writer {
			gs.GemmSet(1, n, C, D, xc, false, wv, false, logits)
		} else {
			be.Fill(logits, 0)
			be.Gemm(1, n, C, D, xc, false, wv, false, logits)
		}
		lv := DenseView(logits, []int{n, C})
		lc, lse := be.CrossEntropy(lv, tg, C)
		share := float32(vc) / float32(valid)
		be.AddTo(loss, lc, share) // dilimin ortalaması × geçerli payı
		giveBack(be, lc)
		if wantX || wantW {
			linearCEChunkGrad(be, lv, lse, tg, share, xc, wv, gx, gw, r0, D, C, writer)
		}
		giveBack(be, lse)
		giveBack(be, logits)
	}
	out := fromStorage(be, loss, nil)
	r := track(out, "linear_cross_entropy", []*Tensor{x, w}, func(o *node) {
		if xn := o.parents[0]; xn.wants() {
			scaledAdd(be, xn, gx, o.grad)
		}
		if wn := o.parents[1]; wn.wants() {
			scaledAdd(be, wn, gw, o.grad)
		}
		releaseAll(be, gx, gw)
	})
	if r.IsLeaf() {
		releaseAll(be, gx, gw)
	}
	return r
}

// validTargets R satırın hedeflerini C sınıfa göre denetler (CrossEntropy'nin kuralları) ve
// geçerli (negatif olmayan) hedef sayısını verir; hiçbiri geçerli değilse panikler.
func validTargets(targets []int, R, C int) int {
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
		panic(messages.New(messages.H3047, R))
	}
	return valid
}

// linearCEChunkGrad bir dilimin logit gradyanını (kaybın geçerli hedefler üzerinden
// ortalamasına göre: share = dilimin geçerli payı) çıkarır ve x'in (gx'in r0. satırından)
// ve w'nin (gw'ye eklenerek) gradyanına götürür.
func linearCEChunkGrad(be Backend, lv View, lse Storage, tg []int, share float32, xc, wv View, gx, gw Storage, r0, D, C int, writer bool) {
	n := len(tg)
	seed := scalarStorage(be, share)
	dl := allocUninit(be, n*C, lv.DType())
	if cs, ok := be.(CrossEntropyGradSetter); ok {
		cs.CrossEntropyGradSet(dl, seed, lv, lse, tg, C)
	} else {
		be.Fill(dl, 0)
		be.CrossEntropyGrad(dl, seed, lv, lse, tg, C)
	}
	giveBack(be, seed)
	dv := DenseView(dl, []int{n, C})
	if gx != nil {
		// dX (n×D) = dL (n×C) · Wᵀ
		part := allocUninit(be, n*D, gx.DType())
		if gs, ok := be.(GemmSetter); ok && writer {
			gs.GemmSet(1, n, D, C, dv, false, wv, true, part)
		} else {
			be.Fill(part, 0)
			be.Gemm(1, n, D, C, dv, false, wv, true, part)
		}
		be.Copy(gx, r0*D, DenseView(part, []int{n, D}))
		giveBack(be, part)
	}
	if gw != nil {
		// dW (D×C) += Xᵀ (D×n) · dL (n×C)
		be.Gemm(1, D, C, n, xc, true, dv, false, gw)
	}
	giveBack(be, dl)
}

// releaseAll nil olmayan depoları arka uca iade eder.
func releaseAll(be Backend, ss ...Storage) {
	for _, s := range ss {
		if s != nil {
			giveBack(be, s)
		}
	}
}

// scaledAdd n'nin gradyanına g · s ekler (s tek elemanlı gelen gradyan; kaybın ölçeği).
func scaledAdd(be Backend, n *node, s, g Storage) {
	shape := n.shape
	gv := View{S: g, Shape: []int{}, Strides: []int{}}
	if c, fresh := n.freshGrad(true); fresh {
		be.Binary(OpMul, c, shape, DenseView(s, shape), gv)
		return
	}
	tmp := allocUninit(be, numelOf(shape), n.gradDType())
	be.Binary(OpMul, tmp, shape, DenseView(s, shape), gv)
	be.AddTo(n.accGrad(), tmp, 1)
	giveBack(be, tmp)
}

// zeroRows tamamen yok sayılan dilimin x gradyanı satırlarını ([r0, r0+n) × D) sıfırlar: gx
// sıfırsız ayrıldığından yalnızca bu satırlar yazılır (bütün gx'i sıfırlı ayırmaktan ucuz).
func zeroRows(be Backend, gx Storage, r0, n, D int) {
	if gx == nil {
		return
	}
	z := be.Alloc(n*D, gx.DType())
	be.Copy(gx, r0*D, DenseView(z, []int{n, D}))
	giveBack(be, z)
}

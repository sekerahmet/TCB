package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İndirgemeler ve softmax (cihazdan bağımsız katman).

// reducedShape indirgenmiş çıktı şeklini verir.
func reducedShape(shape []int, axis int, keepdim bool) []int {
	if keepdim {
		sh := copyInts(shape)
		sh[axis] = 1
		return sh
	}
	sh := make([]int, 0, len(shape)-1)
	sh = append(sh, shape[:axis]...)
	return append(sh, shape[axis+1:]...)
}

// Sum verilen eksen boyunca toplar. keepdim doğruysa eksen 1 uzunlukla kalır.
func (t *Tensor) Sum(axis int, keepdim bool) *Tensor {
	return t.sumScaled(axis, keepdim, false)
}

// Mean verilen eksen boyunca ortalama alır.
func (t *Tensor) Mean(axis int, keepdim bool) *Tensor {
	return t.sumScaled(axis, keepdim, true)
}

func (t *Tensor) sumScaled(axis int, keepdim, mean bool) *Tensor {
	sy := counterStart(t.be, reduceOp(mean)).start()
	axis = normAxis(axis, len(t.shape))
	scale := float32(1)
	op := "sum"
	if mean {
		op = "mean"
		n := t.shape[axis]
		if n == 0 {
			// Boş kümenin ortalaması tanımsızdır; sessizce 0 dönmesin (çıktı da boşsa
			// hesaplanacak eleman yoktur).
			if NumelOf(reducedShape(t.shape, axis, false)) > 0 {
				panic(messages.New(messages.H3063, t.shape, axis))
			}
		} else {
			scale = 1 / float32(n)
		}
	}
	out := zerosOf(isReduce, t.be, reducedShape(t.shape, axis, keepdim), t.dtyp)
	t.be.Sum(out.storage, t.view(), axis, scale)
	kd := reducedShape(t.shape, axis, true)
	r := track(out, op, []*Tensor{t}, func(o *node) {
		in := o.parents[0]
		in.be.ExpandAdd(in.accGrad(), in.shape, o.grad, kd, scale)
	})
	sy.finish()
	return r
}

// SumAll tüm elemanları toplar ve skaler (0 boyutlu) tensör verir.
func (t *Tensor) SumAll() *Tensor { return t.allScaled(false) }

// MeanAll tüm elemanların ortalamasını skaler tensör olarak verir.
func (t *Tensor) MeanAll() *Tensor { return t.allScaled(true) }

func (t *Tensor) allScaled(mean bool) *Tensor {
	sy := counterStart(t.be, reduceOp(mean)).start()
	scale := float32(1)
	op := "sum_all"
	if mean {
		op = "mean_all"
		n := t.Numel()
		if n == 0 {
			panic(messages.New(messages.H3064, t.shape))
		}
		scale = 1 / float32(n)
	}
	out := zerosOf(isReduce, t.be, nil, t.dtyp)
	t.be.SumAll(out.storage, t.view(), scale)
	r := track(out, op, []*Tensor{t}, func(o *node) {
		in := o.parents[0]
		in.be.ExpandAdd(in.accGrad(), in.shape, o.grad, nil, scale)
	})
	sy.finish()
	return r
}

// Max verilen eksen boyunca en büyük değeri alır. Gradyan, eşitlik halinde
// ilk en büyük elemana gider.
func (t *Tensor) Max(axis int, keepdim bool) *Tensor {
	sy := counterStart(t.be, OpKindMax).start()
	axis = normAxis(axis, len(t.shape))
	if t.shape[axis] == 0 {
		panic(messages.New(messages.H3065))
	}
	out := zerosOf(isReduce, t.be, reducedShape(t.shape, axis, keepdim), t.dtyp)
	t.be.Max(out.storage, t.view(), axis)
	tv := t.view()
	r := track(out, "max", []*Tensor{t}, func(o *node) {
		in := o.parents[0]
		in.be.MaxGrad(in.accGrad(), o.grad, tv, axis)
	}, t)
	sy.finish()
	return r
}

// ArgMax verilen eksen boyunca en büyük elemanın indeksini verir (türevsiz).
// Sonuç, eksen çıkarılmış şeklin bitişik düzenindedir.
func (t *Tensor) ArgMax(axis int) []int {
	sy := counterStart(t.be, OpKindArgMax).start()
	idx := t.be.ArgMax(t.view(), normAxis(axis, len(t.shape)))
	sy.finish()
	return idx
}

// Softmax verilen eksen boyunca sayısal kararlı softmax (en büyük değer
// çıkarılarak) hesaplar.
func (t *Tensor) Softmax(axis int) *Tensor { return t.softmaxImpl(axis, false) }

// LogSoftmax verilen eksen boyunca sayısal kararlı log-softmax hesaplar.
func (t *Tensor) LogSoftmax(axis int) *Tensor { return t.softmaxImpl(axis, true) }

func (t *Tensor) softmaxImpl(axis int, logMode bool) *Tensor {
	is := OpKindSoftmax
	if logMode {
		is = OpKindLogSoftmax
	}
	sy := counterStart(t.be, is).start()
	axis = normAxis(axis, len(t.shape))
	out := emptyOf(isSoftmaxOp, t.be, t.shape, t.dtyp)
	t.be.Softmax(out.storage, t.view(), axis, logMode)
	op := "softmax"
	if logMode {
		op = "log_softmax"
	}
	os := out.storage
	r := track(out, op, []*Tensor{t}, func(o *node) {
		in := o.parents[0]
		in.be.SoftmaxGrad(in.accGrad(), o.grad, os, in.shape, axis, logMode)
	}, out)
	sy.finish()
	return r
}

// reduceOp toplamın ya da ortalamanın sayaç işlem türüdür.
func reduceOp(meanVal bool) OpKind {
	if meanVal {
		return OpKindMean
	}
	return OpKindSum
}

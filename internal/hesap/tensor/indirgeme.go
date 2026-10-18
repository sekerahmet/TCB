package tensor

import "fmt"

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
	axis = normAxis(axis, len(t.shape))
	scale := float32(1)
	op := "sum"
	if mean {
		op = "mean"
		n := t.shape[axis]
		if n == 0 {
			// Boş kümenin ortalaması tanımsızdır; sessizce 0 dönmesin (çıktı da boşsa
			// hesaplanacak eleman yoktur).
			if numelOf(reducedShape(t.shape, axis, false)) > 0 {
				panic(fmt.Sprintf("tensor: %v şeklinde %d. eksen boş; boş kümenin ortalaması tanımsız", t.shape, axis))
			}
		} else {
			scale = 1 / float32(n)
		}
	}
	out := zerosOf(isIndirgeme, t.be, reducedShape(t.shape, axis, keepdim), t.tur)
	t.be.Sum(out.storage, t.view(), axis, scale)
	kd := reducedShape(t.shape, axis, true)
	return track(out, op, []*Tensor{t}, func() {
		t.be.ExpandAdd(t.accGrad(), t.shape, out.grad, kd, scale)
	})
}

// SumAll tüm elemanları toplar ve skaler (0 boyutlu) tensör verir.
func (t *Tensor) SumAll() *Tensor { return t.allScaled(false) }

// MeanAll tüm elemanların ortalamasını skaler tensör olarak verir.
func (t *Tensor) MeanAll() *Tensor { return t.allScaled(true) }

func (t *Tensor) allScaled(mean bool) *Tensor {
	scale := float32(1)
	op := "sum_all"
	if mean {
		op = "mean_all"
		n := t.Numel()
		if n == 0 {
			panic(fmt.Sprintf("tensor: boş tensörün ortalaması tanımsız (şekil %v)", t.shape))
		}
		scale = 1 / float32(n)
	}
	out := zerosOf(isIndirgeme, t.be, nil, t.tur)
	t.be.SumAll(out.storage, t.view(), scale)
	return track(out, op, []*Tensor{t}, func() {
		t.be.ExpandAdd(t.accGrad(), t.shape, out.grad, nil, scale)
	})
}

// Max verilen eksen boyunca en büyük değeri alır. Gradyan, eşitlik halinde
// ilk en büyük elemana gider.
func (t *Tensor) Max(axis int, keepdim bool) *Tensor {
	axis = normAxis(axis, len(t.shape))
	if t.shape[axis] == 0 {
		panic("tensor: boş eksende Max alınamaz")
	}
	out := zerosOf(isIndirgeme, t.be, reducedShape(t.shape, axis, keepdim), t.tur)
	t.be.Max(out.storage, t.view(), axis)
	return track(out, "max", []*Tensor{t}, func() {
		t.be.MaxGrad(t.accGrad(), out.grad, t.view(), axis)
	}, t)
}

// ArgMax verilen eksen boyunca en büyük elemanın indeksini verir (türevsiz).
// Sonuç, eksen çıkarılmış şeklin bitişik düzenindedir.
func (t *Tensor) ArgMax(axis int) []int {
	return t.be.ArgMax(t.view(), normAxis(axis, len(t.shape)))
}

// Softmax verilen eksen boyunca sayısal kararlı softmax (en büyük değer
// çıkarılarak) hesaplar.
func (t *Tensor) Softmax(axis int) *Tensor { return t.softmaxImpl(axis, false) }

// LogSoftmax verilen eksen boyunca sayısal kararlı log-softmax hesaplar.
func (t *Tensor) LogSoftmax(axis int) *Tensor { return t.softmaxImpl(axis, true) }

func (t *Tensor) softmaxImpl(axis int, logKip bool) *Tensor {
	axis = normAxis(axis, len(t.shape))
	out := emptyOf(isSoftmax, t.be, t.shape, t.tur)
	t.be.Softmax(out.storage, t.view(), axis, logKip)
	op := "softmax"
	if logKip {
		op = "log_softmax"
	}
	return track(out, op, []*Tensor{t}, func() {
		t.be.SoftmaxGrad(t.accGrad(), out.grad, out.storage, t.shape, axis, logKip)
	}, out)
}

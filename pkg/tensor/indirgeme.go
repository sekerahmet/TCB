package tensor

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
		if n := t.shape[axis]; n > 0 {
			scale = 1 / float32(n)
		}
	}
	out := zerosOn(t.be, reducedShape(t.shape, axis, keepdim))
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
		if n := t.Numel(); n > 0 {
			scale = 1 / float32(n)
		}
	}
	out := zerosOn(t.be, nil)
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
	out := zerosOn(t.be, reducedShape(t.shape, axis, keepdim))
	t.be.Max(out.storage, t.view(), axis)
	return track(out, "max", []*Tensor{t}, func() {
		t.be.MaxGrad(t.accGrad(), out.grad, t.view(), axis)
	})
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
	out := zerosOn(t.be, t.shape)
	t.be.Softmax(out.storage, t.view(), axis, logKip)
	op := "softmax"
	if logKip {
		op = "log_softmax"
	}
	return track(out, op, []*Tensor{t}, func() {
		t.be.SoftmaxGrad(t.accGrad(), out.grad, out.storage, t.shape, axis, logKip)
	})
}

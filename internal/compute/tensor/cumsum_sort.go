package tensor

import "fmt"

// Eksen boyunca birikimli toplam ve sıralama (cihazdan bağımsız katman).

// EnCokSiralamaBoyu sıralanan bir eksenin en çok uzunluğudur: indeksler float32 tensörde
// tutulur ve 2^24'e kadar tam gösterilir.
const MaxSortLen = 1 << 24

// CumSum verilen eksen boyunca birikimli toplam (her konumda o konuma kadar olanların
// toplamı); şekil korunur. Türevi gradyanın sondan birikimli toplamıdır. Toplama sırası
// sabittir (belirlenimci).
func (t *Tensor) CumSum(axis int) *Tensor {
	axis = normAxis(axis, len(t.shape))
	sy := counterStart(t.be, OpKindCumSum).start()
	out := emptyOf(isReduce, t.be, t.shape, t.dtyp)
	t.be.CumSum(out.storage, t.view(), axis)
	r := track(out, "cumsum", []*Tensor{t}, func() {
		t.be.CumSumGrad(t.accGrad(), out.grad, t.shape, axis)
	})
	sy.finish()
	return r
}

// siralamaDenetle eksenin sıralanabilirliğini ve k'yı denetler.
func (t *Tensor) checkSort(ad string, axis, k int) {
	L := t.shape[axis]
	if L > MaxSortLen {
		panic(fmt.Sprintf("tensor: %s: %v şeklinde %d. eksen %d uzunlukta; en çok %d (2^24) uzunluklu eksen sıralanır",
			ad, t.shape, axis, L, MaxSortLen))
	}
	if k < 0 || k > L {
		panic(fmt.Sprintf("tensor: %s: k = %d, %v şeklinde %d. eksenin uzunluğu %d; k 0 ile %d arasında olmalı", ad, k, t.shape, axis, L, L))
	}
}

// TopK verilen eksende en büyük k elemanı büyükten küçüğe verir: değerler (türevli;
// gradyan seçilen konumlara gider) ve eksen indeksleri (float32 tam sayı, türevsiz).
// Eşit değerlerde küçük indeks önce gelir; NaN her sayıdan büyük sayılır. Sonuçların
// şekli girdinin şeklidir, yalnızca eksenin uzunluğu k'dır.
func (t *Tensor) TopK(k, axis int) (vals, idx *Tensor) {
	axis = normAxis(axis, len(t.shape))
	t.checkSort("TopK", axis, k)
	sh := copyInts(t.shape)
	sh[axis] = k
	sy := counterStart(t.be, OpKindTopK).start()
	vals = emptyOf(isReduce, t.be, sh, t.dtyp)
	idx = emptyOf(isReduce, t.be, sh, Float32)
	t.be.Sort(vals.storage, idx.storage, t.view(), axis, k, true)
	vals = track(vals, "topk", []*Tensor{t}, func() {
		t.be.GatherGrad(t.accGrad(), vals.grad, idx.storage, t.shape, axis, k)
	}, idx)
	sy.finish()
	return vals, idx
}

// ArgSort verilen eksende küçükten büyüğe kararlı sıralamanın indekslerini (float32 tam
// sayı, türevsiz) verir: eşitlerde küçük indeks önce gelir, NaN'lar sondadır.
func (t *Tensor) ArgSort(axis int) *Tensor {
	axis = normAxis(axis, len(t.shape))
	t.checkSort("ArgSort", axis, t.shape[axis])
	sy := counterStart(t.be, OpKindArgSort).start()
	idx := emptyOf(isReduce, t.be, t.shape, Float32)
	t.be.Sort(nil, idx.storage, t.view(), axis, t.shape[axis], false)
	sy.finish()
	return idx
}

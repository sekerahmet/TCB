package tensor

import (
	"fmt"
	"math"
)

// Concat tensörleri verilen eksen boyunca birleştirir (negatif eksen sondan
// sayar). Bütün girdiler aynı cihazda ve aynı eksen sayısında olmalı, birleşen
// eksen dışındaki eksenleri eşit olmalıdır. Sonuç türü girdilerin türleri
// aynıysa o tür, değilse float32'dir. Türevlenebilir: her girdinin gradyanı
// çıktının gradyanındaki kendi diliminden kopyalanır.
//
// Hesap arka ucun ScatterAdd ve Copy işlemleriyle yapılır: her girdi sıfır
// çıktının kendi ofsetine adımlarıyla bir kez eklenir (girdi başına tek arka
// uç çağrısı).
func Concat(ts []*Tensor, axis int) *Tensor {
	if len(ts) == 0 {
		panic("tensor: Concat en az bir tensör ister")
	}
	be := sameDevice("concat", ts...)
	sy := counterStart(be, OpKindConcat).start()
	first := ts[0]
	nd := len(first.shape)
	if nd == 0 {
		panic("tensor: Concat 0 boyutlu tensörleri birleştiremez")
	}
	axis = normAxis(axis, nd)
	dt := first.dtyp
	total := 0
	for i, t := range ts {
		if len(t.shape) != nd {
			panic(fmt.Sprintf("tensor: Concat: %d. girdi %d boyutlu (%v), ilki %d boyutlu (%v)", i, len(t.shape), t.shape, nd, first.shape))
		}
		for d := 0; d < nd; d++ {
			if d != axis && t.shape[d] != first.shape[d] {
				panic(fmt.Sprintf("tensor: Concat: %d. girdinin şekli %v, ilkininki %v; %d. eksen dışındaki eksenler eşit olmalı", i, t.shape, first.shape, axis))
			}
		}
		if total > math.MaxInt-t.shape[axis] {
			// Verisiz (sıfır eksenli) girdilerin dev eksenleri toplanırken taşma sessizce
			// küçük bir şekle sarmasın.
			panic(fmt.Sprintf("tensor: birleştirme: %d. eksenin toplam uzunluğu tam sayı sınırını aşıyor (%d. girdi %v); "+
				"şekilleri denetleyin", axis, i, t.shape))
		}
		total += t.shape[axis]
		dt = resultDType(dt, t.dtyp)
	}
	outShape := copyInts(first.shape)
	outShape[axis] = total
	ost := contiguousStrides(outShape)

	// Girdiler float32 bir tampona eklenir; 16 bitlik çıktıda tek kez yuvarlanır. Tampon -0
	// ile başlar: -0 + x her x için (-0 dahil) x'tir, girdilerin işaretli sıfırları korunur.
	buffer := allocUninit(be, allocSizeOn(be, isConcat, outShape, Float32), Float32)
	be.Fill(buffer, float32(math.Copysign(0, -1)))
	offset := 0
	for _, t := range ts {
		if t.Numel() > 0 {
			be.ScatterAdd(buffer, offset*ost[axis], ost, t.denseStorage(), t.shape)
		}
		offset += t.shape[axis]
	}
	var out *Tensor
	if dt == Float32 {
		out = fromStorage(be, buffer, outShape)
	} else {
		out = emptyOf(isConcat, be, outShape, dt)
		be.Copy(out.storage, 0, View{S: buffer, Shape: outShape, Strides: ost})
		giveBack(be, buffer)
	}
	r := track(out, "concat", ts, func() {
		offset := 0
		for _, t := range ts {
			begin := offset * ost[axis]
			offset += t.shape[axis]
			if !t.requiresGrad || t.Numel() == 0 {
				continue
			}
			g := View{S: out.grad, Offset: begin, Shape: t.shape, Strides: ost}
			if t.grad == nil {
				s := allocUninit(be, t.Numel(), gradDTypeFor(t.dtyp))
				be.Copy(s, 0, g)
				t.grad = s
				continue
			}
			s := allocUninit(be, t.Numel(), out.grad.DType())
			be.Copy(s, 0, g)
			be.AddTo(t.grad, s, 1)
			giveBack(be, s)
		}
	})
	sy.finish()
	return r
}

// Split tensörü verilen eksen boyunca n eşit parçaya böler; parçalar veriyi
// kopyalamayan, türev izleyen görünümlerdir (Slice). Eksenin uzunluğu n'nin
// katı olmalıdır.
func (t *Tensor) Split(n, axis int) []*Tensor {
	axis = normAxis(axis, len(t.shape))
	if n <= 0 || t.shape[axis]%n != 0 {
		panic(fmt.Sprintf("tensor: Split: %d. eksen (%d) %d eşit parçaya bölünemez", axis, t.shape[axis], n))
	}
	m := t.shape[axis] / n
	out := make([]*Tensor, n)
	for i := range out {
		out[i] = t.Slice(axis, i*m, (i+1)*m)
	}
	return out
}

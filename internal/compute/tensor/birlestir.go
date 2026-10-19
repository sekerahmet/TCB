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
	sy := sayacBasla(be, IslemBirlestirme).baslat()
	ilk := ts[0]
	nd := len(ilk.shape)
	if nd == 0 {
		panic("tensor: Concat 0 boyutlu tensörleri birleştiremez")
	}
	axis = normAxis(axis, nd)
	dt := ilk.tur
	toplam := 0
	for i, t := range ts {
		if len(t.shape) != nd {
			panic(fmt.Sprintf("tensor: Concat: %d. girdi %d boyutlu (%v), ilki %d boyutlu (%v)", i, len(t.shape), t.shape, nd, ilk.shape))
		}
		for d := 0; d < nd; d++ {
			if d != axis && t.shape[d] != ilk.shape[d] {
				panic(fmt.Sprintf("tensor: Concat: %d. girdinin şekli %v, ilkininki %v; %d. eksen dışındaki eksenler eşit olmalı", i, t.shape, ilk.shape, axis))
			}
		}
		if toplam > math.MaxInt-t.shape[axis] {
			// Verisiz (sıfır eksenli) girdilerin dev eksenleri toplanırken taşma sessizce
			// küçük bir şekle sarmasın.
			panic(fmt.Sprintf("tensor: birleştirme: %d. eksenin toplam uzunluğu tam sayı sınırını aşıyor (%d. girdi %v); "+
				"şekilleri denetleyin", axis, i, t.shape))
		}
		toplam += t.shape[axis]
		dt = sonucTuru(dt, t.tur)
	}
	outShape := copyInts(ilk.shape)
	outShape[axis] = toplam
	ost := contiguousStrides(outShape)

	// Girdiler float32 bir tampona eklenir; 16 bitlik çıktıda tek kez yuvarlanır.
	tampon := be.Alloc(ayirmaBoyuOn(be, isBirlestirme, outShape, Float32), Float32)
	ofset := 0
	for _, t := range ts {
		if t.Numel() > 0 {
			be.ScatterAdd(tampon, ofset*ost[axis], ost, t.denseStorage(), t.shape)
		}
		ofset += t.shape[axis]
	}
	var out *Tensor
	if dt == Float32 {
		out = fromStorage(be, tampon, outShape)
	} else {
		out = emptyOf(isBirlestirme, be, outShape, dt)
		be.Copy(out.storage, 0, View{S: tampon, Shape: outShape, Strides: ost})
	}
	r := track(out, "concat", ts, func() {
		ofset := 0
		for _, t := range ts {
			bas := ofset * ost[axis]
			ofset += t.shape[axis]
			if !t.requiresGrad || t.Numel() == 0 {
				continue
			}
			g := View{S: out.grad, Offset: bas, Shape: t.shape, Strides: ost}
			if t.grad == nil {
				s := allocBos(be, t.Numel(), gradTuru(t.tur))
				be.Copy(s, 0, g)
				t.grad = s
				continue
			}
			s := allocBos(be, t.Numel(), out.grad.DType())
			be.Copy(s, 0, g)
			be.AddTo(t.grad, s, 1)
			iadeEt(be, s)
		}
	})
	sy.bitir()
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

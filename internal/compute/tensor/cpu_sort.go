package tensor

import (
	"math"
	"slices"
)

// CPU arka ucu: eksen boyunca birikimli toplam ve sıralama. Tensör bitişik (dış, eksen,
// iç) üçlüsü olarak gezilir (eksenDuzeni); satırlar işçilere bölünür, her satır tek
// görevde sabit sırayla işlenir (belirlenimci).

// f32CumSum out[o, d, i] = Σ_{e ≤ d} x[o, e, i]; birikim float64'tedir. ters doğruysa
// toplam sondan başlar (Σ_{e ≥ d}); ekle doğruysa out'a eklenir.
func f32CumSum(out []float32, xd []float32, shape []int, axis int, inverse, add bool) {
	outer, dim, inner := AxisOrder(shape, axis)
	if outer*dim*inner == 0 {
		return
	}
	parallelFor(outer, dim*inner, func(lo, hi int) {
		acc := make([]float64, inner)
		for o := lo; o < hi; o++ {
			clear(acc)
			for k := 0; k < dim; k++ {
				d := k
				if inverse {
					d = dim - 1 - k
				}
				begin := (o*dim + d) * inner
				row, target := xd[begin:begin+inner], out[begin:begin+inner]
				for i, v := range row {
					acc[i] += float64(v)
					if add {
						target[i] += float32(acc[i])
					} else {
						target[i] = float32(acc[i])
					}
				}
			}
		}
	})
}

func (cpuBackend) f32CumSumGrad(gx, g Storage, shape []int, axis int) {
	f32CumSum(cd(gx), cd(g), shape, axis, true, true)
}

// orderKey float32'yi sıralama sırasını (13.10, 15.22b) koruyan 32 bitlik anahtara çevirir:
// −0.0 0.0'dan küçüktür, NaN (işaretinden bağımsız) en büyüktür, NaN'lar eşittir. azalan
// doğruysa düzen tersine çevrilir (NaN en küçük, yani ilk olur). GPU arka ucu aynı anahtarı
// kullanır.
func orderKey(v float32, descending bool) uint32 {
	var a uint32
	switch b := math.Float32bits(v); {
	case v != v:
		a = math.MaxUint32
	case b>>31 == 1:
		a = ^b
	default:
		a = b | 1<<31
	}
	if descending {
		a = ^a
	}
	return a
}

// f32Sort her (dış, iç) satırını (anahtar, indeks) çiftlerine göre sıralar; anahtarlar
// çiftler arasında tekil olduğu için sıralama kararlıdır (eşitlerde küçük indeks önce).
// Küçük k'da (k ≤ smallTopK ve k < eksen) satır sıralanmaz, selectSmallest seçer; sonuç
// tam sıralamanın ilk k'sıyla bayt bayt aynıdır.
func (cpuBackend) f32Sort(vals, idx Storage, x View, axis, k int, desc bool) {
	xd := gatherCPU(x)
	outer, dim, inner := AxisOrder(x.Shape, axis)
	vd, id := cd(vals), cd(idx)
	if outer*inner == 0 || k == 0 {
		return
	}
	small := k <= smallTopK && k < dim
	cost, held := max(1, dim*bitsLen(dim)), dim
	if small {
		cost, held = dim, k
	}
	parallelFor(outer*inner, cost, func(lo, hi int) {
		double := make([]uint64, held)
		for r := lo; r < hi; r++ {
			o, i := r/inner, r%inner
			begin := o*dim*inner + i
			if small {
				selectSmallest(double, xd, begin, dim, inner, desc)
			} else {
				for d := range double {
					double[d] = uint64(orderKey(xd[begin+d*inner], desc))<<32 | uint64(d)
				}
				slices.Sort(double)
			}
			cb := o*k*inner + i
			for j, c := range double[:k] {
				d := int(uint32(c))
				id[cb+j*inner] = float32(d)
				if vd != nil {
					vd[cb+j*inner] = xd[begin+d*inner]
				}
			}
		}
	})
}

// smallTopK sıralamasız seçimin (selectSmallest) en büyük k'sı; k bundan büyükse ya da
// eksene eşitse satır bütünüyle sıralanır. TR_argsort'un k'sı eksendir, hep sıralar.
// Testler 0'a çekip eski yolu aynı girdide koşar.
var smallTopK = 16

// selectSmallest satırın (begin'den inner adımla dim eleman) en küçük len(list) anahtarını
// artan sırayla list'e yazar. Anahtar f32Sort'unkidir: orderKey << 32 | indeks. Satır bir
// kez gezilir; liste son elemanından küçük anahtar gelince sokma ile güncellenir.
// Anahtarlar satırda tekil olduğundan sonuç gezme sırasına bağlı değildir ve tam
// sıralamanın ilk len(list) anahtarıdır.
func selectSmallest(list []uint64, xd []float32, begin, dim, inner int, desc bool) {
	n := len(list)
	for d := range list {
		list[d] = uint64(orderKey(xd[begin+d*inner], desc))<<32 | uint64(d)
	}
	slices.Sort(list)
	last := list[n-1]
	for d := n; d < dim; d++ {
		c := uint64(orderKey(xd[begin+d*inner], desc))<<32 | uint64(d)
		if c >= last {
			continue
		}
		j := n - 1
		for j > 0 && list[j-1] > c {
			list[j] = list[j-1]
			j--
		}
		list[j] = c
		last = list[n-1]
	}
}

// bitsLen n'nin ikili uzunluğu (sıralama maliyeti tahmini için).
func bitsLen(n int) int {
	b := 1
	for n > 1 {
		n >>= 1
		b++
	}
	return b
}

// f32GatherGrad gx[o, idx[o, j, i], i] += g[o, j, i]. Bir satırdaki indeksler tekildir.
func (cpuBackend) f32GatherGrad(gx, g, idx Storage, shape []int, axis, k int) {
	pg, gd, id := cd(gx), cd(g), cd(idx)
	outer, dim, inner := AxisOrder(shape, axis)
	parallelFor(outer, k*inner, func(lo, hi int) {
		for o := lo; o < hi; o++ {
			for j := 0; j < k; j++ {
				kb := (o*k + j) * inner
				for i := 0; i < inner; i++ {
					d := int(id[kb+i])
					pg[(o*dim+d)*inner+i] += gd[kb+i]
				}
			}
		}
	})
}

// ----------------------------------------------------------------------------
// Tür uyarlayıcıları
// ----------------------------------------------------------------------------

func (b cpuBackend) CumSum(out Storage, x View, axis int) {
	var gt temp
	defer gt.finish()
	o, last := gt.write(out, false)
	f32CumSum(cd(o), gatherCPU(gt.read(x)), x.Shape, axis, false, false)
	last()
}

func (b cpuBackend) CumSumGrad(gx, g Storage, shape []int, axis int) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32CumSumGrad(d, gt.readD(g), shape, axis)
	last()
}

func (b cpuBackend) Sort(vals, idx Storage, x View, axis, k int, desc bool) {
	var gt temp
	defer gt.finish()
	v, lastV := gt.write(vals, false)
	b.f32Sort(v, idx, gt.read(x), axis, k, desc)
	lastV()
}

func (b cpuBackend) GatherGrad(gx, g, idx Storage, shape []int, axis, k int) {
	var gt temp
	defer gt.finish()
	d, last := gt.write(gx, true)
	b.f32GatherGrad(d, gt.readD(g), gt.readD(idx), shape, axis, k)
	last()
}

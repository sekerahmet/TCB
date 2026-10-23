package tensor

import "math"

// CPU arka ucu: indirgemeler ve softmax.

// AxisOrder bir eksen etrafında tensörü (dış, eksen, iç) üçlüsüne ayırır:
// bitişik düzende eleman (o, d, i) konumu (o*dim + d)*inner + i'dedir.
func AxisOrder(shape []int, axis int) (outer, dim, inner int) {
	outer, inner = 1, 1
	for i := 0; i < axis; i++ {
		outer *= shape[i]
	}
	dim = shape[axis]
	for i := axis + 1; i < len(shape); i++ {
		inner *= shape[i]
	}
	return
}

// Sum indirgemesinde iş bölme ayarları. Seçimler yalnızca şekle bağlıdır (işçi
// sayısına değil); böylece toplama sırası, dolayısıyla sonuç, her makinede ve her
// çalıştırmada bit düzeyinde aynıdır.
const (
	// fewOuter bundan az dış satır varsa eksen boyunca da bölünür.
	fewOuter = 16
	// colBlock geniş iç eksenler sütun bloklarına bu genişlikte bölünür.
	colBlock = 256
)

func (cpuBackend) f32Sum(out Storage, x View, axis int, scale float32) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := AxisOrder(x.Shape, axis)
	sc := float64(scale)
	switch {
	case inner == 1:
		// Son eksen: her çıktı düz bir dilimin toplamı.
		if outer == 1 {
			y[0] = float32(parallelSum(dim, 1, func(lo, hi int) float64 { return total64(xd[lo:hi]) }) * sc)
			return
		}
		parallelFor(outer, dim, func(lo, hi int) {
			for o := lo; o < hi; o++ {
				y[o] = float32(total64(xd[o*dim:(o+1)*dim]) * sc)
			}
		})
	case outer >= fewOuter || outer*dim*inner < parallelThreshold:
		// Her dış satır tek görevde, eksen sırasıyla toplanır.
		parallelFor(outer, dim*inner, func(lo, hi int) {
			acc := make([]float64, inner)
			for o := lo; o < hi; o++ {
				colSum(acc, xd[o*dim*inner:(o+1)*dim*inner], dim, inner, 0, inner)
				scaleWrite(y[o*inner:(o+1)*inner], acc, sc)
			}
		})
	case inner >= 8*colBlock:
		// Az dış satır, geniş iç eksen: (dış satır, sütun bloğu) görevleri.
		nb := (inner + colBlock - 1) / colBlock
		parallelFor(outer*nb, dim*colBlock, func(lo, hi int) {
			acc := make([]float64, colBlock)
			for t := lo; t < hi; t++ {
				o, i0 := t/nb, (t%nb)*colBlock
				i1 := min(i0+colBlock, inner)
				a := acc[:i1-i0]
				colSum(a, xd[o*dim*inner:(o+1)*dim*inner], dim, inner, i0, i1)
				scaleWrite(y[o*inner+i0:o*inner+i1], a, sc)
			}
		})
	default:
		// Az dış satır, dar iç eksen (ör. eksen 0): eksen sabit uzunluklu satır
		// parçalarına bölünür, kısmi toplamlar parça sırasıyla birleştirilir.
		rpc := max(1, reducePart/inner)
		nc := (dim + rpc - 1) / rpc
		partial := make([]float64, outer*nc*inner)
		parallelFor(outer*nc, rpc*inner, func(lo, hi int) {
			for t := lo; t < hi; t++ {
				o, c := t/nc, t%nc
				d0, d1 := c*rpc, min((c+1)*rpc, dim)
				blk := xd[(o*dim+d0)*inner : (o*dim+d1)*inner]
				colSum(partial[t*inner:(t+1)*inner], blk, d1-d0, inner, 0, inner)
			}
		})
		acc := make([]float64, inner)
		for o := 0; o < outer; o++ {
			clear(acc)
			for c := 0; c < nc; c++ {
				for i, v := range partial[(o*nc+c)*inner : (o*nc+c+1)*inner] {
					acc[i] += v
				}
			}
			scaleWrite(y[o*inner:(o+1)*inner], acc, sc)
		}
	}
}

// colSum acc[0:i1-i0] = Σ_d blok[d*inner + i0 : d*inner + i1] (d sırasıyla).
func colSum(acc []float64, blk []float32, dim, inner, i0, i1 int) {
	clear(acc)
	for d := 0; d < dim; d++ {
		row := blk[d*inner+i0 : d*inner+i1]
		a := acc[:len(row)]
		for i, v := range row {
			a[i] += float64(v)
		}
	}
}

func scaleWrite(y []float32, acc []float64, sc float64) {
	for i, v := range acc[:len(y)] {
		y[i] = float32(v * sc)
	}
}

// total64 dilimin float64 toplamı; dört ayrı biriktirici bağımlılık zincirini
// kırar (sıra sabit olduğu için sonuç belirlenimcidir).
func total64(xs []float32) float64 {
	var s0, s1, s2, s3 float64
	for len(xs) >= 4 {
		s0 += float64(xs[0])
		s1 += float64(xs[1])
		s2 += float64(xs[2])
		s3 += float64(xs[3])
		xs = xs[4:]
	}
	for _, v := range xs {
		s0 += float64(v)
	}
	return (s0 + s1) + (s2 + s3)
}

func (cpuBackend) f32SumAll(out Storage, x View, scale float32) {
	xd := gatherCPU(x)
	s := parallelSum(len(xd), 1, func(lo, hi int) float64 { return total64(xd[lo:hi]) })
	cd(out)[0] = float32(s * float64(scale))
}

// argmaxCPU her (dış, iç) satırı için ilk en büyük elemanın eksen indeksini verir. NaN
// her sayıdan büyük sayılır: satırda NaN varsa sonuç ilk NaN'dır (yerinde NaN yayılır;
// maks, min = -maks(-x) ve maks_indeks tutarlı kalır).
func argmaxCPU(xd []float32, outer, dim, inner int, fn func(k, d int, v float32)) {
	parallelFor(outer, dim*inner, func(lo, hi int) {
		for o := lo; o < hi; o++ {
			base := o * dim * inner
			for i := 0; i < inner; i++ {
				best := float32(math.Inf(-1))
				bi := 0
				for d := 0; d < dim; d++ {
					if v := xd[base+d*inner+i]; v > best || (v != v && best == best) {
						best, bi = v, d
					}
				}
				fn(o*inner+i, bi, best)
			}
		}
	})
}

func (cpuBackend) f32Max(out Storage, x View, axis int) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := AxisOrder(x.Shape, axis)
	argmaxCPU(xd, outer, dim, inner, func(k, d int, v float32) { y[k] = v })
}

func (cpuBackend) f32MaxGrad(gx, g Storage, x View, axis int) {
	xd := gatherCPU(x)
	pg, gd := cd(gx), cd(g)
	outer, dim, inner := AxisOrder(x.Shape, axis)
	argmaxCPU(xd, outer, dim, inner, func(k, d int, v float32) {
		o, i := k/inner, k%inner
		pg[(o*dim+d)*inner+i] += gd[k]
	})
}

func (cpuBackend) f32ArgMax(x View, axis int) []int {
	xd := gatherCPU(x)
	outer, dim, inner := AxisOrder(x.Shape, axis)
	res := make([]int, outer*inner)
	argmaxCPU(xd, outer, dim, inner, func(k, d int, v float32) { res[k] = d })
	return res
}

func (cpuBackend) f32Softmax(out Storage, x View, axis int, logMode bool) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := AxisOrder(x.Shape, axis)
	rows := outer * inner
	if inner == 1 {
		// Son eksen: satırlar bitişik; üstel vektörel hesaplanır.
		parallelFor(rows, dim*4, func(lo, hi int) {
			for r := lo; r < hi; r++ {
				softmaxRow(y[r*dim:(r+1)*dim], xd[r*dim:(r+1)*dim], logMode)
			}
		})
		return
	}
	// Diğer eksenler: her satır geçici bitişik tampona toplanıp son eksenle aynı satır
	// işleviyle hesaplanır; böylece sonuç eksenden bağımsızdır (aynı üstel çekirdeği ve
	// aynı toplama sırası) ve üstel vektörel kalır.
	parallelFor(rows, dim*8, func(lo, hi int) {
		tmp := make([]float32, 2*dim)
		xs, ys := tmp[:dim], tmp[dim:]
		for r := lo; r < hi; r++ {
			o, i := r/inner, r%inner
			base := o*dim*inner + i
			for d := range xs {
				xs[d] = xd[base+d*inner]
			}
			softmaxRow(ys, xs, logMode)
			for d, v := range ys {
				y[base+d*inner] = v
			}
		}
	})
}

// softmaxRow bitişik bir satırın (log) softmax'ı; toplam float64'tedir.
func softmaxRow(y, x []float32, logMode bool) {
	y = y[:len(x)]
	// En büyük NaN'ı korur: NaN içeren satır (tamamı NaN olan da) NaN verir, tamamen
	// maskeli (-sonsuz) satırla karışmaz.
	m := float32(math.Inf(-1))
	for _, v := range x {
		if v > m || v != v {
			m = v
		}
	}
	if math.IsInf(float64(m), -1) {
		// Tüm satır -sonsuz (tamamen maskeli): softmax 0, log_softmax -sonsuz.
		empty := float32(0)
		if logMode {
			empty = float32(math.Inf(-1))
		}
		for i := range y {
			y[i] = empty
		}
		return
	}
	for i, v := range x {
		y[i] = v - m
	}
	expVec(y, y)
	s := total64(y)
	if logMode {
		// (x - m) - log Σ: büyük logitlerde hassasiyet kaybını önler.
		ls := float32(math.Log(s))
		for i, v := range x {
			y[i] = (v - m) - ls
		}
		return
	}
	inv := float32(1 / s)
	for i := range y {
		y[i] *= inv
	}
}

func (cpuBackend) f32SoftmaxGrad(gx, g, y Storage, shape []int, axis int, logMode bool) {
	pg, gd, yd := cd(gx), cd(g), cd(y)
	outer, dim, inner := AxisOrder(shape, axis)
	rows := outer * inner
	parallelFor(rows, dim*4, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			o, i := r/inner, r%inner
			base := o*dim*inner + i
			var s float64
			if logMode {
				// dx = g - softmax · Σg. Tamamen maskeli satırda (bütün çıktılar -sonsuz)
				// gradyan 0'dır (softmax'ta y = 0 olduğundan kendiliğinden öyledir).
				empty := true
				for d := 0; d < dim; d++ {
					if !math.IsInf(float64(yd[base+d*inner]), -1) {
						empty = false
						break
					}
				}
				if empty {
					continue
				}
				for d := 0; d < dim; d++ {
					s += float64(gd[base+d*inner])
				}
				sf := float32(s)
				for d := 0; d < dim; d++ {
					k := base + d*inner
					pg[k] += gd[k] - expf32(yd[k])*sf
				}
			} else {
				// dx = y · (g - Σ g·y)
				for d := 0; d < dim; d++ {
					k := base + d*inner
					s += float64(gd[k]) * float64(yd[k])
				}
				sf := float32(s)
				for d := 0; d < dim; d++ {
					k := base + d*inner
					pg[k] += yd[k] * (gd[k] - sf)
				}
			}
		}
	})
}

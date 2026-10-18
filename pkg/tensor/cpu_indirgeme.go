package tensor

import "math"

// CPU arka ucu: indirgemeler ve softmax.

// eksenDuzeni bir eksen etrafında tensörü (dış, eksen, iç) üçlüsüne ayırır:
// bitişik düzende eleman (o, d, i) konumu (o*dim + d)*inner + i'dedir.
func eksenDuzeni(shape []int, axis int) (outer, dim, inner int) {
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
	// azDis bundan az dış satır varsa eksen boyunca da bölünür.
	azDis = 16
	// sutunBlok geniş iç eksenler sütun bloklarına bu genişlikte bölünür.
	sutunBlok = 256
)

func (cpuBackend) Sum(out Storage, x View, axis int, scale float32) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := eksenDuzeni(x.Shape, axis)
	sc := float64(scale)
	switch {
	case inner == 1:
		// Son eksen: her çıktı düz bir dilimin toplamı.
		if outer == 1 {
			y[0] = float32(parallelSum(dim, 1, func(lo, hi int) float64 { return toplam64(xd[lo:hi]) }) * sc)
			return
		}
		parallelFor(outer, dim, func(lo, hi int) {
			for o := lo; o < hi; o++ {
				y[o] = float32(toplam64(xd[o*dim:(o+1)*dim]) * sc)
			}
		})
	case outer >= azDis || outer*dim*inner < paralelEsik:
		// Her dış satır tek görevde, eksen sırasıyla toplanır.
		parallelFor(outer, dim*inner, func(lo, hi int) {
			acc := make([]float64, inner)
			for o := lo; o < hi; o++ {
				sutunTopla(acc, xd[o*dim*inner:(o+1)*dim*inner], dim, inner, 0, inner)
				olcekleYaz(y[o*inner:(o+1)*inner], acc, sc)
			}
		})
	case inner >= 8*sutunBlok:
		// Az dış satır, geniş iç eksen: (dış satır, sütun bloğu) görevleri.
		nb := (inner + sutunBlok - 1) / sutunBlok
		parallelFor(outer*nb, dim*sutunBlok, func(lo, hi int) {
			acc := make([]float64, sutunBlok)
			for t := lo; t < hi; t++ {
				o, i0 := t/nb, (t%nb)*sutunBlok
				i1 := min(i0+sutunBlok, inner)
				a := acc[:i1-i0]
				sutunTopla(a, xd[o*dim*inner:(o+1)*dim*inner], dim, inner, i0, i1)
				olcekleYaz(y[o*inner+i0:o*inner+i1], a, sc)
			}
		})
	default:
		// Az dış satır, dar iç eksen (ör. eksen 0): eksen sabit uzunluklu satır
		// parçalarına bölünür, kısmi toplamlar parça sırasıyla birleştirilir.
		rpc := max(1, indirgemeParca/inner)
		nc := (dim + rpc - 1) / rpc
		kismi := make([]float64, outer*nc*inner)
		parallelFor(outer*nc, rpc*inner, func(lo, hi int) {
			for t := lo; t < hi; t++ {
				o, c := t/nc, t%nc
				d0, d1 := c*rpc, min((c+1)*rpc, dim)
				blok := xd[(o*dim+d0)*inner : (o*dim+d1)*inner]
				sutunTopla(kismi[t*inner:(t+1)*inner], blok, d1-d0, inner, 0, inner)
			}
		})
		acc := make([]float64, inner)
		for o := 0; o < outer; o++ {
			clear(acc)
			for c := 0; c < nc; c++ {
				for i, v := range kismi[(o*nc+c)*inner : (o*nc+c+1)*inner] {
					acc[i] += v
				}
			}
			olcekleYaz(y[o*inner:(o+1)*inner], acc, sc)
		}
	}
}

// sutunTopla acc[0:i1-i0] = Σ_d blok[d*inner + i0 : d*inner + i1] (d sırasıyla).
func sutunTopla(acc []float64, blok []float32, dim, inner, i0, i1 int) {
	clear(acc)
	for d := 0; d < dim; d++ {
		row := blok[d*inner+i0 : d*inner+i1]
		a := acc[:len(row)]
		for i, v := range row {
			a[i] += float64(v)
		}
	}
}

func olcekleYaz(y []float32, acc []float64, sc float64) {
	for i, v := range acc[:len(y)] {
		y[i] = float32(v * sc)
	}
}

// toplam64 dilimin float64 toplamı; dört ayrı biriktirici bağımlılık zincirini
// kırar (sıra sabit olduğu için sonuç belirlenimcidir).
func toplam64(xs []float32) float64 {
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

func (cpuBackend) SumAll(out Storage, x View, scale float32) {
	xd := gatherCPU(x)
	s := parallelSum(len(xd), 1, func(lo, hi int) float64 { return toplam64(xd[lo:hi]) })
	cd(out)[0] = float32(s * float64(scale))
}

// argmaxCPU her (dış, iç) satırı için ilk en büyük elemanın eksen indeksini verir.
func argmaxCPU(xd []float32, outer, dim, inner int, fn func(k, d int, v float32)) {
	parallelFor(outer, dim*inner, func(lo, hi int) {
		for o := lo; o < hi; o++ {
			base := o * dim * inner
			for i := 0; i < inner; i++ {
				best := float32(math.Inf(-1))
				bi := 0
				for d := 0; d < dim; d++ {
					if v := xd[base+d*inner+i]; v > best {
						best, bi = v, d
					}
				}
				fn(o*inner+i, bi, best)
			}
		}
	})
}

func (cpuBackend) Max(out Storage, x View, axis int) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := eksenDuzeni(x.Shape, axis)
	argmaxCPU(xd, outer, dim, inner, func(k, d int, v float32) { y[k] = v })
}

func (cpuBackend) MaxGrad(gx, g Storage, x View, axis int) {
	xd := gatherCPU(x)
	pg, gd := cd(gx), cd(g)
	outer, dim, inner := eksenDuzeni(x.Shape, axis)
	argmaxCPU(xd, outer, dim, inner, func(k, d int, v float32) {
		o, i := k/inner, k%inner
		pg[(o*dim+d)*inner+i] += gd[k]
	})
}

func (cpuBackend) ArgMax(x View, axis int) []int {
	xd := gatherCPU(x)
	outer, dim, inner := eksenDuzeni(x.Shape, axis)
	res := make([]int, outer*inner)
	argmaxCPU(xd, outer, dim, inner, func(k, d int, v float32) { res[k] = d })
	return res
}

func (cpuBackend) Softmax(out Storage, x View, axis int, logKip bool) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := eksenDuzeni(x.Shape, axis)
	rows := outer * inner
	if inner == 1 {
		// Son eksen: satırlar bitişik; üstel vektörel hesaplanır.
		parallelFor(rows, dim*4, func(lo, hi int) {
			for r := lo; r < hi; r++ {
				softmaxSatir(y[r*dim:(r+1)*dim], xd[r*dim:(r+1)*dim], logKip)
			}
		})
		return
	}
	parallelFor(rows, dim*8, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			o, i := r/inner, r%inner
			base := o*dim*inner + i
			m := float32(math.Inf(-1))
			for d := 0; d < dim; d++ {
				if v := xd[base+d*inner]; v > m {
					m = v
				}
			}
			if math.IsInf(float64(m), -1) {
				m = 0 // tüm satır -sonsuz: NaN üretmemek için kaydırma yok
			}
			var s float64
			for d := 0; d < dim; d++ {
				e := expf32(xd[base+d*inner] - m)
				s += float64(e)
				if !logKip {
					y[base+d*inner] = e
				}
			}
			if logKip {
				// (x - m) - log Σ: büyük logitlerde hassasiyet kaybını önler.
				ls := float32(math.Log(s))
				for d := 0; d < dim; d++ {
					y[base+d*inner] = (xd[base+d*inner] - m) - ls
				}
			} else {
				inv := float32(1 / s)
				for d := 0; d < dim; d++ {
					y[base+d*inner] *= inv
				}
			}
		}
	})
}

// softmaxSatir bitişik bir satırın (log) softmax'ı; toplam float64'tedir.
func softmaxSatir(y, x []float32, logKip bool) {
	y = y[:len(x)]
	m := float32(math.Inf(-1))
	for _, v := range x {
		if v > m {
			m = v
		}
	}
	if math.IsInf(float64(m), -1) {
		m = 0 // tüm satır -sonsuz: NaN üretmemek için kaydırma yok
	}
	for i, v := range x {
		y[i] = v - m
	}
	expVek(y, y)
	s := toplam64(y)
	if logKip {
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

func (cpuBackend) SoftmaxGrad(gx, g, y Storage, shape []int, axis int, logKip bool) {
	pg, gd, yd := cd(gx), cd(g), cd(y)
	outer, dim, inner := eksenDuzeni(shape, axis)
	rows := outer * inner
	parallelFor(rows, dim*4, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			o, i := r/inner, r%inner
			base := o*dim*inner + i
			var s float64
			if logKip {
				// dx = g - softmax · Σg
				for d := 0; d < dim; d++ {
					s += float64(gd[base+d*inner])
				}
				sf := float32(s)
				for d := 0; d < dim; d++ {
					k := base + d*inner
					pg[k] += gd[k] - exp32(yd[k])*sf
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

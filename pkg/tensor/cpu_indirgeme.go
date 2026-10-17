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

func (cpuBackend) Sum(out Storage, x View, axis int, scale float32) {
	xd := gatherCPU(x)
	y := cd(out)
	outer, dim, inner := eksenDuzeni(x.Shape, axis)
	sc := float64(scale)
	parallelFor(outer, dim*inner, func(lo, hi int) {
		acc := make([]float64, inner)
		for o := lo; o < hi; o++ {
			for i := range acc {
				acc[i] = 0
			}
			base := o * dim * inner
			for d := 0; d < dim; d++ {
				row := xd[base+d*inner : base+(d+1)*inner]
				for i, v := range row {
					acc[i] += float64(v)
				}
			}
			for i, v := range acc {
				y[o*inner+i] = float32(v * sc)
			}
		}
	})
}

func (cpuBackend) SumAll(out Storage, x View, scale float32) {
	xd := gatherCPU(x)
	var s float64
	for _, v := range xd {
		s += float64(v)
	}
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
				e := math.Exp(float64(xd[base+d*inner] - m))
				s += e
				if !logKip {
					y[base+d*inner] = float32(e)
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

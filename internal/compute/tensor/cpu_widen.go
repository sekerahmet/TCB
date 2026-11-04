package tensor

// widen16 dst = float32(src) (paralel).
func widen16(dst []float32, src []uint16, dt DType) {
	src = src[:len(dst)]
	parallelFor(len(dst), 2, func(lo, hi int) { widenSpan(dst[lo:hi], src[lo:hi], dt) })
}

// widenSpan dst = float32(src), tek iş parçacığında (src en az dst kadar uzun).
func widenSpan(dst []float32, src []uint16, dt DType) {
	src = src[:len(dst)]
	if dt == BFloat16 {
		for i, v := range src {
			dst[i] = bf16ToF32(v)
		}
		return
	}
	for i, v := range src {
		dst[i] = f16ToF32(v)
	}
}

// widenInto 16 bitlik görünümün elemanlarını mantıksal sırayla float32
// olarak out'a yazar.
func widenInto(out []float32, v View) {
	c := cpuStore(v.S)
	if v.IsContiguous() {
		widen16(out, c.h[v.Offset:v.Offset+len(out)], c.dtyp)
		return
	}
	bf := c.dtyp == BFloat16
	iterRowsPar(v.Shape, []int{v.Offset, 0}, [][]int{v.Strides, ContiguousStrides(v.Shape)}, 1, func(off []int, m int, in []int) {
		src, dst, s := off[0], off[1], in[0]
		o := out[dst : dst+m]
		for i := range o {
			if bf {
				o[i] = bf16ToF32(c.h[src+i*s])
			} else {
				o[i] = f16ToF32(c.h[src+i*s])
			}
		}
	})
}

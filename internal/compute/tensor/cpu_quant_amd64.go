package tensor

// sdotq8 x·aç(w)'dir ("q8", n 32'nin katı; F16C ister). Assembly: cpu_quant_amd64.s.
//
//go:noescape
func sdotq8(n int, x *float32, w *byte) float32

// sdotq4k x·aç(w)'dir ("q4_k"; blocks blok). scales blok başına 16 float32'dir: dört grubun d·a ve
// m·b çiftleri, quantScalesQ4K. Assembly: cpu_quant_amd64.s.
//
//go:noescape
func sdotq4k(blocks int, x *float32, w *byte, scales *float32) float32

// sdotq6k x·aç(w)'dir ("q6_k"; blocks blok). scales blok başına 16 float32'dir: d·a, quantScalesQ6K.
// Assembly: cpu_quant_amd64.s.
//
//go:noescape
func sdotq6k(blocks int, x *float32, w *byte, scales *float32) float32

// prefetchBytes p'den n baytı önbelleğe çağırır; adres geçersizse hiçbir şey olmaz, bellek okunmaz.
// Assembly: cpu_quant_amd64.s.
//
//go:noescape
func prefetchBytes(p uintptr, n int)

// quantScalesQ4K blok başına [d·a₀, m·b₀, d·a₁, m·b₁, ...] yazar; çarpımlar float32'de kesindir.
func quantScalesQ4K(out []float32, w []byte) {
	for o, b := 0, 0; b < len(w); o, b = o+16, b+q4kBytes {
		block := w[b : b+q4kBytes]
		d, minimum := fp16(block), fp16(block[2:])
		for j := range 8 {
			sc, m := q4kScale(j, block[4:16])
			out[o+2*j], out[o+2*j+1] = d*float32(sc), minimum*float32(m)
		}
	}
}

// quantScalesQ6K blok başına d·a yazar; çarpımlar float32'de kesindir.
func quantScalesQ6K(out []float32, w []byte) {
	for o, b := 0, 0; b < len(w); o, b = o+16, b+q6kBytes {
		block := w[b : b+q6kBytes]
		d := fp16(block[208:])
		for j, sc := range block[192:208] {
			out[o+j] = d * float32(int8(sc))
		}
	}
}

// quantDot x·aç(w)'dir, dotF32'nin sırasıyla: çekirdek blokları yazmaçta açar, açılan değer ve
// toplama "aç + dotF32" ile bit bit aynıdır. AVX2 ya da F16C yoksa Go eşi.
func quantDot(dt DType, x []float32, w []byte, s *quantScratch) float32 {
	if !hasAVX2 || !hasF16C {
		return dotQuantGo(dt, x, w, s)
	}
	switch dt {
	case Q4K:
		quantScalesQ4K(s.scales, w)
		return sdotq4k(len(x)/kBlock, &x[0], &w[0], &s.scales[0])
	case Q6K:
		quantScalesQ6K(s.scales, w)
		return sdotq6k(len(x)/kBlock, &x[0], &w[0], &s.scales[0])
	}
	return sdotq8(len(x), &x[0], &w[0])
}

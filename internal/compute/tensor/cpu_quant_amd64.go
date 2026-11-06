package tensor

// sdotq8 x·aç(w)'dir ("q8", n 32'nin katı; F16C ister). Assembly: cpu_quant_amd64.s.
//
//go:noescape
func sdotq8(n int, x *float32, w *byte) float32

// sdotq4k x·aç(w)'dir ("q4_k"; blocks blok). scales blok başına 16 float32'lik çalışma alanıdır:
// çekirdek dört grubun d·a ve m·b çiftlerini oraya açar, çarpımlar float32'de kesindir. Go'da
// açmak çekirdeğin kendisinden uzun sürüyordu [ölçüldü: h1k2b]. Assembly: cpu_quant_amd64.s.
//
//go:noescape
func sdotq4k(blocks int, x *float32, w *byte, scales *float32) float32

// sdotq6k x·aç(w)'dir ("q6_k"; blocks blok). scales blok başına 16 float32'lik çalışma alanıdır:
// çekirdek d·a'ları oraya açar, çarpımlar float32'de kesindir. Assembly: cpu_quant_amd64.s.
//
//go:noescape
func sdotq6k(blocks int, x *float32, w *byte, scales *float32) float32

// quantDot x·aç(w)'dir, dotF32'nin sırasıyla: çekirdek blokları yazmaçta açar, açılan değer ve
// toplama "aç + dotF32" ile bit bit aynıdır. AVX2 ya da F16C yoksa Go eşi.
func quantDot(dt DType, x []float32, w []byte, s *quantScratch) float32 {
	if !hasAVX2 || !hasF16C {
		return dotQuantGo(dt, x, w, s)
	}
	switch dt {
	case Q4K:
		return sdotq4k(len(x)/kBlock, &x[0], &w[0], &s.scales[0])
	case Q6K:
		return sdotq6k(len(x)/kBlock, &x[0], &w[0], &s.scales[0])
	}
	return sdotq8(len(x), &x[0], &w[0])
}

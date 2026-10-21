package tensor

// sgemv64 y[0:64] += Σ_i x[i]·w[i*ldw + 0:64] (i < k). Assembly: cpu_gemv_amd64.s.
//
//go:noescape
func sgemv64(k int, x *float32, w *float32, ldw int, y *float32)

// sdot32 Σ x[i]·y[i] (n 32'nin katı). Assembly: cpu_gemv_amd64.s.
//
//go:noescape
func sdot32(n int, x *float32, y *float32) float32

// gemvStrip y[0:64] += Σ_k a[k]·b[k*ldb + 0:64]; AVX2 yoksa Go eşi.
func gemvStrip(a, b []float32, ldb int, y []float32) {
	if hasAVX2 && len(a) > 0 {
		_ = b[(len(a)-1)*ldb+63]
		_ = y[63]
		sgemv64(len(a), &a[0], &b[0], ldb, &y[0])
		return
	}
	gemvStripGo(a, b, ldb, y[:64])
}

// dotF32 x·y; AVX2'de 32'lik bloklar assembly ile, kalan Go ile (sabit sıra).
func dotF32(x, y []float32) float32 {
	y = y[:len(x)]
	if !hasAVX2 || len(x) < 32 {
		return dot32(x, y)
	}
	n := len(x) &^ 31
	s := sdot32(n, &x[0], &y[0])
	if n < len(x) {
		s += dot32(x[n:], y[n:])
	}
	return s
}

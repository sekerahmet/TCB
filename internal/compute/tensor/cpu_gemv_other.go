//go:build !amd64

package tensor

// gemvStrip y[0:64] += Σ_k a[k]·b[k*ldb + 0:64] (amd64 dışında Go eşi).
func gemvStrip(a, b []float32, ldb int, y []float32) { gemvStripGo(a, b, ldb, y[:64]) }

// dotF32 x·y (amd64 dışında Go eşi).
func dotF32(x, y []float32) float32 { return dot32(x, y) }

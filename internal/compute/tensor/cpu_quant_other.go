//go:build !amd64

package tensor

// prefetchBytes amd64 dışında bir şey yapmaz.
func prefetchBytes(uintptr, int) {}

// quantDot x·aç(w)'dir (amd64 dışında Go eşi).
func quantDot(dt DType, x []float32, w []byte, s *quantScratch) float32 {
	return dotQuantGo(dt, x, w, s)
}

//go:build !amd64

package tensor

// direct16 amd64 dışında 16 bitlik yol hep Go eşidir.
func direct16(DType) bool { return true }

// gemvStrip16 gemvStrip'in 16 bitlik eşi (amd64 dışında Go).
func gemvStrip16(a []float32, b []uint16, ldb int, y []float32, dt DType) {
	gemvStripGo16(a, b, ldb, y[:64], dt)
}

// dot16 x·float32(y) (amd64 dışında Go): y row'a açılır, sonra dotF32 sırasıyla toplanır.
func dot16(x []float32, y []uint16, dt DType, row []float32) float32 {
	widenSpan(row[:len(x)], y, dt)
	return dot32(x, row[:len(x)])
}

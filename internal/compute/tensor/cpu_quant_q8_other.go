//go:build !amd64

package tensor

func q8Finite(a []float32) bool { return q8FiniteGo(a) }

func quantizeQ8(x []float32, r *q8kRow) { quantizeQ8Go(x, r) }

func dotQ4KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 { return dotQ4KQ8Go(blocks, w, r) }

func dotQ4KQ8W4(blocks int, w []byte, rowBytes int, r *q8kRow, tmp []byte, out []float32) {
	dotQ8W4Go(dotQ4KQ8Go, blocks, 144, w, rowBytes, r, out)
}

func dotQ6KQ8W4(blocks int, w []byte, rowBytes int, r *q8kRow, tmp []byte, out []float32) {
	dotQ8W4Go(dotQ6KQ8Go, blocks, 210, w, rowBytes, r, out)
}

func dotQ4KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	dotQ8x4Go(dotQ4KQ8Go, blocks, w, r0, K, out)
}

// q4x8Ready: sekiz satırlı çekirdek yalnız amd64'te; burada quantGemmQ8 dörtlü gruplarla çarpar.
func q4x8Ready() bool { return false }

func dotQ4KQ8x8(blocks int, w []byte, r0 *q8kRow, K int, dT []float32, ps []int16, tmp []byte, out []float32) {
	dotQ8RowsGo(8, dotQ4KQ8Go, blocks, w, r0, K, out)
}

func dotQ4Q8(blocks int, w []byte, r *q8kRow) float32 { return dotQ4Q8Go(blocks, w, r) }

func dotQ6KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 { return dotQ6KQ8Go(blocks, w, r) }

func dotQ6KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	dotQ8x4Go(dotQ6KQ8Go, blocks, w, r0, K, out)
}

// q4PanelGemm yalnız amd64'te (q4x8Ready burada false; çağrılmaz).
func q4PanelGemm(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, c8 []int32, idx []int, c []float32, pan []byte, ps []float32) int {
	return n0
}

func q4kSubSums(rows []*q8kRow, nb int) []int16 { return nil }

func q4kPanelGemm(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, bs32 []int16, idx []int, c []float32, pan, pmeta []byte) int {
	return n0
}

func q6kPanelGemm(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, idx []int, c []float32, pan, pmeta []byte) int {
	return n0
}

type q8Panel struct{ on bool }

func newQ8Panel(dt DType, rows []*q8kRow, nb int) q8Panel { return q8Panel{} }

func (p q8Panel) alloc() ([]byte, []byte, []float32) { return nil, nil, nil }

func (p q8Panel) run(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, idx []int, c []float32, pan, pmeta []byte, ps []float32) int {
	return n0
}

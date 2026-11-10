//go:build !amd64

package tensor

func dotQ4KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 { return dotQ4KQ8Go(blocks, w, r) }

func dotQ4KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	dotQ4KQ8x4Go(blocks, w, r0, K, out)
}

func dotQ6KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 { return dotQ6KQ8Go(blocks, w, r) }

func dotQ6KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	dotQ6KQ8x4Go(blocks, w, r0, K, out)
}

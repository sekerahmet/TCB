package tensor

// sdotq4kq8 dotQ4KQ8Go'nun AVX2 eşidir (blocks blok; tmp 16 baytlık çalışma alanı). Assembly: cpu_quant_q8_amd64.s.
//
//go:noescape
func sdotq4kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32

func dotQ4KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if !hasAVX2 || !hasF16C {
		return dotQ4KQ8Go(blocks, w, r)
	}
	return sdotq4kq8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0], &tmp[0])
}

// sdotq4kq8x4 dört Q8 satırını aynı Q4_K satırıyla çarpar (satırlar sq, sd, sb bayt arayla); out[r] += sonuç.
//
//go:noescape
func sdotq4kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)

// dotQ4KQ8x4 r0'dan başlayan dört bitişik satırı (aynı tampondan, K arayla) çarpar.
func dotQ4KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C {
		dotQ4KQ8x4Go(blocks, w, r0, K, out)
		return
	}
	sdotq4kq8x4(blocks, &w[0], &r0.q[0], &r0.d[0], &r0.bs[0], &tmp[0], &out[0], K, K/kBlock*4, K/16*2)
}

// sdotq6kq8 dotQ6KQ8Go'nun AVX2 eşidir (tmp 32 baytlık çalışma alanı).
//
//go:noescape
func sdotq6kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32

func dotQ6KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if !hasAVX2 || !hasF16C {
		return dotQ6KQ8Go(blocks, w, r)
	}
	return sdotq6kq8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0], &tmp[0])
}

// sdotq6kq8x4 dört Q8 satırını aynı Q6_K satırıyla çarpar (satırlar sq, sd, sb bayt arayla; tmp 48 bayt);
// out[r] += sonuç.
//
//go:noescape
func sdotq6kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)

// dotQ6KQ8x4 r0'dan başlayan dört bitişik satırı (aynı tampondan, K arayla) çarpar.
func dotQ6KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C {
		dotQ6KQ8x4Go(blocks, w, r0, K, out)
		return
	}
	_ = tmp[47]
	sdotq6kq8x4(blocks, &w[0], &r0.q[0], &r0.d[0], &r0.bs[0], &tmp[0], &out[0], K, K/kBlock*4, K/16*2)
}

package tensor

// Sarmalayıcıların sınır denetimleri: assembly dilimlerin dışına çıkmaz, çünkü burada çekirdeğin okuyacağı her
// aralık (ağırlık, kodlar, ölçekler, alt toplamlar, tmp, out) Go'da bir kez dilimlenir; çağıranın sözü yetmezse
// assembly sessizce okumak yerine Go panik verir.

// sdotq4kq8 dotQ4KQ8Go'nun AVX2 eşidir (blocks blok; tmp 16 baytlık çalışma alanı). Assembly: cpu_quant_q8_amd64.s.
//
//go:noescape
func sdotq4kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32

func dotQ4KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		return dotQ4KQ8Go(blocks, w, r)
	}
	_, _, _, _, _ = w[:blocks*144], r.q[:blocks*kBlock], r.d[:blocks], r.bs[:blocks*16], tmp[:16]
	return sdotq4kq8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0], &tmp[0])
}

// sdotq4kq8x4 dört Q8 satırını aynı Q4_K satırıyla çarpar (satırlar sq, sd, sb bayt arayla; tmp 32 bayt);
// her bloğun sonucunu out[r]'ye ekler.
//
//go:noescape
func sdotq4kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)

// dotQ4KQ8x4 r0'dan başlayan dört bitişik satırı (aynı tampondan, K arayla) çarpar: out[r] = r'inci satırın sonucu.
func dotQ4KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8x4Go(dotQ4KQ8Go, blocks, w, r0, K, out)
		return
	}
	checkQ8x4(blocks, 144, w, r0, K, tmp[:32], out)
	clear(out[:4])
	sdotq4kq8x4(blocks, &w[0], &r0.q[0], &r0.d[0], &r0.bs[0], &tmp[0], &out[0], K, K/kBlock*4, K/16*2)
}

// sdotq6kq8 dotQ6KQ8Go'nun AVX2 eşidir (tmp 32 baytlık çalışma alanı).
//
//go:noescape
func sdotq6kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32

func dotQ6KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		return dotQ6KQ8Go(blocks, w, r)
	}
	_, _, _, _, _ = w[:blocks*210], r.q[:blocks*kBlock], r.d[:blocks], r.bs[:blocks*16], tmp[:32]
	return sdotq6kq8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0], &tmp[0])
}

// sdotq6kq8x4 dört Q8 satırını aynı Q6_K satırıyla çarpar (satırlar sq, sd, sb bayt arayla; tmp 48 bayt);
// her bloğun sonucunu out[r]'ye ekler.
//
//go:noescape
func sdotq6kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)

// dotQ6KQ8x4 r0'dan başlayan dört bitişik satırı (aynı tampondan, K arayla) çarpar: out[r] = r'inci satırın sonucu.
func dotQ6KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8x4Go(dotQ6KQ8Go, blocks, w, r0, K, out)
		return
	}
	checkQ8x4(blocks, 210, w, r0, K, tmp[:48], out)
	clear(out[:4])
	sdotq6kq8x4(blocks, &w[0], &r0.q[0], &r0.d[0], &r0.bs[0], &tmp[0], &out[0], K, K/kBlock*4, K/16*2)
}

// checkQ8x4 dört satırlı çekirdeğin okuyacağı aralıkları Go'da dilimler: ağırlık satırı, r0'ın arkasındaki üç
// satırın kodları, ölçekleri ve alt toplamları (bitişik tamponun kapasitesinden), out'un dört öğesi.
func checkQ8x4(blocks, blockBytes int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	nb := K / kBlock
	_, _, _, _ = w[:blocks*blockBytes], r0.q[:3*K+blocks*kBlock], r0.d[:3*nb+blocks], r0.bs[:3*K/16+blocks*16]
	_, _ = tmp[0], out[:4]
}

package tensor

// Tek satırlı çarpımın dört ağırlık satırlı çekirdekleri ve AVX2 nicemleme sarmalayıcıları.
// Sınırlar burada Go dilimleriyle denetlenir; kısa tampon assembly'ye girmeden panik verir.

// q6w4Const sdotq6kw4q8 ve sdotq6kw4q8Vnni'nin sabit tablosudur:
//   - [0..31]: AVX2 için dört adet 32 baytlık VPSHUFB ölçek maskesi,
//   - [32..39]: 0x30303030 maskesi,
//   - [40..47] ve [48..55]: VNNI için çift-alt-blok birleşik VPSHUFB ölçek maskeleri.
var q6w4Const = func() (t [56]uint32) {
	for p := range 4 {
		lo := uint32(4*p) | uint32(4*p+1)<<8
		lo |= lo << 16
		hi := uint32(4*p+2) | uint32(4*p+3)<<8
		hi |= hi << 16
		for k := range 4 {
			t[p*8+k] = lo
			t[p*8+4+k] = hi
		}
	}
	for k := range 8 {
		t[32+k] = 0x30303030
	}
	lo13 := uint32(0) | uint32(1)<<8 | uint32(8)<<16 | uint32(9)<<24
	hi13 := uint32(2) | uint32(3)<<8 | uint32(10)<<16 | uint32(11)<<24
	lo24 := uint32(4) | uint32(5)<<8 | uint32(12)<<16 | uint32(13)<<24
	hi24 := uint32(6) | uint32(7)<<8 | uint32(14)<<16 | uint32(15)<<24
	for k := range 4 {
		t[40+k], t[44+k] = lo13, hi13
		t[48+k], t[52+k] = lo24, hi24
	}
	return
}()

//go:noescape
func q8FiniteAsm(x *float32, n int) bool

// q8Finite a'nın tüm öğelerinin sonlu olduğunu denetler (AVX2 hızlı yolu + Go kuyruğu).
func q8Finite(a []float32) bool {
	if !hasAVX2 || len(a) < 8 {
		return q8FiniteGo(a)
	}
	m := len(a) &^ 7
	if !q8FiniteAsm(&a[0], m) {
		return false
	}
	return q8FiniteGo(a[m:])
}

//go:noescape
func quantizeQ8Asm(x *float32, q *int8, d *float32, bs *int16, nb, blk int)

// quantizeQ8 x'i r.blk'lık bloklarla nicemler; AVX2 yolu quantizeQ8Go ile bit bit aynıdır.
func quantizeQ8(x []float32, r *q8kRow) {
	nb := len(x) / r.blk
	if !hasAVX2 || nb == 0 || r.blk < 32 || r.blk%32 != 0 {
		quantizeQ8Go(x, r)
		return
	}
	_, _, _, _ = x[:nb*r.blk], r.q[:nb*r.blk], r.d[:nb], r.bs[:nb*(r.blk/16)]
	quantizeQ8Asm(&x[0], &r.q[0], &r.d[0], &r.bs[0], nb, r.blk)
}

//go:noescape
func sdotq4kw4q8(blocks int, w *byte, rowBytes int, q *int8, d *float32, bs *int16, tmp *byte, out *float32)

//go:noescape
func sdotq4kw4q8Vnni(blocks int, w *byte, rowBytes int, q *int8, d *float32, bs *int16, tmp *byte, out *float32)

// dotQ4KQ8W4 dört Q4_K ağırlık satırını aynı Q8 satırıyla çarpar: out[j] = dotQ4KQ8Go(j'inci ağırlık satırı).
func dotQ4KQ8W4(blocks int, w []byte, rowBytes int, r *q8kRow, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8W4Go(dotQ4KQ8Go, blocks, 144, w, rowBytes, r, out)
		return
	}
	_, _, _ = w[:3*rowBytes+blocks*144], r.q[:blocks*kBlock], r.d[:blocks]
	_, _, _ = r.bs[:blocks*16], tmp[:64], out[:4]
	if hasAVXVNNI {
		sdotq4kw4q8Vnni(blocks, &w[0], rowBytes, &r.q[0], &r.d[0], &r.bs[0], &tmp[0], &out[0])
		return
	}
	sdotq4kw4q8(blocks, &w[0], rowBytes, &r.q[0], &r.d[0], &r.bs[0], &tmp[0], &out[0])
}

//go:noescape
func sdotq6kw4q8(blocks int, w *byte, rowBytes int, q *int8, d *float32, bs *int16, cnst *uint32, out *float32)

//go:noescape
func sdotq6kw4q8Vnni(blocks int, w *byte, rowBytes int, q *int8, d *float32, bs *int16, cnst *uint32, out *float32)

// dotQ6KQ8W4 dört Q6_K ağırlık satırını aynı Q8 satırıyla çarpar: out[j] = dotQ6KQ8Go(j'inci ağırlık satırı).
func dotQ6KQ8W4(blocks int, w []byte, rowBytes int, r *q8kRow, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8W4Go(dotQ6KQ8Go, blocks, 210, w, rowBytes, r, out)
		return
	}
	_, _, _ = w[:3*rowBytes+blocks*210], r.q[:blocks*kBlock], r.d[:blocks]
	_, _ = r.bs[:blocks*16], out[:4]
	if hasAVXVNNI {
		sdotq6kw4q8Vnni(blocks, &w[0], rowBytes, &r.q[0], &r.d[0], &r.bs[0], &q6w4Const[0], &out[0])
		return
	}
	sdotq6kw4q8(blocks, &w[0], rowBytes, &r.q[0], &r.d[0], &r.bs[0], &q6w4Const[0], &out[0])
}

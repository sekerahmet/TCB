package tensor

import "time"

// Nicemlenmiş ağırlıkla çarpım ve gömme (15.53). Sağ işlenen [N, K] deposudur, bloklar K boyuncadır;
// etkinleştirme nicemlenmez, biriktirme float32'dedir. Bütün ağırlık hiç açılmaz: tek satırda
// çekirdek blokları yazmaçta açar, çok satırda satır parçaları sırayla tampona açılır.

// quantTileRows çok satırlı çarpımın bir parçasında açılan ağırlık satırıdır. Sabittir: toplama
// sırası yalnız şekle bağlı kalır. Tampon en çok K·256 float32'dir: Qwen3-4B'nin en geniş
// ağırlığında (K = 9728) 10 MB [hesap].
const quantTileRows = 256

// quantHold tek satırlı çarpımdan sonra işçilerin uyumadan beklediği süredir: üretimde sıradaki
// çarpım bundan önce gelir. 2026-11-05 ölçümü (EPYC 7B12, Qwen3-0.6B Q8_0, ms/parça, ortanca):
// 50 µs'de 37,5; 300'de 32,5; 500'de 31,6; 1000'de 31,1; 2000'de 30,8.
const quantHold = time.Millisecond

// quantRowBytes K öğelik bir ağırlık satırının baytıdır.
func quantRowBytes(dt DType, K int) int {
	elems, size := dt.QuantBlock()
	return K / elems * size
}

// quantScratch bir iş parçacığının satır tamponlarıdır: Go eşinin açılmış satırı ve çekirdeğin
// ölçek tablosu (blok başına 16 float32). Çekirdek yolunda satır hiç ayrılmaz.
type quantScratch struct{ row, scales []float32 }

func newQuantScratch(dt DType, K int) *quantScratch {
	if dt == Q8 {
		return &quantScratch{}
	}
	return &quantScratch{scales: make([]float32, K/16)}
}

// dotQuantGo x·aç(w)'dir: quantDot'un Go eşi ve ölçütüdür. Sonuç "aç + dotF32"nin kendisidir.
func dotQuantGo(dt DType, x []float32, w []byte, s *quantScratch) float32 {
	if s.row == nil {
		s.row = make([]float32, len(x))
	}
	dequantBlocks(dt, s.row, w)
	return dotF32(x, s.row)
}

// quantGemm c[m, n] += Σ_k a[m, k]·aç(w)[n, k]'dir (a [M, K], w [N, K] deposu, c [M, N]).
func quantGemm(M, N, K int, a []float32, w []byte, dt DType, c []float32) {
	if K == 0 {
		return // boş toplam: c olduğu gibi kalır, float çarpımıyla aynı
	}
	if M == 1 {
		quantGemv(N, K, a, w, dt, c)
		return
	}
	rowBytes := quantRowBytes(dt, K)
	rows := min(quantTileRows, N)
	// Tamponlar havuzdandır: her çarpımda ayrılsalar 128 konumluk geçişte 900 MB çöp olurlardı.
	// tile her turda yazılır, part sıfırlanır: önceki çağrının verisi okunmaz.
	tp, pp := takePack(&packPoolB, rows*K), takePack(&packPoolA, M*rows)
	defer packPoolB.Put(tp)
	defer packPoolA.Put(pp)
	tile, part := *tp, *pp
	for n0 := 0; n0 < N; n0 += rows {
		t := min(rows, N-n0)
		parallelFor(t, K, func(lo, hi int) {
			dequantBlocks(dt, tile[lo*K:hi*K], w[(n0+lo)*rowBytes:(n0+hi)*rowBytes])
		})
		clear(part[:M*t])
		gemm(1, M, t, K, a, false, tile[:t*K], true, part[:M*t])
		parallelFor(M, t, func(lo, hi int) {
			for m := lo; m < hi; m++ {
				out := c[m*N+n0 : m*N+n0+t]
				for j, v := range part[m*t : (m+1)*t] {
					out[j] += v
				}
			}
		})
	}
}

// quantGemv tek satırlık çarpımdır: her çıktı bir ağırlık satırının nokta çarpımıdır, sırası
// parça sınırlarından bağımsızdır (belirlenimci).
func quantGemv(N, K int, a []float32, w []byte, dt DType, c []float32) {
	rowBytes := quantRowBytes(dt, K)
	self := prefetchBefore(w)
	parallelFor(N, K, func(lo, hi int) {
		s := newQuantScratch(dt, K)
		for n := lo; n < hi; n++ {
			c[n] += quantDot(dt, a, w[n*rowBytes:(n+1)*rowBytes], s)
		}
	})
	holdWorkers(quantHold)
	prefetchAfter(self)
}

// quantEmbedding idx satırlarını float32'ye açar (15.53): yalnız seçilen satırların blokları.
func quantEmbedding(out []float32, raw []byte, dt DType, idx []int, D int) {
	rowBytes := quantRowBytes(dt, D)
	parallelFor(len(idx), D, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			r := idx[i]
			dequantBlocks(dt, out[i*D:(i+1)*D], raw[r*rowBytes:(r+1)*rowBytes])
		}
	})
}

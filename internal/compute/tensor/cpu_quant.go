package tensor

import "time"

// Nicemlenmiş ağırlıkla çarpım ve gömme (15.53). Sağ işlenen [N, K] deposudur, bloklar K boyuncadır;
// etkinleştirme nicemlenmez, biriktirme float32'dedir. Bütün ağırlık hiç açılmaz: tek satırda
// çekirdek blokları yazmaçta açar, çok satırda satır parçaları sırayla tampona açılır.

// Çok satırlı çarpımın iş bölümü. Parça boyu ve satır bloğu bitleri değiştirmez: her öğe
// kcBlock parçalarıyla toplanır, sıra yalnız K'ye bağlıdır (bkz. gemmTiles).
//
// quantTileRows bir parçada açılan en çok ağırlık satırıdır. Tamponlar havuzda kalsın diye parça
// ayrıca bufMaxB'yi, M satırlık ara sonuç bufMax'ı aşmaz (quantTile): Qwen3-4B'de 512 parçalık
// istemde K = 2560'ta 1024, K = 9728'de 416 satır [hesap].
//
// quantRowBlock gemm'in bir iş parçasındaki en az satırdır. gemm kendi başına işçi başına sekiz
// parça ister: M = 512 ve 20 işçide 6 satır; açılan her ağırlık paneli 6 satır için okunur.
// 48 satırda aynı panel 8 kat çok satıra kullanılır [hesap]. İş azsa sütun yönünde bölünür.
const (
	quantTileRows = 1024
	quantRowBlock = 48
)

// quantTile [M, K]·[N, K]ᵀ çarpımında bir parçanın ağırlık satırıdır: en çok quantTileRows ve N;
// açılan parça bufMaxB'yi, ara sonuç bufMax'ı aşmaz; nr'nin katıdır (en az nr).
func quantTile(M, N, K int) int {
	rows := min(quantTileRows, bufMaxB/K, bufMax/M) / nr * nr
	return min(max(rows, nr), N)
}

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
	rows := quantTile(M, N, K)
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
		gemmPart(1, M, t, K, a, false, tile[:t*K], true, part[:M*t], quantRowBlock) // M > 1: gemv yolu yok
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
	parallelFor(N, K, func(lo, hi int) {
		s := newQuantScratch(dt, K)
		for n := lo; n < hi; n++ {
			c[n] += quantDot(dt, a, w[n*rowBytes:(n+1)*rowBytes], s)
		}
	})
	holdWorkers(quantHold)
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

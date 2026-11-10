package tensor

import "time"

// Nicemlenmiş ağırlıkla çarpım ve gömme (15.53). Sağ işlenen [N, K] deposudur, bloklar K boyuncadır;
// etkinleştirme nicemlenmez, biriktirme float32'dedir. Bütün ağırlık hiç açılmaz: tek satırda
// çekirdek blokları yazmaçta açar, az satırda (quantPanelMaxM) her iş nr satırı mikro çekirdeğin
// paneline açar, daha çok satırda satır parçaları sırayla tampona açılır.

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
	if quantQ8Ready(dt) { // H1 hız prototipi (15.53): ağırlık açılmaz, bkz. cpu_quant_q8.go
		if M == 1 {
			quantGemvQ8(N, K, a, w, dt, c)
		} else {
			quantGemmQ8(M, N, K, a, w, dt, c)
		}
		return
	}
	if M == 1 {
		quantGemv(N, K, a, w, dt, c)
		return
	}
	if M <= quantPanelMaxM {
		quantGemmPanels(M, N, K, a, w, dt, c)
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

// quantPanelMaxM bu kadar satıra kadar çok satırlı çarpım quantGemmPanels'tedir. Sınır gemm'in en
// büyük satır bloğudur (rowB); paketli A en geniş ağırlıkta (K = 9728) 96 × 9728 float32, packPoolA'nın
// sakladığı boyun (bufMax) altında kalır [hesap]; tamponlar quantPanelPool'dadır.
const quantPanelMaxM = rowB

// quantPanelPool quantGemmPanels'in tamponlarıdır (paketli A; panel, satır ve çıktı parçası tek
// tamponda). Ortak packPoolA ve packPoolB'den alınsalar yığınların toplam sınırını (bufTotalMax)
// doldururlardı; parça yolunun 16 MB'lık tamponları o zaman geri konmaz, her çarpımda yeniden ayrılırdı.
// Ortak yığınla 630 satırlık okumada tepe özel bellek tabanınkinden ~3,9 GiB yüksekti (H1 sabit bedel).
var quantPanelPool = bufStack{largest: bufMax}

// quantGemmPanels az satırlı çarpımdır (sohbet turunun yeni parçaları). Parça yolunda (quantGemm) her
// 256 satırlık ağırlık önce tampona açılıyor, sonra gemm onu sütun sütun panellere yeniden yazıyor ve
// A'yı her parçada yeniden paketliyordu; 21 satırlık okumada süre bu kopyalarda geçiyordu (H1 sabit
// bedel, profil). Burada her iş nr ağırlık satırını doğrudan packBPanel'in devrik düzenine (K×nr) açar,
// A bir kez packA'nın düzeninde paketlenir. Mikro çekirdek her kc bloğunda gemmPart'ın çağrısıyla aynı
// girdiyi alır ve sıfırlanmış bir tampona biriktirir, tampon sonra c'ye eklenir: toplama sırası parça
// yolununkiyle aynıdır, bitler değişmez (TestQuantMatMulSameBitsAsOpened).
func quantGemmPanels(M, N, K int, a []float32, w []byte, dt DType, c []float32) {
	rowBytes := quantRowBytes(dt, K)
	mPad := (M + mr - 1) / mr * mr
	apBuf := takePack(&quantPanelPool, mPad*K)
	defer quantPanelPool.Put(apBuf)
	ap := *apBuf
	// kc bloğu p0'ın panelleri ap[p0*mPad:]'dedir: önceki blokların hepsi kcBlock derinliğindedir.
	for p0 := 0; p0 < K; p0 += kcBlock {
		kb := min(kcBlock, K-p0)
		packA(a, K, 1, 0, M, p0, kb, ap[p0*mPad:p0*mPad+mPad*kb])
	}
	np := (N + nr - 1) / nr
	parallelFor(np, K*(nr+mPad), func(lo, hi int) {
		bb := takePack(&quantPanelPool, K*nr+K+mPad*nr)
		defer quantPanelPool.Put(bb)
		pan, row, part := (*bb)[:K*nr], (*bb)[K*nr:K*nr+K], (*bb)[K*nr+K:K*nr+K+mPad*nr]
		for jp := lo; jp < hi; jp++ {
			j0 := jp * nr
			nn := min(nr, N-j0)
			if nn < nr {
				clear(pan) // eksik sütunlar sıfırdır (packBPanel gibi)
			}
			for col := 0; col < nn; col++ {
				dequantBlocks(dt, row, w[(j0+col)*rowBytes:(j0+col+1)*rowBytes])
				for k, v := range row {
					pan[k*nr+col] = v
				}
			}
			clear(part)
			for p0 := 0; p0 < K; p0 += kcBlock {
				kb := min(kcBlock, K-p0)
				blk := ap[p0*mPad:]
				for ip := 0; ip*mr < M; ip++ {
					microKernel(kb, blk[ip*kb*mr:(ip+1)*kb*mr], pan[p0*nr:(p0+kb)*nr], part[ip*mr*nr:], nr, min(mr, M-ip*mr), nn)
				}
			}
			for m := 0; m < M; m++ {
				out := c[m*N+j0 : m*N+j0+nn]
				for j, v := range part[m*nr : m*nr+nn] {
					out[j] += v
				}
			}
		}
	})
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

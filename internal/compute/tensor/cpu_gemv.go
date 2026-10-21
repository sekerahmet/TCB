package tensor

// gemvMinWork tek satırlık çarpımın gemv yoluna gitmesi için en az çarpım sayısıdır (N·K).
// Küçük çarpımlarda (576 × 1536 gibi katman ağırlıkları) paketleme ucuzdur ve mikro çekirdek
// daha hızlıdır (ölçüldü); büyük N'de (dil modelinin
// çıkış katmanı, 576 × 49 152) B'yi her çağrıda paketlemek işi kendisinden pahalı yapar.
const gemvMinWork = 1 << 22

// gemv tek satırlık çarpımdır (M = 1; çıkarımda bir parça): c[b, n] = Σ_k a[b, k]·op(B)[b, k, n].
// B paketlenmez, ağırlık bir kez okunur; iş B'nin sütunları (çıktı elemanları) üzerinden
// çekirdeklere bölünür. transB'de (B [N, K] satır öncelikli: paylaşımlı gömmenin devriği)
// her çıktı bitişik bir satırla nokta çarpımdır; değilse (B [K, N]) sütun dilimi boyunca
// satırlar birikir. Her çıktı tek bir parçada, sabit sırayla hesaplanır: sonuç parça
// sınırlarından bağımsızdır (belirlenimci).
func gemv(batch, N, K int, a, b []float32, transB bool, c []float32) {
	if transB {
		parallelFor(batch*N, K, func(lo, hi int) {
			for t := lo; t < hi; t++ {
				bi, n := t/N, t%N
				c[bi*N+n] = dot32(a[bi*K:(bi+1)*K], b[bi*N*K+n*K:bi*N*K+(n+1)*K])
			}
		})
		return
	}
	// Sütun dilimleri: her parça [lo, hi) çıktısını sıfırdan biriktirir (B'nin satır parçaları
	// bitişik okunur).
	const chunk = 256
	chunks := (N + chunk - 1) / chunk
	parallelFor(batch*chunks, K*chunk, func(lo, hi int) {
		for t := lo; t < hi; t++ {
			bi, j0 := t/chunks, (t%chunks)*chunk
			j1 := min(j0+chunk, N)
			out := c[bi*N+j0 : bi*N+j1]
			clear(out)
			for k, s := range a[bi*K : (bi+1)*K] {
				row := b[bi*K*N+k*N+j0 : bi*K*N+k*N+j1]
				for j, v := range row {
					out[j] += s * v
				}
			}
		}
	})
}

// dot32 iki eşit uzunluklu dilimin nokta çarpımıdır; sekiz ayrı toplayıcı (bağımlılık zinciri
// kısalır), dilimler sekizer kaydırılır (sınır denetimi döngü dışına çıkar), sonda sabit sırayla
// toplanır.
func dot32(x, y []float32) float32 {
	y = y[:len(x)]
	var s0, s1, s2, s3, s4, s5, s6, s7 float32
	for len(x) >= 8 && len(y) >= 8 {
		xx, yy := x[:8:8], y[:8:8]
		s0 += xx[0] * yy[0]
		s1 += xx[1] * yy[1]
		s2 += xx[2] * yy[2]
		s3 += xx[3] * yy[3]
		s4 += xx[4] * yy[4]
		s5 += xx[5] * yy[5]
		s6 += xx[6] * yy[6]
		s7 += xx[7] * yy[7]
		x, y = x[8:], y[8:]
	}
	for i := range x {
		s0 += x[i] * y[i]
	}
	return ((s0 + s1) + (s2 + s3)) + ((s4 + s5) + (s6 + s7))
}

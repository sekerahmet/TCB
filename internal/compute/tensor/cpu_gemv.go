package tensor

// gemvMinWork tek satırlık çarpımın gemv yoluna gitmesi için en az çarpım sayısıdır (N·K);
// altındaki çarpımlar paketli mikro çekirdeğe gider (çağrı ve iş parçacığı başlatma yükü
// baskın kalır).
const gemvMinWork = 1 << 12

// gemvStripN gemv'nin bir parçasındaki sütun sayısıdır (gemvStrip, 64 sütunluk şerit).
const gemvStripN = 64

// gemv tek satırlık çarpımdır (M = 1; çıkarımda bir parça): c[b, n] += Σ_k a[b, k]·op(B)[b, k, n]
// (gemm gibi ekler: geri yayılım gradyanı biriktirir).
// B paketlenmez, ağırlık bir kez okunur; iş çıktı sütunları üzerinden çekirdeklere bölünür.
// transB'de (B [N, K] satır öncelikli: paylaşımlı gömmenin devriği) her çıktı bitişik bir
// satırla nokta çarpımdır (dotF32). Değilse (B [K, N]: doğrusal katman x @ W) 64 sütunluk
// şeritler biriktiricilerde tutulur, her k adımında W satırının şerit parçası okunur
// (gemvStrip; AVX2/FMA'da assembly). Her çıktı tek bir parçada, k sırasıyla hesaplanır:
// sonuç parça sınırlarından bağımsızdır (belirlenimci).
func gemv(batch, N, K int, a, b []float32, transB bool, c []float32) {
	if transB {
		parallelFor(batch*N, K, func(lo, hi int) {
			for t := lo; t < hi; t++ {
				bi, n := t/N, t%N
				c[bi*N+n] += dotF32(a[bi*K:(bi+1)*K], b[bi*N*K+n*K:bi*N*K+(n+1)*K])
			}
		})
		return
	}
	strips := (N + gemvStripN - 1) / gemvStripN
	parallelFor(batch*strips, K*gemvStripN, func(lo, hi int) {
		var tail [gemvStripN]float32
		for t := lo; t < hi; t++ {
			bi, j0 := t/strips, (t%strips)*gemvStripN
			ab, bb := a[bi*K:(bi+1)*K], b[bi*K*N:(bi+1)*K*N]
			if j0+gemvStripN <= N {
				gemvStrip(ab, bb[j0:], N, c[bi*N+j0:bi*N+j0+gemvStripN])
				continue
			}
			// Son şerit kısa: geçerli sütunlar Go ile (şerit çekirdeği 64 sütun okur).
			out := tail[:N-j0]
			clear(out)
			for k, s := range ab {
				row := bb[k*N+j0 : k*N+N]
				for j, v := range row {
					out[j] += s * v
				}
			}
			for j, v := range out {
				c[bi*N+j0+j] += v
			}
		}
	})
}

// gemvStripGo gemvStrip'in Go eşidir: y[j] += Σ_k a[k]·b[k*ldb + j] (j < len(y)); şerit önce
// sıfırdan biriktirilir, sonra eklenir (assembly ile aynı sıra).
func gemvStripGo(a, b []float32, ldb int, y []float32) {
	var acc [gemvStripN]float32
	t := acc[:len(y)]
	for k, s := range a {
		row := b[k*ldb : k*ldb+len(y)]
		for j, v := range row {
			t[j] += s * v
		}
	}
	for j, v := range t {
		y[j] += v
	}
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

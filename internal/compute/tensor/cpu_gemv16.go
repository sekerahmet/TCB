package tensor

// gemv16 16 bitlik ağırlıkla tek satırlık çarpımdır: gemv'nin eşi, B float32'ye açılmaz, çekirdek
// 16 bitten okur (bayt yarıya iner, tampon gerekmez). Açma kayıpsızdır, her çıktı gemv'deki
// sırayla toplanır: sonuç "aç + gemv" ile bit düzeyinde aynıdır. Çağıran, direct16 ile bu yolun
// işlemcide o sonucu verdiğini denetler.
func gemv16(batch, N, K int, a []float32, b []uint16, dt DType, transB bool, c []float32) {
	if transB {
		parallelFor(batch*N, K, func(lo, hi int) {
			row := make([]float32, K)
			for t := lo; t < hi; t++ {
				bi, n := t/N, t%N
				c[bi*N+n] += dot16(a[bi*K:(bi+1)*K], b[bi*N*K+n*K:bi*N*K+(n+1)*K], dt, row)
			}
		})
		return
	}
	strips := (N + gemvStripN - 1) / gemvStripN
	parallelFor(batch*strips, K*gemvStripN, func(lo, hi int) {
		for t := lo; t < hi; t++ {
			bi, j0 := t/strips, (t%strips)*gemvStripN
			ab, bb := a[bi*K:(bi+1)*K], b[bi*K*N:(bi+1)*K*N]
			if j0+gemvStripN <= N {
				gemvStrip16(ab, bb[j0:], N, c[bi*N+j0:bi*N+j0+gemvStripN], dt)
				continue
			}
			// Son şerit kısa: geçerli sütunlar Go ile (şerit çekirdeği 64 sütun okur).
			gemvStripGo16(ab, bb[j0:], N, c[bi*N+j0:bi*N+N], dt)
		}
	})
}

// gemvStripGo16 gemvStripGo'nun 16 bitlik eşidir: her k adımında satırın şerit parçası açılır,
// sonra aynı sırayla biriktirilir.
func gemvStripGo16(a []float32, b []uint16, ldb int, y []float32, dt DType) {
	var acc, wide [gemvStripN]float32
	t, w := acc[:len(y)], wide[:len(y)]
	for k, s := range a {
		widenSpan(w, b[k*ldb:k*ldb+len(y)], dt)
		for j, v := range w {
			t[j] += s * v
		}
	}
	for j, v := range t {
		y[j] += v
	}
}

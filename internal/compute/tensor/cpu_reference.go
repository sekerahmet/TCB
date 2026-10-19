package tensor

// Assembly çekirdeklerinin (amd64'te AVX2/FMA'lı işlemcide: cpu_micro_amd64.s,
// cpu_vector_amd64.s) taşınabilir Go başvuruları. Yapı etiketi yoktur: AVX2'siz işlemciler ve
// öbür mimariler bunları çekirdek olarak kullanır (cpu_vector_amd64.go, cpu_vector_other.go),
// testler de assembly sonucunu aynı programda bunlarla karşılaştırır (cpu_reference_test.go).
// Öteki başvurular (geluForwardGo, geluBackwardGo, ceRowGo, ceBackwardGo, lnForwardBlockGo,
// lnBackwardBlockGo) cpu_backend.go ve cpu_limit.go'dadır.

// microAGo C[0:mm, 0:nn] += A·Bp; A(r, k) = a[r*rs + k*ks], bp kb×nr paketli, c satır
// adımı ldc: mikroA'nın (v3'te sgemm6x16) basit döngülerle başvurusu. Her eleman için
// toplam sıfırdan k sırasıyla biriktirilip c'ye eklenir (çekirdeğin işlem sırası).
func microAGo(kb int, a []float32, rs, ks int, bp []float32, c []float32, ldc, mm, nn int) {
	for i := 0; i < mm; i++ {
		for j := 0; j < nn; j++ {
			var s float32
			for k := 0; k < kb; k++ {
				s += a[i*rs+k*ks] * bp[k*nr+j]
			}
			c[i*ldc+j] += s
		}
	}
}

// expVecGo dst[i] = e^src[i] (dst ve src aynı dilim olabilir): expVek'in skaler expf32
// döngüsüyle başvurusu.
func expVecGo(dst, src []float32) {
	dst = dst[:len(src)]
	for i, v := range src {
		dst[i] = expf32(v)
	}
}

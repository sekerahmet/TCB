package tensor

// Assembly çekirdeklerinin (GOAMD64=v3: cpu_mikro_amd64.s, cpu_vektor_amd64.s) taşınabilir
// Go başvuruları. Yapı etiketi yoktur: v3 dışındaki derlemeler bunları çekirdek olarak
// kullanır (cpu_mikro_genel.go, cpu_vektor_genel.go), v3 derlemesinde ise yalnızca testler
// assembly sonucunu aynı programda bunlarla karşılaştırır (cekirdek_esi_test.go). Öteki
// başvurular (geluIleriGo, geluGeriGo, ceSatirGo, ceGeriGo, lnIleriBlokGo, lnGeriBlokGo)
// zaten etiketsiz dosyalardadır (cpu_arkauc.go, cpu_sinir.go).

// mikroAGo C[0:mm, 0:nn] += A·Bp; A(r, k) = a[r*rs + k*ks], bp kb×nr paketli, c satır
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

// expVekGo dst[i] = e^src[i] (dst ve src aynı dilim olabilir): expVek'in skaler expf32
// döngüsüyle başvurusu.
func expVecGo(dst, src []float32) {
	dst = dst[:len(src)]
	for i, v := range src {
		dst[i] = expf32(v)
	}
}

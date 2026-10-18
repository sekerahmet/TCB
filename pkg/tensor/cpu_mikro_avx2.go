//go:build amd64.v3

package tensor

// AVX2/FMA mikro çekirdeği (GOAMD64=v3). 6×16'lık C bloğu 12 ymm yazmacında
// biriktirilir; her k adımında B'nin 16 elemanı iki yazmaca yüklenir, A'nın 6
// elemanı sırayla yayınlanır ve 12 kaynaştırılmış çarpma-toplama yapılır
// (16 yazmacın 15'i kullanılır, yığına taşma yoktur). Çekirdek A'yı adımlarla
// okuyabildiğinden tam A panelleri paketlenmeden doğrudan kaynaktan okunur
// (küçük N'de paketleme maliyeti çarpımla aynı mertebedeydi); yalnızca mr'den
// kısa kenar panelleri sıfır dolgulu paketlenir.
const (
	mr = 6  // mikro çekirdek satır sayısı
	nr = 16 // mikro çekirdek sütun sayısı
	// dogrudanA tam A panellerinin paketlenmeden okunması.
	dogrudanA = true
)

// mikroCekirdekAdi hız raporları ve testler için çekirdeğin adı.
const mikroCekirdekAdi = "avx2-6x16"

// sgemm6x16 C[0:6, 0:16] += A·Bp; A(r, k) = a[r*rs + k*ks], bp kb×16 paketli,
// c satır adımı ldc (elemanlar). Assembly: cpu_mikro_amd64.s.
//
//go:noescape
func sgemm6x16(kb int, a *float32, rs, ks int, bp, c *float32, ldc int)

// mikroA C[0:mm, 0:nn] += A·Bp; A(r, k) = a[r*rs + k*ks] (r < mr, k < kb).
// Tam bloklar doğrudan C'ye yazılır; kenar blokları geçici bir 6×16 tampona
// hesaplanıp geçerli kısmı eklenir. A'nın mr satırının tamamı okunabilir
// olmalıdır (kenar panelleri paketlenip sıfırla doldurulur).
func mikroA(kb int, a []float32, rs, ks int, bp []float32, c []float32, ldc, mm, nn int) {
	if kb <= 0 || mm <= 0 || nn <= 0 {
		return
	}
	_ = a[(mr-1)*rs+(kb-1)*ks]
	_ = bp[kb*nr-1]
	if mm == mr && nn == nr {
		_ = c[(mr-1)*ldc+nr-1]
		sgemm6x16(kb, &a[0], rs, ks, &bp[0], &c[0], ldc)
		return
	}
	var t [mr * nr]float32
	sgemm6x16(kb, &a[0], rs, ks, &bp[0], &t[0], nr)
	for i := 0; i < mm; i++ {
		row := c[i*ldc : i*ldc+nn]
		for j := range row {
			row[j] += t[i*nr+j]
		}
	}
}

// microKernel C[0:mm, 0:nn] += Ap·Bp (Ap: kb×mr, Bp: kb×nr paketli).
func microKernel(kb int, ap, bp []float32, c []float32, ldc, mm, nn int) {
	mikroA(kb, ap, 1, mr, bp, c, ldc, mm, nn)
}

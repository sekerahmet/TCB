//go:build amd64.v3

package tensor

// AVX2/FMA mikro çekirdeği (GOAMD64=v3). 6×16'lık C bloğu 12 ymm yazmacında
// biriktirilir; her k adımında B'nin 16 elemanı iki yazmaca yüklenir, A'nın 6
// elemanı sırayla yayınlanır ve 12 kaynaştırılmış çarpma-toplama yapılır
// (16 yazmacın 15'i kullanılır, yığına taşma yoktur).
const (
	mr = 6  // mikro çekirdek satır sayısı
	nr = 16 // mikro çekirdek sütun sayısı
)

// mikroCekirdekAdi hız raporları ve testler için çekirdeğin adı.
const mikroCekirdekAdi = "avx2-6x16"

// sgemm6x16 C[0:6, 0:16] += Ap·Bp; ap kb×6, bp kb×16 paketli, c satır adımı ldc
// (eleman). Assembly: cpu_mikro_amd64.s.
//
//go:noescape
func sgemm6x16(kb int, ap, bp, c *float32, ldc int)

// microKernel C[0:mm, 0:nn] += Ap·Bp (Ap: kb×mr, Bp: kb×nr paketli). Tam
// bloklar doğrudan C'ye yazılır; kenar blokları geçici bir 6×16 tampona
// hesaplanıp geçerli kısmı eklenir (paketler sıfırla doldurulmuş olduğundan
// fazladan satır/sütunlar sıfırdır).
func microKernel(kb int, ap, bp []float32, c []float32, ldc, mm, nn int) {
	if kb <= 0 || mm <= 0 || nn <= 0 {
		return
	}
	_ = ap[kb*mr-1]
	_ = bp[kb*nr-1]
	if mm == mr && nn == nr {
		_ = c[(mr-1)*ldc+nr-1]
		sgemm6x16(kb, &ap[0], &bp[0], &c[0], ldc)
		return
	}
	var t [mr * nr]float32
	sgemm6x16(kb, &ap[0], &bp[0], &t[0], nr)
	for i := 0; i < mm; i++ {
		row := c[i*ldc : i*ldc+nn]
		for j := range row {
			row[j] += t[i*nr+j]
		}
	}
}

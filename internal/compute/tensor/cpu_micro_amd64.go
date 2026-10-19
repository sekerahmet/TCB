package tensor

// amd64 mikro çekirdeği: AVX2/FMA'lı işlemcide assembly (hasAVX2), öbürlerinde aynı 6×16
// düzenli Go eşi (micro6x16Go). AVX2 çekirdeğinde 6×16'lık C bloğu 12 ymm yazmacında
// biriktirilir; her k adımında B'nin 16 elemanı iki yazmaca yüklenir, A'nın 6
// elemanı sırayla yayınlanır ve 12 kaynaştırılmış çarpma-toplama yapılır
// (16 yazmacın 15'i kullanılır, yığına taşma yoktur). Çekirdek A'yı adımlarla
// okuyabildiğinden tam A panelleri paketlenmeden doğrudan kaynaktan okunur
// (küçük N'de paketleme maliyeti çarpımla aynı mertebedeydi); yalnızca mr'den
// kısa kenar panelleri sıfır dolgulu paketlenir.
const (
	mr = 6  // mikro çekirdek satır sayısı
	nr = 16 // mikro çekirdek sütun sayısı
	// directA tam A panellerinin paketlenmeden okunması.
	directA = true
)

// microKernelName hız raporları ve testler için çekirdeğin adı.
func microKernelName() string {
	if hasAVX2 {
		return "avx2-6x16"
	}
	return "go-6x16"
}

// sgemm6x16 C[0:6, 0:16] += A·Bp; A(r, k) = a[r*rs + k*ks], bp kb×16 paketli,
// c satır adımı ldc (elemanlar). Assembly: cpu_micro_amd64.s.
//
//go:noescape
func sgemm6x16(kb int, a *float32, rs, ks int, bp *float32, ldb int, c *float32, ldc int)

// microA C[0:mm, 0:nn] += A·Bp; A(r, k) = a[r*rs + k*ks] (r < mr, k < kb).
// Tam bloklar doğrudan C'ye yazılır; kenar blokları geçici bir 6×16 tampona
// hesaplanıp geçerli kısmı eklenir. A'nın mr satırının tamamı okunabilir
// olmalıdır (kenar panelleri paketlenip sıfırla doldurulur).
func microA(kb int, a []float32, rs, ks int, bp []float32, c []float32, ldc, mm, nn int) {
	microAB(kb, a, rs, ks, bp, nr, c, ldc, mm, nn)
}

// directB tam B panellerinin paketlenmeden (satır öncelikli B'den ldb adımıyla) okunabilmesi.
const directB = true

// microAB microA'nın B adımlı biçimi: B(k, j) = bp[k*ldb + j] (j < nr). Paketli B için
// ldb = nr; paketlenmemiş satır öncelikli B için ldb = N (yalnızca tam paneller: 16 sütunun
// hepsi okunabilir olmalıdır).
func microAB(kb int, a []float32, rs, ks int, bp []float32, ldb int, c []float32, ldc, mm, nn int) {
	if kb <= 0 || mm <= 0 || nn <= 0 {
		return
	}
	_ = a[(mr-1)*rs+(kb-1)*ks]
	_ = bp[(kb-1)*ldb+nr-1]
	if mm == mr && nn == nr {
		_ = c[(mr-1)*ldc+nr-1]
		if hasAVX2 {
			sgemm6x16(kb, &a[0], rs, ks, &bp[0], ldb, &c[0], ldc)
		} else {
			micro6x16Go(kb, a, rs, ks, bp, ldb, c, ldc)
		}
		return
	}
	var t [mr * nr]float32
	if hasAVX2 {
		sgemm6x16(kb, &a[0], rs, ks, &bp[0], ldb, &t[0], nr)
	} else {
		micro6x16Go(kb, a, rs, ks, bp, ldb, t[:], nr)
	}
	for i := 0; i < mm; i++ {
		row := c[i*ldc : i*ldc+nn]
		for j := range row {
			row[j] += t[i*nr+j]
		}
	}
}

// microKernel C[0:mm, 0:nn] += Ap·Bp (Ap: kb×mr, Bp: kb×nr paketli).
func microKernel(kb int, ap, bp []float32, c []float32, ldc, mm, nn int) {
	microA(kb, ap, 1, mr, bp, c, ldc, mm, nn)
}

// micro6x16Go sgemm6x16'nın Go eşi (AVX2'siz amd64 işlemciler): C[0:6, 0:16] += A·B. Blok
// 2×4'lük 12 alt blokta hesaplanır; her alt blok 8 biriktiriciyle yazmaçlarda kalır ve her k
// adımında 6 yükleme ile 8 çarpma-toplama yapar (taşınabilir 4×2 çekirdeğiyle aynı oran).
func micro6x16Go(kb int, a []float32, rs, ks int, bp []float32, ldb int, c []float32, ldc int) {
	bp = bp[:(kb-1)*ldb+nr]
	for i := 0; i < mr; i += 2 {
		a0, a1 := a[i*rs:], a[(i+1)*rs:]
		for j := 0; j < nr; j += 4 {
			var c00, c01, c02, c03, c10, c11, c12, c13 float32
			for k, o := 0, 0; k < kb; k, o = k+1, o+ks {
				b := bp[k*ldb+j : k*ldb+j+4 : k*ldb+j+4]
				x0, x1 := a0[o], a1[o]
				c00 += x0 * b[0]
				c01 += x0 * b[1]
				c02 += x0 * b[2]
				c03 += x0 * b[3]
				c10 += x1 * b[0]
				c11 += x1 * b[1]
				c12 += x1 * b[2]
				c13 += x1 * b[3]
			}
			r := c[i*ldc+j : i*ldc+j+4 : i*ldc+j+4]
			r[0] += c00
			r[1] += c01
			r[2] += c02
			r[3] += c03
			r = c[(i+1)*ldc+j : (i+1)*ldc+j+4 : (i+1)*ldc+j+4]
			r[0] += c10
			r[1] += c11
			r[2] += c12
			r[3] += c13
		}
	}
}

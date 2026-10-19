//go:build !amd64

package tensor

// Taşınabilir mikro çekirdek (amd64 dışındaki mimariler).
const (
	mr = 4 // mikro çekirdek satır sayısı
	nr = 2 // mikro çekirdek sütun sayısı
	// directA tam A panellerinin paketlenmeden okunması (bu çekirdekte hayır).
	directA = false
)

// microA C[0:mm, 0:nn] += A·Bp; A(r, k) = a[r*rs + k*ks]. Bu derlemede gemm
// tarafından kullanılmaz (dogrudanA = false); arayüz eşliği ve testler için
// başvurunun kendisidir (microAGo, cpu_reference.go).
func microA(kb int, a []float32, rs, ks int, bp []float32, c []float32, ldc, mm, nn int) {
	microAGo(kb, a, rs, ks, bp, c, ldc, mm, nn)
}

// microKernelName hız raporları ve testler için çekirdeğin adı.
func microKernelName() string { return "go-4x2" }

// microKernel C[0:mm, 0:nn] += Ap·Bp (Ap: kb×mr, Bp: kb×nr paketli).
// 4×2 boyutu, 8 biriktirici ve yükleme yazmaçlarının yazmaç dosyasına
// taşmadan sığması için seçilmiştir (daha büyük bloklar yığına taşma
// nedeniyle ölçümlerde daha yavaştı).
func microKernel(kb int, ap, bp []float32, c []float32, ldc, mm, nn int) {
	var c00, c01, c10, c11, c20, c21, c30, c31 float32
	ap = ap[:kb*mr]
	bp = bp[:kb*nr]
	for len(ap) >= 4 && len(bp) >= 2 {
		b0, b1 := bp[0], bp[1]
		a0 := ap[0]
		c00 += a0 * b0
		c01 += a0 * b1
		a1 := ap[1]
		c10 += a1 * b0
		c11 += a1 * b1
		a2 := ap[2]
		c20 += a2 * b0
		c21 += a2 * b1
		a3 := ap[3]
		c30 += a3 * b0
		c31 += a3 * b1
		ap = ap[4:]
		bp = bp[2:]
	}
	if mm == mr && nn == nr {
		r := c[0:2:2]
		r[0] += c00
		r[1] += c01
		r = c[ldc : ldc+2 : ldc+2]
		r[0] += c10
		r[1] += c11
		r = c[2*ldc : 2*ldc+2 : 2*ldc+2]
		r[0] += c20
		r[1] += c21
		r = c[3*ldc : 3*ldc+2 : 3*ldc+2]
		r[0] += c30
		r[1] += c31
		return
	}
	t := [mr][nr]float32{{c00, c01}, {c10, c11}, {c20, c21}, {c30, c31}}
	for i := 0; i < mm; i++ {
		for j := 0; j < nn; j++ {
			c[i*ldc+j] += t[i][j]
		}
	}
}

//go:build amd64.v3

package tensor

import "math"

// AVX2/FMA ile 8'li üstel (GOAMD64=v3). Algoritma expf32 ile aynıdır (aralık
// indirgemesi ve aynı polinom); ölçekleme iki çarpanla yapıldığı için normal
// altı sonuçlar ve taşma kütüphane işlevine düşmeden doğru çıkar.

// expSabit çekirdeğin sabit tablosu: her değer 8 kez (bir ymm) yinelenir.
var expSabit = func() (t [13 * 8]float32) {
	degerler := [...]float32{
		-104, 89, log2e32, ln2Yuksek, ln2Dusuk,
		1.9875691500e-4, 1.3981999507e-3, 8.3334519073e-3, 4.1665795894e-2, 1.6666665459e-1, 5.0000001201e-1,
		1, math.Float32frombits(127),
	}
	for i, v := range degerler {
		for j := 0; j < 8; j++ {
			t[i*8+j] = v
		}
	}
	return
}()

//go:noescape
func expVekCekirdek(dst, src *float32, n int, sabit *float32)

// expVek dst[i] = e^src[i] (dst ve src aynı dilim olabilir). 8'in katı olmayan kuyruk
// da aynı çekirdekten (sıfır dolgulu 8'lik blokla) geçer: skaler expf32 FMA birleştirmesi
// yüzünden çekirdekten 1 ulp farklı olabildiği için sonuç elemanın konumuna bağlı
// olmasın.
func expVek(dst, src []float32) {
	n := len(src)
	dst = dst[:n]
	m := n &^ 7
	if m > 0 {
		expVekCekirdek(&dst[0], &src[0], m, &expSabit[0])
	}
	if k := n - m; k > 0 {
		var blok [8]float32
		copy(blok[:], src[m:])
		expVekCekirdek(&blok[0], &blok[0], 8, &expSabit[0])
		copy(dst[m:], blok[:k])
	}
}

// vekMaske kuyruk maskeleri: &vekMaske[8-k]'dan okunan 8 değerin ilk k'sı -1'dir
// (k < 8 elemanlık kuyruk, maskeli yükleme ve saklamayla işlenir).
var vekMaske = [16]int32{-1, -1, -1, -1, -1, -1, -1, -1}

//go:noescape
func lnIleriCekirdek(y, x, gam, bet, mean, rstd *float32, satir, boy int, dD, eps float64)

//go:noescape
func lnGeriCekirdek(xg *float32, pg, pb *float64, x, gr, gam, mean, rstd *float32, satir, boy int, dD float64)

// ilk32 dilimin ilk elemanının adresi; boş dilim için nil (çekirdeklerde "yok").
func ilk32(s []float32) *float32 {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

func ilk64(s []float64) *float64 {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

// lnIleriBlok katman normunun (mean nil ise RMS normunun) ileri yönü; tanım ve
// taşınabilir eşi lnIleriBlokGo'dadır, sonuç onunla bit düzeyinde aynıdır.
func lnIleriBlok(y, x, g, b, mean, rstd []float32, D int, eps float32) {
	R := lnDenetle(D, x, y, g, b, mean, rstd, nil, nil, nil)
	if R == 0 {
		return
	}
	lnIleriCekirdek(&y[0], &x[0], ilk32(g), ilk32(b), ilk32(mean), &rstd[0], R, D, float64(D), float64(eps))
}

// lnGeriBlok katman normunun (mean nil ise RMS normunun) geri yönü; tanım ve
// taşınabilir eşi lnGeriBlokGo'dadır, sonuç onunla bit düzeyinde aynıdır.
func lnGeriBlok(xg []float32, pg, pb []float64, x, gr, gam, mean, rstd []float32, D int) {
	R := lnDenetle(D, x, gr, gam, nil, mean, rstd, xg, pg, pb)
	if R == 0 {
		return
	}
	lnGeriCekirdek(ilk32(xg), ilk64(pg), ilk64(pb), &x[0], &gr[0], ilk32(gam), ilk32(mean), &rstd[0], R, D, float64(D))
}

//go:build amd64.v3

package tensor

import "math"

// AVX2/FMA ile 8'li üstel (GOAMD64=v3). Algoritma expf32 ile aynıdır (aralık
// indirgemesi ve aynı polinom); ölçekleme iki çarpanla yapıldığı için normal
// altı sonuçlar ve taşma kütüphane işlevine düşmeden doğru çıkar.

// expConst çekirdeğin sabit tablosu: her değer 8 kez (bir ymm) yinelenir.
var expConst = func() (t [13 * 8]float32) {
	values := [...]float32{
		-104, 89, log2e32, ln2Hi, ln2Lo,
		1.9875691500e-4, 1.3981999507e-3, 8.3334519073e-3, 4.1665795894e-2, 1.6666665459e-1, 5.0000001201e-1,
		1, math.Float32frombits(127),
	}
	for i, v := range values {
		for j := 0; j < 8; j++ {
			t[i*8+j] = v
		}
	}
	return
}()

//go:noescape
func expVecKernel(dst, src *float32, n int, constant *float32)

// expVec dst[i] = e^src[i] (dst ve src aynı dilim olabilir). 8'in katı olmayan kuyruk
// da aynı çekirdekten (sıfır dolgulu 8'lik blokla) geçer: skaler expf32 FMA birleştirmesi
// yüzünden çekirdekten 1 ulp farklı olabildiği için sonuç elemanın konumuna bağlı
// olmasın.
func expVec(dst, src []float32) {
	n := len(src)
	dst = dst[:n]
	m := n &^ 7
	if m > 0 {
		expVecKernel(&dst[0], &src[0], m, &expConst[0])
	}
	if k := n - m; k > 0 {
		var blk [8]float32
		copy(blk[:], src[m:])
		expVecKernel(&blk[0], &blk[0], 8, &expConst[0])
		copy(dst[m:], blk[:k])
	}
}

// vecMask kuyruk maskeleri: &vekMaske[8-k]'dan okunan 8 değerin ilk k'sı -1'dir
// (k < 8 elemanlık kuyruk, maskeli yükleme ve saklamayla işlenir).
var vecMask = [16]int32{-1, -1, -1, -1, -1, -1, -1, -1}

// geluConst GELU çekirdeklerinin sabitleri (her biri 8 kez): a, −2c, 1, c, 3a.
var geluConst = func() (t [5 * 8]float32) {
	for i, v := range [...]float32{geluA, geluM2C, 1, geluC, gelu3A} {
		for j := 0; j < 8; j++ {
			t[i*8+j] = v
		}
	}
	return
}()

//go:noescape
func geluForwardKernel(y, x *float32, n int)

//go:noescape
func geluBackwardKernel(gx, gr, x *float32, n int)

// geluForwardVec y = GELU(x); tanım ve taşınabilir eşi geluIleriGo'dadır. Üstel
// vektörel olduğundan sonuç Go eşinden en çok birkaç ulp farklı olabilir.
func geluForwardVec(y, x []float32) {
	if len(x) == 0 {
		return
	}
	_ = y[len(x)-1]
	geluForwardKernel(&y[0], &x[0], len(x))
}

// geluBackwardVec gx += g·GELU'(x); tanım ve taşınabilir eşi geluGeriGo'dadır.
func geluBackwardVec(gx, g, x []float32) {
	if len(gx) == 0 {
		return
	}
	_, _ = g[len(gx)-1], x[len(gx)-1]
	geluBackwardKernel(&gx[0], &g[0], &x[0], len(gx))
}

//go:noescape
func ceRowKernel(row *float32, n int) (m float32, s float64)

//go:noescape
func ceBackwardKernel(target, row *float32, n int, l, k float32)

// ceRow çapraz entropinin satır işi: en büyük m ve Σ e^{x−m}; tanım ve
// taşınabilir eşi ceSatirGo'dadır (üstel 1-2 ulp farklı olabilir).
func ceRow(row []float32) (float32, float64) {
	if len(row) == 0 {
		return float32(math.Inf(-1)), 0
	}
	return ceRowKernel(&row[0], len(row))
}

// ceBackward hedef += k·e^{x−l}; tanım ve taşınabilir eşi ceGeriGo'dadır.
func ceBackward(target, row []float32, l, k float32) {
	if len(row) == 0 {
		return
	}
	_ = target[len(row)-1]
	ceBackwardKernel(&target[0], &row[0], len(row), l, k)
}

//go:noescape
func lnForwardKernel(y, x, gammaPtr, betaPtr, mean, rstd *float32, row, size int, dD, eps float64)

//go:noescape
func lnBackwardKernel(xg *float32, pg, pb *float64, x, gr, gammaPtr, mean, rstd *float32, row, size int, dD float64)

// first32 dilimin ilk elemanının adresi; boş dilim için nil (çekirdeklerde "yok").
func first32(s []float32) *float32 {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

func first64(s []float64) *float64 {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

// lnForwardBlock katman normunun (mean nil ise RMS normunun) ileri yönü; tanım ve
// taşınabilir eşi lnIleriBlokGo'dadır, sonuç onunla bit düzeyinde aynıdır.
func lnForwardBlock(y, x, g, b, mean, rstd []float32, D int, eps float32) {
	R := checkLN(D, x, y, g, b, mean, rstd, nil, nil, nil)
	if R == 0 {
		return
	}
	lnForwardKernel(&y[0], &x[0], first32(g), first32(b), first32(mean), &rstd[0], R, D, float64(D), float64(eps))
}

// lnBackwardBlock katman normunun (mean nil ise RMS normunun) geri yönü; tanım ve
// taşınabilir eşi lnGeriBlokGo'dadır, sonuç onunla bit düzeyinde aynıdır.
func lnBackwardBlock(xg []float32, pg, pb []float64, x, gr, gammaPtr, mean, rstd []float32, D int) {
	R := checkLN(D, x, gr, gammaPtr, nil, mean, rstd, xg, pg, pb)
	if R == 0 {
		return
	}
	lnBackwardKernel(first32(xg), first64(pg), first64(pb), &x[0], &gr[0], first32(gammaPtr), first32(mean), &rstd[0], R, D, float64(D))
}

//go:build !amd64.v3

package tensor

// expVek dst[i] = e^src[i] (dst ve src aynı dilim olabilir). Taşınabilir
// derlemede skaler expf32 döngüsüdür (expVekGo, cpu_basvuru.go); GOAMD64=v3'te
// AVX2 çekirdeği kullanılır.
func expVec(dst, src []float32) { expVecGo(dst, src) }

// ceSatir çapraz entropinin satır işi (bkz. ceSatirGo).
func ceRow(row []float32) (float32, float64) { return ceRowGo(row) }

// ceGeri hedef += k·e^{x−l} (bkz. ceGeriGo).
func ceBackward(target, row []float32, l, k float32) { ceBackwardGo(target, row, l, k) }

// geluIleriVek y = GELU(x) (bkz. geluIleriGo).
func geluForwardVec(y, x []float32) { geluForwardGo(y, x) }

// geluGeriVek gx += g·GELU'(x) (bkz. geluGeriGo).
func geluBackwardVec(gx, g, x []float32) { geluBackwardGo(gx, g, x) }

// lnIleriBlok katman normunun (mean nil ise RMS normunun) ileri yönü (bkz.
// lnIleriBlokGo; GOAMD64=v3'te aynı sonucu veren AVX2 çekirdeği kullanılır).
func lnForwardBlock(y, x, g, b, mean, rstd []float32, D int, eps float32) {
	if checkLN(D, x, y, g, b, mean, rstd, nil, nil, nil) > 0 {
		lnForwardBlockGo(y, x, g, b, mean, rstd, D, eps)
	}
}

// lnGeriBlok katman normunun (mean nil ise RMS normunun) geri yönü (bkz.
// lnGeriBlokGo; GOAMD64=v3'te aynı sonucu veren AVX2 çekirdeği kullanılır).
func lnBackwardBlock(xg []float32, pg, pb []float64, x, gr, gammaPtr, mean, rstd []float32, D int) {
	if checkLN(D, x, gr, gammaPtr, nil, mean, rstd, xg, pg, pb) > 0 {
		lnBackwardBlockGo(xg, pg, pb, x, gr, gammaPtr, mean, rstd, D)
	}
}

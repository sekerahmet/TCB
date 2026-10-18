//go:build !amd64.v3

package tensor

// expVek dst[i] = e^src[i] (dst ve src aynı dilim olabilir). Taşınabilir
// derlemede skaler expf32 döngüsüdür; GOAMD64=v3'te AVX2 çekirdeği kullanılır.
func expVek(dst, src []float32) {
	dst = dst[:len(src)]
	for i, v := range src {
		dst[i] = expf32(v)
	}
}

// geluIleriVek y = GELU(x) (bkz. geluIleriGo).
func geluIleriVek(y, x []float32) { geluIleriGo(y, x) }

// geluGeriVek gx += g·GELU'(x) (bkz. geluGeriGo).
func geluGeriVek(gx, g, x []float32) { geluGeriGo(gx, g, x) }

// lnIleriBlok katman normunun (mean nil ise RMS normunun) ileri yönü (bkz.
// lnIleriBlokGo; GOAMD64=v3'te aynı sonucu veren AVX2 çekirdeği kullanılır).
func lnIleriBlok(y, x, g, b, mean, rstd []float32, D int, eps float32) {
	if lnDenetle(D, x, y, g, b, mean, rstd, nil, nil, nil) > 0 {
		lnIleriBlokGo(y, x, g, b, mean, rstd, D, eps)
	}
}

// lnGeriBlok katman normunun (mean nil ise RMS normunun) geri yönü (bkz.
// lnGeriBlokGo; GOAMD64=v3'te aynı sonucu veren AVX2 çekirdeği kullanılır).
func lnGeriBlok(xg []float32, pg, pb []float64, x, gr, gam, mean, rstd []float32, D int) {
	if lnDenetle(D, x, gr, gam, nil, mean, rstd, xg, pg, pb) > 0 {
		lnGeriBlokGo(xg, pg, pb, x, gr, gam, mean, rstd, D)
	}
}

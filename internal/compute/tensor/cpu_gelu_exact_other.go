//go:build !amd64

package tensor

// geluExactForwardVec y = x·Φ(x) (tam GELU). Taşınabilir derlemede Go eşidir (geluExactForwardGo,
// cpu_gelu_exact.go); amd64'teki AVX2 çekirdeğiyle bit bit aynı sonucu verir.
func geluExactForwardVec(y, x []float32) { geluExactForwardGo(y, x) }

// geluExactBackwardVec gx += g·(Φ(x) + x·φ(x)) (bkz. geluExactBackwardGo).
func geluExactBackwardVec(gx, g, x []float32) { geluExactBackwardGo(gx, g, x) }

package tensor

import "math"

// gelu64Const tam GELU'nun AVX2 çekirdeğinin float64 sabit tablosu; her değer 4 kez (bir ymm)
// yinelenir. Sıra cpu_gelu_exact_amd64.s'deki uzaklıklarla eşleşir (32 bayt × sıra).
//
//lint:ignore U1000 cpu_gelu_exact_amd64.s okur
var gelu64Const = func() (t [geluTabRows * 4]float64) {
	put := func(row int, v float64) {
		for j := 0; j < 4; j++ {
			t[row*4+j] = v
		}
	}
	put(geluTabAbs, math.Float64frombits(1<<63-1))
	put(geluTabInvSqrt2, invSqrt2)
	put(geluTabHalf, 0.5)
	put(geluTabMinusHalf, -0.5)
	for i, c := range ErfSeriesCoef {
		put(geluTabErfSeries+i, c)
	}
	put(geluTabOne, 1)
	put(geluTabThree, 3)
	put(geluTabMinusOne, -1)
	for i, c := range ErfcxChebCoef {
		put(geluTabErfcx+i, c)
	}
	put(geluTabArgMin, geluArgMin)
	put(geluTabLog2e, math.Log2E)
	put(geluTabLn2Hi, geluLn2Hi)
	put(geluTabLn2Lo, geluLn2Lo)
	for i, c := range geluExpCoef {
		put(geluTabExp+i, c)
	}
	put(geluTabBias, math.Float64frombits(1023))
	put(geluTabBig, geluBig)
	put(geluTabSqrtPi, geluSqrtPi)
	put(geluTabInvSqrt2Pi, invSqrt2Pi)
	return
}()

// Tablonun satırları (uzaklık 32·satır; cpu_gelu_exact_amd64.s'nin başındaki liste).
const (
	geluTabAbs = iota
	geluTabInvSqrt2
	geluTabHalf
	geluTabMinusHalf
	geluTabErfSeries  // 7 satır
	geluTabOne        = geluTabErfSeries + 7
	geluTabThree      = geluTabOne + 1
	geluTabMinusOne   = geluTabThree + 1
	geluTabErfcx      = geluTabMinusOne + 1 // 13 satır
	geluTabArgMin     = geluTabErfcx + 13
	geluTabLog2e      = geluTabArgMin + 1
	geluTabLn2Hi      = geluTabLog2e + 1
	geluTabLn2Lo      = geluTabLn2Hi + 1
	geluTabExp        = geluTabLn2Lo + 1 // 11 satır
	geluTabBias       = geluTabExp + 11
	geluTabBig        = geluTabBias + 1
	geluTabSqrtPi     = geluTabBig + 1
	geluTabInvSqrt2Pi = geluTabSqrtPi + 1
	geluTabRows       = geluTabInvSqrt2Pi + 1
)

//go:noescape
func geluExactForwardKernel(y, x *float32, n int)

//go:noescape
func geluExactBackwardKernel(gx, gr, x *float32, n int)

// geluExactForwardVec y = x·Φ(x) (tam GELU); yöntem ve taşınabilir eşi geluExactForwardGo'dadır,
// iki yol bit bit aynıdır.
func geluExactForwardVec(y, x []float32) {
	if !hasAVX2 {
		geluExactForwardGo(y, x)
		return
	}
	if len(x) == 0 {
		return
	}
	_ = y[len(x)-1]
	geluExactForwardKernel(&y[0], &x[0], len(x))
}

// geluExactBackwardVec gx += g·(Φ(x) + x·φ(x)); taşınabilir eşi geluExactBackwardGo.
func geluExactBackwardVec(gx, g, x []float32) {
	if !hasAVX2 {
		geluExactBackwardGo(gx, g, x)
		return
	}
	if len(gx) == 0 {
		return
	}
	_, _ = g[len(gx)-1], x[len(gx)-1]
	geluExactBackwardKernel(&gx[0], &g[0], &x[0], len(gx))
}

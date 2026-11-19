package tensor

import "math"

// Tam (erf tabanlı) GELU'nun CPU çekirdekleri (dil tanımı 15.17, "Hesap yolu"). Φ(x) float64'te,
// GPU'nun iki bölgeli yöntemiyle ve aynı katsayılarla (internal/compute/gpu/ptx_math_erf.go)
// hesaplanır, sonuç float32'ye bir kez yuvarlanır:
//
//   - t = |x|/√2 < 1/2: erf(t) = t·P(t²) (Maclaurin serisinin 7 terimi), Φ = ½ ± ½·erf(t).
//   - t ≥ 1/2: q = ½·e^{−x²/2}·g/(t·√π), g erfcx'in 12. dereceden Chebyshev serisi (z = 1/(1+t),
//     s = 3z−1); Φ = q (x < 0) ya da 1 − q. Kuyruk göreli doğrulukla bulunur (1 + erf farkı yok).
//
// x float32'den geldiği için x² float64'te kesindir; e^{−x²/2} Cody–Waite indirgemeli, 10. dereceden
// Taylor'lu üstelle alınır, |x| > 20'de 0 (Φ float64'te 0 ya da 1, sonuç float32'de 0 ya da x).
// Türev Φ + x·φ aynı Φ'den ve aynı üstelden çıkar. AVX2 çekirdeği (cpu_gelu_exact_amd64.s) aynı
// işlemleri aynı sırayla dört float64 şeritte yapar: iki yol her girdide bit bit aynı sonucu verir.
// Bunun için çarp-topla yalnız math.FMA ile yazılır (Go derleyicisi a*b + c'yi kendiliğinden
// birleştirebilir) ve geri yönde çarpım birikimden ayrı yuvarlanır.
//
// Uçlar: NaN girdi olduğu gibi çıkar; +∞ → +∞, −∞ → +0, türev ±∞'da 1 ve +0. İleri yönde x < −1'de
// sıfıra inen sonuç +0'dır, öbür sıfırlar (±0 girdi, çok küçük |x|) x·Φ'nin işaretini taşır (GPU'nun
// sonucuyla aynı, dil tanımı 15.50). Türev çarpanında
// negatif sonlu x'te sıfıra inen değer −0'dır (x·0'ın işareti); gradyan +0'dan biriktiği için
// kullanıcıya +0 görünür.

// ErfSeriesCoef erf(t) = t·P(t²)'nin katsayıları: 2/√π · (−1)^n / (n!·(2n+1)), n = 0..6. GPU'nun
// PTX yolu da bunları kullanır (tek kaynak).
var ErfSeriesCoef = func() (c [7]float64) {
	fact := 1.0
	for n := range c {
		if n > 0 {
			fact *= float64(n)
		}
		c[n] = 2 / math.Sqrt(math.Pi) * math.Pow(-1, float64(n)) / (fact * float64(2*n+1))
	}
	return
}()

// ErfcxChebCoef g(z) = √π·t·erfcx(t)'nin s = 3z−1 değişkenindeki Chebyshev katsayıları T0..T12
// (z = 1/(1+t)). GPU'nun PTX yolu da bunları kullanır (tek kaynak).
var ErfcxChebCoef = [13]float64{
	0.8373258834099423, -0.22580444569265462, -0.06620334093421042, -0.0013181685758215456,
	0.0017603075264132162, -6.775670679915347e-05, -6.348721004562756e-05,
	1.1908634561373757e-05, 1.214095428332053e-06, -8.872659073277788e-07,
	1.1784307464471269e-07, 2.827312170713883e-08, -1.4913116012946786e-08,
}

const (
	geluArgMin = -200.0                     // −x²/2'nin alt sınırı (|x| ≤ 20)
	geluBig    = 20.0                       // |x| > 20'de e^{−x²/2} = 0
	geluLn2Hi  = 6.93147180369123816490e-01 // ln 2'nin üst parçası (alt 32 biti sıfır)
	geluLn2Lo  = 1.90821492927058770002e-10 // ln 2 − geluLn2Hi
	geluSqrtPi = 1.77245385090551602730     // √π
)

// geluExpCoef e^r'nin Taylor katsayıları 1/k!, k = 0..10 (|r| ≤ ln2/2, kesme hatası ~2e-13).
var geluExpCoef = func() (c [11]float64) {
	f := 1.0
	for k := range c {
		if k > 0 {
			f *= float64(k)
		}
		c[k] = 1 / f
	}
	return
}()

// geluExp64 e^a, a ∈ [geluArgMin, 0] ya da NaN (çekirdekte PHI64'ün üstel kısmı).
func geluExp64(a float64) float64 {
	n := math.RoundToEven(a * math.Log2E)
	r := math.FMA(n, -geluLn2Hi, a)
	r = math.FMA(n, -geluLn2Lo, r)
	p := geluExpCoef[10]
	for k := 9; k >= 0; k-- {
		p = math.FMA(p, r, geluExpCoef[k])
	}
	// n ∈ [−289, 0]; NaN'da n'nin tamsayısı ne olursa olsun p NaN'dır.
	return p * math.Float64frombits(uint64(int64(int32(n))+1023)<<52)
}

// geluPhi64 Φ(x) ve e^{−x²/2} (|x| > 20'de 0); x float32'den gelir (çekirdeğin PHI64'ü).
func geluPhi64(x float64) (phi, ex float64) {
	a := math.Abs(x)
	t := a * invSqrt2
	t2 := t * t
	p := ErfSeriesCoef[6]
	for i := 5; i >= 0; i-- {
		p = math.FMA(p, t2, ErfSeriesCoef[i])
	}
	erf := t * p
	h := -0.5
	if x >= 0 {
		h = 0.5
	}
	near := math.FMA(h, erf, 0.5)

	z := 1 / (t + 1)
	s := math.FMA(z, 3, -1)
	twoS := s + s
	b1, b2 := ErfcxChebCoef[12], 0.0
	for i := 11; i >= 1; i-- {
		b0 := math.FMA(twoS, b1, ErfcxChebCoef[i]) - b2
		b2, b1 = b1, b0
	}
	g := math.FMA(s, b1, ErfcxChebCoef[0]) - b2

	arg := max(-0.5*(x*x), geluArgMin)
	ex = geluExp64(arg)
	if a > geluBig {
		ex = 0
	}
	q := ex * g / (t * geluSqrtPi) * 0.5
	far := q
	if x >= 0 {
		far = 1 - q
	}
	phi = far
	if t < 0.5 {
		phi = near
	}
	return phi, ex
}

// geluSignedZero d = 0 iken x·0: negatif sonlu x'te −0, pozitifte +0; ±∞'da (x·0 NaN) +0.
func geluSignedZero(x, d float64) float64 {
	if d != 0 {
		return d
	}
	z := x * 0
	if z != z {
		return 0
	}
	return z
}

// geluExactForwardGo y = float32(x·Φ(x)); x·Φ NaN iken (−∞·0) +0, NaN girdi olduğu gibi. x < −1'de
// sıfıra inen sonuç +0'dır (orada sıfır Φ'nin alttan taşmasından gelir; GPU Φ = 0'da +0 verir, 15.50);
// öbür sıfırlar (±0 ve çok küçük |x|) x·Φ'nin işaretini taşır.
func geluExactForwardGo(y, x []float32) {
	for i, v := range x {
		if v != v {
			y[i] = v
			continue
		}
		x64 := float64(v)
		phi, _ := geluPhi64(x64)
		r := x64 * phi
		if r != r {
			r = 0
		}
		f := float32(r)
		if f == 0 && v < -1 {
			f = 0
		}
		y[i] = f
	}
}

// geluExactSlopeGo d = float32(Φ(x) + x·φ(x)); φ = 0 iken Φ, sıfırın işareti geluSignedZero'dan,
// NaN girdide girdi.
func geluExactSlopeGo(x float32) float32 {
	if x != x {
		return x
	}
	x64 := float64(x)
	phi, ex := geluPhi64(x64)
	dens := ex * invSqrt2Pi
	d := phi
	if dens != 0 {
		d = math.FMA(x64, dens, phi)
	}
	return float32(geluSignedZero(x64, d))
}

// geluExactBackwardGo gx += g·d; çarpım ayrı yuvarlanır (çekirdekle aynı: çarp, sonra topla).
func geluExactBackwardGo(gx, g, x []float32) {
	for i, v := range x {
		gx[i] += float32(g[i] * geluExactSlopeGo(v))
	}
}

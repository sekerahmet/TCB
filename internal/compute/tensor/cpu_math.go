package tensor

import "math"

// float32 üstel ve hiperbolik tanjant yaklaşımları (softmax, GELU, sigmoid,
// çapraz entropi ve geri yönleri için). float64 kütüphane işlevlerine göre
// belirgin biçimde ucuzdur; hata 1–2 ulp'tir (testler: matematik_test.go).
// Toplamlar çağıran tarafta float64'te tutulur.

const (
	expUpperLimit  = 88.72283  // bunun üstünde e^x float32'de sonsuzdur
	expLowerLimit  = -87.33654 // bunun altında sonuç normal altı (alt taşma bölgesi)
	log2e32        = 1.44269504088896341
	ln2Hi          = 0.693359375    // ln2'nin üst kısmı (tam gösterilebilir)
	ln2Lo          = -2.12194440e-4 // ln2 - ln2Yuksek
	expZero        = -104           // bunun altında e^x float32'de 0'dır
	roundMagic     = 12582912       // 1.5·2^23
	tanhSmall      = 0.625          // bunun altında tanh için polinom
	tanhSaturation = 9              // tanh bunun üstünde float32'de ±1'dir
)

// expf32 e^x. Aralık indirgemesi: x = n·ln2 + r, |r| ≤ ln2/2; e^r için 6.
// dereceden polinom; sonuç 2^n ile üs alanına yazılarak ölçeklenir. Uç
// bölgeler (taşma, normal altı, NaN) float64 kütüphane işleviyle hesaplanır.
func expf32(x float32) float32 {
	if !(x > expLowerLimit && x < expUpperLimit) {
		if x < expZero {
			return 0 // -sonsuz dahil (ör. maskelenmiş dikkat puanları)
		}
		// NaN, +sonsuz, taşma ve normal altı sonuçlar.
		return float32(math.Exp(float64(x)))
	}
	return expAralikta(x)
}

// expAralikta e^x, x ∈ (expAltSinir, expUstSinir) için (denetimsiz).
func expAralikta(x float32) float32 {
	// En yakına yuvarlama dalsız: 1.5·2^23 eklenip çıkarılır.
	fn := (x*log2e32 + roundMagic) - roundMagic
	n := int32(fn)
	r := x - fn*ln2Hi
	r -= fn * ln2Lo
	z := r * r
	p := float32(1.9875691500e-4)
	p = p*r + 1.3981999507e-3
	p = p*r + 8.3334519073e-3
	p = p*r + 4.1665795894e-2
	p = p*r + 1.6666665459e-1
	p = p*r + 5.0000001201e-1
	y := p*z + r + 1
	// 2^n: n ∈ [-126, 128]; n = 128 yalnızca y < 1 iken olur, iki adımda ölçeklenir.
	if n > 127 {
		return y * 2 * math.Float32frombits(uint32(n-1+127)<<23)
	}
	return y * math.Float32frombits(uint32(n+127)<<23)
}

// tanhf32 hiperbolik tanjant. İki yol da hesaplanır ve sonuç bit maskesiyle
// seçilir (rastgele girdilerde dal tahmini kaçırma maliyeti olmasın):
//   - |x| < 0.625: x + x³·P(x²) tek dereceli polinom,
//   - aksi halde 1 - 2/(e^{2|x|}+1) (|x| 9'a kırpılır; tanh(9) float32'de 1'dir).
//
// İşaret sonradan geri konur; NaN girdi NaN döner.
func tanhf32(x float32) float32 {
	bx := math.Float32bits(x)
	sign := bx & 0x80000000
	a := math.Float32frombits(bx &^ 0x80000000)
	// Polinom yolu (a üzerinde).
	z := a * a
	p := float32(-5.70498872745e-3)
	p = p*z + 2.06390887954e-2
	p = p*z - 5.37397155531e-2
	p = p*z + 1.33314422036e-1
	p = p*z - 3.33332819422e-1
	small := a + a*z*p
	// Üstel yol.
	e := expAralikta(2 * min(a, tanhSaturation))
	large := 1 - 2/(e+1)
	var m uint32
	if a < tanhSmall {
		m = ^uint32(0)
	}
	r := math.Float32bits(small)&m | math.Float32bits(large)&^m
	if a != a {
		return x
	}
	return math.Float32frombits(r | sign)
}

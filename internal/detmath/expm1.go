// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_expm1.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// Yöntem: x = k*ln2 + r, |r| <= 0.5*ln2 olacak biçimde k tamsayısı ve r
// bulunur; r yuvarlanırken oluşan hata bir düzeltme terimi c ile giderilir.
// expm1(r), [0, 0.34658] aralığında özel bir rasyonel fonksiyonla (r*r'nin
// beşinci derece çokterimlisi) yaklaşık hesaplanır; sonuç k'ye göre
// 2**k ile ölçeklenir, k'nin değerine göre ayrı dallar kullanılır.
// Özel durumlar: expm1(+Inf) = +Inf, expm1(-Inf) = -1, expm1(NaN) = NaN;
// sonlu bağımsız değişkenlerde yalnız expm1(0) = 0 tamdır.
// Hata analizine göre hata her zaman 1 ulp'den küçüktür.
//
// Sabitlerin onaltılık değerleri esastır. Ondalık değerler, derleyici
// ondalıktan ikiliğe onaltılık değerleri verecek kadar doğru çeviriyorsa
// kullanılabilir.
//

// Expm1, e**x - 1 değerini, yani x'in e tabanlı üstelinin bir eksiğini verir.
// x sıfıra yakınken [Exp](x) - 1'den daha doğrudur.
//
// Özel durumlar:
//
//	Expm1(+Inf) = +Inf
//	Expm1(-Inf) = -1
//	Expm1(NaN) = NaN
//
// Çok büyük değerlerde sonuç -1'e ya da +Inf'e taşar.
func Expm1(x float64) float64 {
	return expm1(x)
}

func expm1(x float64) float64 {
	const (
		Othreshold = 7.09782712893383973096e+02 // 0x40862E42FEFA39EF
		Ln2X56     = 3.88162421113569373274e+01 // 0x4043687a9f1af2b1
		Ln2HalfX3  = 1.03972077083991796413e+00 // 0x3ff0a2b23f3bab73
		Ln2Half    = 3.46573590279972654709e-01 // 0x3fd62e42fefa39ef
		Ln2Hi      = 6.93147180369123816490e-01 // 0x3fe62e42fee00000
		Ln2Lo      = 1.90821492927058770002e-10 // 0x3dea39ef35793c76
		InvLn2     = 1.44269504088896338700e+00 // 0x3ff71547652b82fe
		Tiny       = 1.0 / (1 << 54)            // 2**-54 = 0x3c90000000000000
		// expm1 ile ilgili ölçeklenmiş katsayılar
		Q1 = -3.33333333333331316428e-02 // 0xBFA11111111110F4
		Q2 = 1.58730158725481460165e-03  // 0x3F5A01A019FE5585
		Q3 = -7.93650757867487942473e-05 // 0xBF14CE199EAADBB7
		Q4 = 4.00821782732936239552e-06  // 0x3ED0CFCA86E65239
		Q5 = -2.01099218183624371326e-07 // 0xBE8AFDB76E09C32D
	)

	// özel durumlar
	switch {
	case math.IsInf(x, 1) || math.IsNaN(x):
		return x
	case math.IsInf(x, -1):
		return -1
	}

	absx := x
	sign := false
	if x < 0 {
		absx = -absx
		sign = true
	}

	// çok büyük bağımsız değişkeni ayıkla
	if absx >= Ln2X56 { // |x| >= 56 * ln2 ise
		if sign {
			return -1 // x < -56*ln2 ise -1 döner
		}
		if absx >= Othreshold { // |x| >= 709.78... ise
			return math.Inf(1)
		}
	}

	// bağımsız değişkeni indirge
	var c float64
	var k int
	if absx > Ln2Half { // |x| > 0.5 * ln2 ise
		var hi, lo float64
		if absx < Ln2HalfX3 { // ve |x| < 1.5 * ln2
			if !sign {
				hi = x - Ln2Hi
				lo = Ln2Lo
				k = 1
			} else {
				hi = x + Ln2Hi
				lo = -Ln2Lo
				k = -1
			}
		} else {
			if !sign {
				k = int(float64(InvLn2*x) + 0.5)
			} else {
				k = int(float64(InvLn2*x) - 0.5)
			}
			t := float64(k)
			hi = x - float64(t*Ln2Hi) // t * Ln2Hi burada tamdır
			lo = float64(t * Ln2Lo)
		}
		x = hi - lo
		c = (hi - x) - lo
	} else if absx < Tiny { // |x| < 2**-54 ise x döner
		return x
	} else {
		k = 0
	}

	// x artık ana aralıkta
	hfx := float64(0.5 * x)
	hxs := float64(x * hfx)
	r1 := 1 + float64(hxs*(Q1+float64(hxs*(Q2+float64(hxs*(Q3+float64(hxs*(Q4+float64(hxs*Q5)))))))))
	t := 3 - float64(r1*hfx)
	e := float64(hxs * ((r1 - t) / (6.0 - float64(x*t))))
	if k == 0 {
		return x - (float64(x*e) - hxs) // c sıfırdır
	}
	e = (float64(x*(e-c)) - c)
	e -= hxs
	switch {
	case k == -1:
		return float64(0.5*(x-e)) - 0.5
	case k == 1:
		if x < -0.25 {
			return float64(-2 * (e - (x + 0.5)))
		}
		return 1 + float64(2*(x-e))
	case k <= -2 || k > 56: // exp(x)-1 döndürmek yeter
		y := 1 - (e - x)
		y = math.Float64frombits(math.Float64bits(y) + uint64(k)<<52) // k'yi y'nin üssüne ekle
		return y - 1
	}
	if k < 20 {
		t := math.Float64frombits(0x3ff0000000000000 - (0x20000000000000 >> uint(k))) // t=1-2**-k
		y := t - (e - x)
		y = math.Float64frombits(math.Float64bits(y) + uint64(k)<<52) // k'yi y'nin üssüne ekle
		return y
	}
	t = math.Float64frombits(uint64(0x3ff-k) << 52) // 2**-k
	y := x - (e + t)
	y++
	y = math.Float64frombits(math.Float64bits(y) + uint64(k)<<52) // k'yi y'nin üssüne ekle
	return y
}

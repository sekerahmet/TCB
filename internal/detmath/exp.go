// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

// Exp, e üzeri x'i, yani x'in e tabanlı üstelini verir.
//
// Özel durumlar:
//
//	Exp(+Inf) = +Inf
//	Exp(NaN) = NaN
//
// Çok büyük değerlerde sonuç 0'a ya da +Inf'e taşar.
// Çok küçük değerlerde sonuç 1'e düşer.
func Exp(x float64) float64 {
	return exp(x)
}

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_exp.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 2004 by Sun Microsystems, Inc. All rights reserved.
//
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// Yöntem: x = k*ln2 + r, |r| <= 0.5*ln2 olacak biçimde k tamsayısı ve r
// bulunur (r, doğruluk için hi - lo olarak tutulur). exp(r), [0, 0.34658]
// aralığında özel bir rasyonel fonksiyonla (beşinci derece çokterimli)
// yaklaşık hesaplanır; sonuç 2**k ile ölçeklenir.
// Özel durumlar: exp(+Inf) = +Inf, exp(-Inf) = 0, exp(NaN) = NaN;
// sonlu bağımsız değişkenlerde yalnız exp(0) = 1 tamdır.
// Hata analizine göre hata her zaman 1 ulp'den küçüktür.
//
// Sabitlerin onaltılık değerleri esastır. Ondalık değerler, derleyici
// ondalıktan ikiliğe onaltılık değerleri verecek kadar doğru çeviriyorsa
// kullanılabilir.

func exp(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01
		Ln2Lo = 1.90821492927058770002e-10
		Log2e = 1.44269504088896338700e+00

		Overflow  = 7.09782712893383973096e+02
		Underflow = -7.45133219101941108420e+02
		NearZero  = 1.0 / (1 << 28) // 2**-28
	)

	// özel durumlar
	switch {
	case math.IsNaN(x):
		return x
	case x > Overflow: // x'in +∞ olduğu durumu da karşılar
		return math.Inf(1)
	case x < Underflow: // x'in -∞ olduğu durumu da karşılar
		return 0
	case -NearZero < x && x < NearZero:
		return 1 + x
	}

	// indirge; fazladan duyarlık için r = hi - lo olarak hesaplanır.
	var k int
	switch {
	case x < 0:
		k = int(float64(Log2e*x) - 0.5)
	case x > 0:
		k = int(float64(Log2e*x) + 0.5)
	}
	hi := x - float64(float64(k)*Ln2Hi)
	lo := float64(float64(k) * Ln2Lo)

	// hesapla
	return expmulti(hi, lo, k)
}

// expmulti, r = hi - lo ve |r| ≤ ln(2)/2 olmak üzere e**r × 2**k değerini verir.
func expmulti(hi, lo float64, k int) float64 {
	const (
		P1 = 1.66666666666666657415e-01  /* 0x3FC55555; 0x55555555 */
		P2 = -2.77777777770155933842e-03 /* 0xBF66C16C; 0x16BEBD93 */
		P3 = 6.61375632143793436117e-05  /* 0x3F11566A; 0xAF25DE2C */
		P4 = -1.65339022054652515390e-06 /* 0xBEBBBD41; 0xC5D26BF1 */
		P5 = 4.13813679705723846039e-08  /* 0x3E663769; 0x72BEA4D0 */
	)

	r := hi - lo
	t := float64(r * r)
	c := r - float64(t*(P1+float64(t*(P2+float64(t*(P3+float64(t*(P4+float64(t*P5)))))))))
	y := 1 - ((lo - (float64(r*c))/(2-c)) - hi)
	// TODO: Ldexp'in sınır k değerlerini doğru işlediğinden emin ol
	return math.Ldexp(y, k)
}

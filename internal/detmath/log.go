// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

/*
	Kayan noktalı logaritma.
*/

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_log.c
// and came with this notice. The go code is a simpler
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
// Yöntem: x = 2**k * (1+f), sqrt(2)/2 < 1+f < sqrt(2) olacak biçimde k ve f
// bulunur. s = f/(2+f) alınır; log(1+f) = 2s + s*R yazılır ve R, [0, 0.1716]
// aralığında on dördüncü dereceden bir çokterimliyle yaklaşık hesaplanır.
// Sonuç log(x) = k*Ln2 + log(1+f) olur; Ln2 iki parçaya (Ln2Hi + Ln2Lo)
// bölünür ve |n| < 2000 için n*Ln2Hi tamdır.
// Özel durumlar: x < 0 (-Inf dahil) için NaN, log(+Inf) = +Inf,
// log(0) = -Inf, log(NaN) = NaN.
// Hata analizine göre hata her zaman 1 ulp'den küçüktür.
//
// Sabitlerin onaltılık değerleri esastır. Ondalık değerler, derleyici
// ondalıktan ikiliğe onaltılık değerleri verecek kadar doğru çeviriyorsa
// kullanılabilir.

// Log, x'in doğal logaritmasını verir.
//
// Özel durumlar:
//
//	Log(+Inf) = +Inf
//	Log(0) = -Inf
//	Log(x < 0) = NaN
//	Log(NaN) = NaN
func Log(x float64) float64 {
	return log(x)
}

func log(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01 /* 3fe62e42 fee00000 */
		Ln2Lo = 1.90821492927058770002e-10 /* 3dea39ef 35793c76 */
		L1    = 6.666666666666735130e-01   /* 3FE55555 55555593 */
		L2    = 3.999999999940941908e-01   /* 3FD99999 9997FA04 */
		L3    = 2.857142874366239149e-01   /* 3FD24924 94229359 */
		L4    = 2.222219843214978396e-01   /* 3FCC71C5 1D8E78AF */
		L5    = 1.818357216161805012e-01   /* 3FC74664 96CB03DE */
		L6    = 1.531383769920937332e-01   /* 3FC39A09 D078C69F */
		L7    = 1.479819860511658591e-01   /* 3FC2F112 DF3E5244 */
	)

	// özel durumlar
	switch {
	case math.IsNaN(x) || math.IsInf(x, 1):
		return x
	case x < 0:
		return math.NaN()
	case x == 0:
		return math.Inf(-1)
	}

	// indirge
	f1, ki := math.Frexp(x)
	if f1 < math.Sqrt2/2 {
		f1 = float64(f1 * 2)
		ki--
	}
	f := f1 - 1
	k := float64(ki)

	// hesapla
	s := f / (2 + f)
	s2 := float64(s * s)
	s4 := float64(s2 * s2)
	t1 := float64(s2 * (L1 + float64(s4*(L3+float64(s4*(L5+float64(s4*L7)))))))
	t2 := float64(s4 * (L2 + float64(s4*(L4+float64(s4*L6)))))
	R := t1 + t2
	hfsq := float64(float64(0.5*f) * f)
	return float64(k*Ln2Hi) - ((hfsq - (float64(s*(hfsq+R)) + float64(k*Ln2Lo))) - f)
}

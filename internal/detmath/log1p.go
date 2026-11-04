// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_log1p.c
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
// Yöntem: 1+x = 2**k * (1+f), sqrt(2)/2 < 1+f < sqrt(2) olacak biçimde k ve f
// bulunur. k sıfır değilse f tam gösterilemeyebilir; u = 1+x yuvarlanmış
// değer, c = (1+x)-u kalan olmak üzere log(u) hesaplanır ve c/u düzeltmesi
// geri eklenir (x > 2**53 ise doğrudan log(x) yeter). s = f/(2+f) alınır;
// log(1+f) = 2s + s*R yazılır ve R, [0, 0.1716] aralığında on dördüncü
// dereceden bir çokterimliyle yaklaşık hesaplanır.
// Sonuç log1p(x) = k*ln2 + log1p(f) olur; ln2 iki parçaya bölünür ve
// |n| < 2000 için n*ln2_hi tamdır.
// Özel durumlar: x < -1 (-Inf dahil) için NaN, log1p(+Inf) = +Inf,
// log1p(-1) = -Inf, log1p(NaN) = NaN.
// Hata analizine göre hata her zaman 1 ulp'den küçüktür.
//
// Sabitlerin onaltılık değerleri esastır. Ondalık değerler, derleyici
// ondalıktan ikiliğe onaltılık değerleri verecek kadar doğru çeviriyorsa
// kullanılabilir.

// Log1p, 1 artı x'in doğal logaritmasını verir.
// x sıfıra yakınken [Log](1 + x)'ten daha doğrudur.
//
// Özel durumlar:
//
//	Log1p(+Inf) = +Inf
//	Log1p(±0) = ±0
//	Log1p(-1) = -Inf
//	Log1p(x < -1) = NaN
//	Log1p(NaN) = NaN
func Log1p(x float64) float64 {
	return log1p(x)
}

func log1p(x float64) float64 {
	const (
		Sqrt2M1     = 4.142135623730950488017e-01  // Sqrt(2)-1 = 0x3fda827999fcef34
		Sqrt2HalfM1 = -2.928932188134524755992e-01 // Sqrt(2)/2-1 = 0xbfd2bec333018866
		Small       = 1.0 / (1 << 29)              // 2**-29 = 0x3e20000000000000
		Tiny        = 1.0 / (1 << 54)              // 2**-54
		Ln2Hi       = 6.93147180369123816490e-01   // 3fe62e42fee00000
		Ln2Lo       = 1.90821492927058770002e-10   // 3dea39ef35793c76
		Lp1         = 6.666666666666735130e-01     // 3FE5555555555593
		Lp2         = 3.999999999940941908e-01     // 3FD999999997FA04
		Lp3         = 2.857142874366239149e-01     // 3FD2492494229359
		Lp4         = 2.222219843214978396e-01     // 3FCC71C51D8E78AF
		Lp5         = 1.818357216161805012e-01     // 3FC7466496CB03DE
		Lp6         = 1.531383769920937332e-01     // 3FC39A09D078C69F
		Lp7         = 1.479819860511658591e-01     // 3FC2F112DF3E5244
	)

	// özel durumlar
	switch {
	case x < -1 || math.IsNaN(x): // -Inf dahil
		return math.NaN()
	case x == -1:
		return math.Inf(-1)
	case math.IsInf(x, 1):
		return math.Inf(1)
	}

	absx := math.Abs(x)

	var f float64
	var iu uint64
	k := 1
	if absx < Sqrt2M1 { //  |x| < Sqrt(2)-1
		if absx < Small { // |x| < 2**-29
			if absx < Tiny { // |x| < 2**-54
				return x
			}
			return x - float64(float64(x*x)*0.5)
		}
		if x > Sqrt2HalfM1 { // Sqrt(2)/2-1 < x
			// (Sqrt(2)/2-1) < x < (Sqrt(2)-1)
			k = 0
			f = x
			iu = 1
		}
	}
	var c float64
	if k != 0 {
		f, c, iu, k = log1pReduce(x, absx)
	}
	hfsq := float64(float64(0.5*f) * f)
	var s, R, z float64
	if iu == 0 { // |f| < 2**-20
		if f == 0 {
			if k == 0 {
				return 0
			}
			c += float64(float64(k) * Ln2Lo)
			return float64(float64(k)*Ln2Hi) + c
		}
		R = float64(hfsq * (1.0 - float64(0.66666666666666666*f))) // bölmeden kaçın
		if k == 0 {
			return f - R
		}
		return float64(float64(k)*Ln2Hi) - ((R - (float64(float64(k)*Ln2Lo) + c)) - f)
	}
	s = f / (2.0 + f)
	z = float64(s * s)
	R = float64(z * (Lp1 + float64(z*(Lp2+float64(z*(Lp3+float64(z*(Lp4+float64(z*(Lp5+float64(z*(Lp6+float64(z*Lp7)))))))))))))
	if k == 0 {
		return f - (hfsq - float64(s*(hfsq+R)))
	}
	return float64(float64(k)*Ln2Hi) - ((hfsq - (float64(s*(hfsq+R)) + (float64(float64(k)*Ln2Lo) + c))) - f)
}

// log1pReduce 1+x'i 2**k * (1+f) biçimine indirger; f'yi, yuvarlama düzeltmesi c'yi, f'nin
// kesir bitlerini iu'yu ve k'yi verir.
func log1pReduce(x, absx float64) (f, c float64, iu uint64, k int) {
	const Two53 = 1 << 53 // 2**53
	var u float64
	if absx < Two53 { // 1<<53
		u = 1.0 + x
		iu = math.Float64bits(u)
		k = int((iu >> 52) - 1023)
		// düzeltme terimi
		if k > 0 {
			c = 1.0 - (u - x)
		} else {
			c = x - (u - 1.0)
		}
		c /= u
	} else {
		u = x
		iu = math.Float64bits(u)
		k = int((iu >> 52) - 1023)
		c = 0
	}
	iu &= 0x000fffffffffffff
	if iu < 0x0006a09e667f3bcd { // Sqrt(2)'nin mantisi
		u = math.Float64frombits(iu | 0x3ff0000000000000) // u'yu normalleştir
	} else {
		k++
		u = math.Float64frombits(iu | 0x3fe0000000000000) // u/2'yi normalleştir
		iu = (0x0010000000000000 - iu) >> 2
	}
	f = u - 1.0 // Sqrt(2)/2 < u < Sqrt(2)
	return f, c, iu, k
}

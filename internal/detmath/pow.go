// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

func isOddInt(x float64) bool {
	if math.Abs(x) >= (1 << 53) {
		// 1 << 53, float64 biçiminde tam gösterilebilen en büyük tamsayıdır.
		// Bu aralığın dışındaki her sayı ondalık noktadan önce kesilir,
		// dolayısıyla her zaman çift tamsayıdır.
		// Bu denetim olmazsa ve x int64'ü aşarsa, aşağıdaki int64(xi) dönüşümü
		// bazı mimarilerde yanlış sonuç verebilir (arm64'te verir).
		return false
	}

	xi, xf := math.Modf(x)
	return xf == 0 && int64(xi)&1 == 1
}

// Özel durumlar IEEE Std. 754-2008'in 9.2.1 bölümüne göredir.

// Pow, x**y değerini, yani y'nin x tabanlı üstelini verir.
//
// Özel durumlar (bu sırayla):
//
//	Pow(x, ±0) = 1, her x için
//	Pow(1, y) = 1, her y için
//	Pow(x, 1) = x, her x için
//	Pow(NaN, y) = NaN
//	Pow(x, NaN) = NaN
//	Pow(±0, y) = ±Inf, y tek tamsayı ve < 0 için
//	Pow(±0, -Inf) = +Inf
//	Pow(±0, +Inf) = +0
//	Pow(±0, y) = +Inf, sonlu y < 0 ve tek tamsayı değilse
//	Pow(±0, y) = ±0, y tek tamsayı ve > 0 için
//	Pow(±0, y) = +0, sonlu y > 0 ve tek tamsayı değilse
//	Pow(-1, ±Inf) = 1
//	Pow(x, +Inf) = +Inf, |x| > 1 için
//	Pow(x, -Inf) = +0, |x| > 1 için
//	Pow(x, +Inf) = +0, |x| < 1 için
//	Pow(x, -Inf) = +Inf, |x| < 1 için
//	Pow(+Inf, y) = +Inf, y > 0 için
//	Pow(+Inf, y) = +0, y < 0 için
//	Pow(-Inf, y) = Pow(-0, -y)
//	Pow(x, y) = NaN, sonlu x < 0 ve sonlu tamsayı olmayan y için
func Pow(x, y float64) float64 {
	return pow(x, y)
}

func pow(x, y float64) float64 {
	switch {
	case y == 0 || x == 1:
		return 1
	case y == 1:
		return x
	case math.IsNaN(x) || math.IsNaN(y):
		return math.NaN()
	case x == 0:
		switch {
		case y < 0:
			if math.Signbit(x) && isOddInt(y) {
				return math.Inf(-1)
			}
			return math.Inf(1)
		case y > 0:
			if math.Signbit(x) && isOddInt(y) {
				return x
			}
			return 0
		}
	case math.IsInf(y, 0):
		switch {
		case x == -1:
			return 1
		case (math.Abs(x) < 1) == math.IsInf(y, 1):
			return 0
		default:
			return math.Inf(1)
		}
	case math.IsInf(x, 0):
		if math.IsInf(x, -1) {
			return Pow(1/x, -y) // Pow(-0, -y)
		}
		switch {
		case y < 0:
			return 0
		case y > 0:
			return math.Inf(1)
		}
	case y == 0.5:
		return math.Sqrt(x)
	case y == -0.5:
		return 1 / math.Sqrt(x)
	}

	yi, yf := math.Modf(math.Abs(y))
	if yf != 0 && x < 0 {
		return math.NaN()
	}
	if yi >= 1<<63 {
		// yi, -1 dışındaki her x için taşmaya (ya da 0'a alttaşmaya) yol açan
		// büyük bir çift tamsayıdır (x == 1 daha önce ele alındı)
		switch {
		case x == -1:
			return 1
		case (math.Abs(x) < 1) == (y > 0):
			return 0
		default:
			return math.Inf(1)
		}
	}

	// sonuç = a1 * 2**ae (şimdilik 1).
	a1 := 1.0
	ae := 0

	// sonuç *= x**yf
	if yf != 0 {
		if yf > 0.5 {
			yf--
			yi++
		}
		a1 = Exp(float64(yf * Log(x)))
	}

	// sonuç *= x**yi
	// x'in ardışık karelerini yi'nin bitlerine göre çarparak.
	// ikinin kuvvetleri üs olarak birikir.
	x1, xe := math.Frexp(x)
	for i := int64(yi); i != 0; i >>= 1 {
		if xe < -1<<12 || 1<<12 < xe {
			// xe, aşağıdaki sola kaydırmayı taşırmadan önce yakalanır.
			// i != 0 olduğundan en az bir biti hâlâ 1'dir; ae en az bir yinelemede
			// daha xe biriktirecektir, yani ae += xe, ae için bir alt sınırdır.
			// Bu alt sınır float64 üssünün boyutunu aşar,
			// dolayısıyla son Ldexp çağrısı alttaşma ya da taşma (0/Inf) verir.
			ae += xe
			break
		}
		if i&1 == 1 {
			a1 = float64(a1 * x1)
			ae += xe
		}
		x1 = float64(x1 * x1)
		xe <<= 1
		if x1 < .5 {
			x1 += x1
			xe--
		}
	}

	// sonuç = a1*2**ae
	// y < 0 ise sonuç = 1 / sonuç
	// ama ters sırayla
	if y < 0 {
		a1 = 1 / a1
		ae = -ae
	}
	return math.Ldexp(a1, ae)
}

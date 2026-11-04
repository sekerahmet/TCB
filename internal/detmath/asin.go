// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

/*
	Kayan noktalı arksinüs ve arkkosinüs.

	İkisi de uygun aralık indirgemesinden sonra
	arktanjant hesaplanarak gerçeklenir.
*/

// Asin, x'in arksinüsünü radyan olarak verir.
//
// Özel durumlar:
//
//	Asin(±0) = ±0
//	Asin(x) = NaN, x < -1 veya x > 1 için
func Asin(x float64) float64 {
	return asin(x)
}

func asin(x float64) float64 {
	if x == 0 {
		return x // özel durum
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x > 1 {
		return math.NaN() // özel durum
	}

	temp := math.Sqrt(1 - float64(x*x))
	if x > 0.7 {
		temp = math.Pi/2 - satan(temp/x)
	} else {
		temp = satan(x / temp)
	}

	if sign {
		temp = -temp
	}
	return temp
}

// Acos, x'in arkkosinüsünü radyan olarak verir.
//
// Özel durum:
//
//	Acos(x) = NaN, x < -1 veya x > 1 için
func Acos(x float64) float64 {
	return acos(x)
}

func acos(x float64) float64 {
	return math.Pi/2 - Asin(x)
}

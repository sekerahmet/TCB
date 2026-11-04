// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

// Log10, x'in onluk logaritmasını verir.
// Özel durumlar [Log] ile aynıdır.
func Log10(x float64) float64 {
	return log10(x)
}

func log10(x float64) float64 {
	return float64(Log(x) * (1 / math.Ln10))
}

// Log2, x'in ikilik logaritmasını verir.
// Özel durumlar [Log] ile aynıdır.
func Log2(x float64) float64 {
	return log2(x)
}

func log2(x float64) float64 {
	frac, exp := math.Frexp(x)
	// İkinin tam kuvvetleri tam sonuç versin.
	// Log(0.5)*(1/Ln2)+exp'in tam olarak exp-1 çıkmasına güvenme.
	if frac == 0.5 {
		return float64(exp - 1)
	}
	return float64(Log(frac)*(1/math.Ln2)) + float64(exp)
}

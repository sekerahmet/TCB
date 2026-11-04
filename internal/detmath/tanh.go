// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
// Yöntem: |x| < 0.625 için Cody ve Waite'in x + x**3 P(x)/Q(x) biçimli
// rasyonel fonksiyonu kullanılır. Diğer durumlarda
// tanh(x) = sinh(x)/cosh(x) = 1 - 2/(exp(2x) + 1).
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov
//

var tanhP = [...]float64{
	-9.64399179425052238628e-1,
	-9.92877231001918586564e1,
	-1.61468768441708447952e3,
}
var tanhQ = [...]float64{
	1.12811678491632931402e2,
	2.23548839060100448583e3,
	4.84406305325125486048e3,
}

// Tanh, x'in hiperbolik tanjantını verir.
//
// Özel durumlar:
//
//	Tanh(±0) = ±0
//	Tanh(±Inf) = ±1
//	Tanh(NaN) = NaN
func Tanh(x float64) float64 {
	return tanh(x)
}

func tanh(x float64) float64 {
	const maxLog = 8.8029691931113054295988e+01 // log(2**127)
	z := math.Abs(x)
	switch {
	case z > 0.5*maxLog:
		if x < 0 {
			return -1
		}
		return 1
	case z >= 0.625:
		s := Exp(float64(2 * z))
		z = 1 - 2/(s+1)
		if x < 0 {
			z = -z
		}
	default:
		if x == 0 {
			return x
		}
		s := float64(x * x)
		z = x + float64(float64(x*s)*(float64((float64(tanhP[0]*s)+tanhP[1])*s)+tanhP[2]))/(float64((float64((s+tanhQ[0])*s)+tanhQ[1])*s)+tanhQ[2])
	}
	return z
}

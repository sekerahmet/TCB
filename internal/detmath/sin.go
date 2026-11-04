// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

/*
	Kayan noktalı sinüs ve kosinüs.
*/

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
// Yöntem: bağımsız değişken pi/4'lük aralıklara indirgenir; indirgeme hatası
// genişletilmiş duyarlıklı modüler aritmetikle hemen hemen yok edilir.
// İki çokterimli yaklaşım kullanılır. sin için 0 ile pi/4 arasında
// x + x**3 P(x**2), pi/4 ile pi/2 arasında kosinüs 1 - x**2 Q(x**2) olarak
// temsil edilir; cos için roller tersine döner.
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

// sin katsayıları
var _sin = [...]float64{
	1.58962301576546568060e-10, // 0x3de5d8fd1fd19ccd
	-2.50507477628578072866e-8, // 0xbe5ae5e5a9291f5d
	2.75573136213857245213e-6,  // 0x3ec71de3567d48a1
	-1.98412698295895385996e-4, // 0xbf2a01a019bfdf03
	8.33333333332211858878e-3,  // 0x3f8111111110f7d0
	-1.66666666666666307295e-1, // 0xbfc5555555555548
}

// cos katsayıları
var _cos = [...]float64{
	-1.13585365213876817300e-11, // 0xbda8fa49a0861a9b
	2.08757008419747316778e-9,   // 0x3e21ee9d7b4e3f05
	-2.75573141792967388112e-7,  // 0xbe927e4f7eac4bc6
	2.48015872888517045348e-5,   // 0x3efa01a019c844f5
	-1.38888888888730564116e-3,  // 0xbf56c16c16c14f91
	4.16666666666665929218e-2,   // 0x3fa555555555554b
}

// Cos, radyan cinsinden x'in kosinüsünü verir.
//
// Özel durumlar:
//
//	Cos(±Inf) = NaN
//	Cos(NaN) = NaN
func Cos(x float64) float64 {
	return cos(x)
}

func cos(x float64) float64 {
	const (
		PI4A = 7.85398125648498535156e-1  // 0x3fe921fb40000000, Pi/4 üç parçaya bölünmüş
		PI4B = 3.77489470793079817668e-8  // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645e-15 // 0x3ce8469898cc5170,
	)
	// özel durumlar
	switch {
	case math.IsNaN(x) || math.IsInf(x, 0):
		return math.NaN()
	}

	// bağımsız değişkeni pozitif yap
	sign := false
	x = math.Abs(x)

	var j uint64
	var y, z float64
	if x >= reduceThreshold {
		j, z = trigReduce(x)
	} else {
		j = uint64(float64(x * (4 / math.Pi))) // x/(Pi/4)'ün tamsayı kısmı, evre açısı sınamaları için tamsayı olarak
		y = float64(j)                         // x/(Pi/4)'ün tamsayı kısmı, kayan noktalı olarak

		// sıfırları başlangıca eşle
		if j&1 == 1 {
			j++
			y++
		}
		j &= 7                                                          // 2Pi radyan (360 derece) modülünde sekizde bir dilim
		z = ((x - float64(y*PI4A)) - float64(y*PI4B)) - float64(y*PI4C) // genişletilmiş duyarlıklı modüler aritmetik
	}

	if j > 3 {
		j -= 4
		sign = !sign
	}
	if j > 1 {
		sign = !sign
	}

	zz := float64(z * z)
	if j == 1 || j == 2 {
		y = z + float64(float64(z*zz)*(float64((float64((float64((float64(((float64(_sin[0]*zz))+_sin[1])*zz)+_sin[2])*zz)+_sin[3])*zz)+_sin[4])*zz)+_sin[5]))
	} else {
		y = 1.0 - float64(0.5*zz) + float64(float64(zz*zz)*(float64((float64((float64((float64(((float64(_cos[0]*zz))+_cos[1])*zz)+_cos[2])*zz)+_cos[3])*zz)+_cos[4])*zz)+_cos[5]))
	}
	if sign {
		y = -y
	}
	return y
}

// Sin, radyan cinsinden x'in sinüsünü verir.
//
// Özel durumlar:
//
//	Sin(±0) = ±0
//	Sin(±Inf) = NaN
//	Sin(NaN) = NaN
func Sin(x float64) float64 {
	return sin(x)
}

func sin(x float64) float64 {
	const (
		PI4A = 7.85398125648498535156e-1  // 0x3fe921fb40000000, Pi/4 üç parçaya bölünmüş
		PI4B = 3.77489470793079817668e-8  // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645e-15 // 0x3ce8469898cc5170,
	)
	// özel durumlar
	switch {
	case x == 0 || math.IsNaN(x):
		return x // ±0 ya da NaN döner
	case math.IsInf(x, 0):
		return math.NaN()
	}

	// bağımsız değişkeni pozitif yap ama işareti sakla
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	var j uint64
	var y, z float64
	if x >= reduceThreshold {
		j, z = trigReduce(x)
	} else {
		j = uint64(float64(x * (4 / math.Pi))) // x/(Pi/4)'ün tamsayı kısmı, evre açısı sınamaları için tamsayı olarak
		y = float64(j)                         // x/(Pi/4)'ün tamsayı kısmı, kayan noktalı olarak

		// sıfırları başlangıca eşle
		if j&1 == 1 {
			j++
			y++
		}
		j &= 7                                                          // 2Pi radyan (360 derece) modülünde sekizde bir dilim
		z = ((x - float64(y*PI4A)) - float64(y*PI4B)) - float64(y*PI4C) // genişletilmiş duyarlıklı modüler aritmetik
	}
	// x eksenine göre yansıt
	if j > 3 {
		sign = !sign
		j -= 4
	}
	zz := float64(z * z)
	if j == 1 || j == 2 {
		y = 1.0 - float64(0.5*zz) + float64(float64(zz*zz)*(float64((float64((float64((float64(((float64(_cos[0]*zz))+_cos[1])*zz)+_cos[2])*zz)+_cos[3])*zz)+_cos[4])*zz)+_cos[5]))
	} else {
		y = z + float64(float64(z*zz)*(float64((float64((float64((float64(((float64(_sin[0]*zz))+_sin[1])*zz)+_sin[2])*zz)+_sin[3])*zz)+_sin[4])*zz)+_sin[5]))
	}
	if sign {
		y = -y
	}
	return y
}

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

/*
	Kayan noktalı tanjant.
*/

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
// Yöntem: bağımsız değişken pi/4 modülünde indirgenir; [0, pi/4] aralığında
// x + x**3 P(x**2)/Q(x**2) biçiminde bir rasyonel fonksiyon kullanılır.
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

// tan katsayıları
var _tanP = [...]float64{
	-1.30936939181383777646e4, // 0xc0c992d8d24f3f38
	1.15351664838587416140e6,  // 0x413199eca5fc9ddd
	-1.79565251976484877988e7, // 0xc1711fead3299176
}
var _tanQ = [...]float64{
	1.00000000000000000000e0,
	1.36812963470692954678e4,  // 0x40cab8a5eeb36572
	-1.32089234440210967447e6, // 0xc13427bc582abc96
	2.50083801823357915839e7,  // 0x4177d98fc2ead8ef
	-5.38695755929454629881e7, // 0xc189afe03cbe5a31
}

// Tan, radyan cinsinden x'in tanjantını verir.
//
// Özel durumlar:
//
//	Tan(±0) = ±0
//	Tan(±Inf) = NaN
//	Tan(NaN) = NaN
func Tan(x float64) float64 {
	return tan(x)
}

func tan(x float64) float64 {
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

		/* sıfırları ve tekillikleri başlangıca eşle */
		if j&1 == 1 {
			j++
			y++
		}

		z = ((x - float64(y*PI4A)) - float64(y*PI4B)) - float64(y*PI4C)
	}
	zz := float64(z * z)

	if zz > 1e-14 {
		y = z + float64(z*(float64(zz*(float64(((float64(_tanP[0]*zz))+_tanP[1])*zz)+_tanP[2]))/(float64((float64((float64((zz+_tanQ[1])*zz)+_tanQ[2])*zz)+_tanQ[3])*zz)+_tanQ[4])))
	} else {
		y = z
	}
	if j&2 == 2 {
		y = -1 / y
	}
	if sign {
		y = -y
	}
	return y
}

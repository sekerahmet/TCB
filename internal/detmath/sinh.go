// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import "math"

/*
	Kayan noktalı hiperbolik sinüs ve kosinüs.

	Büyüklüğü 0.5'ten fazla olan bağımsız değişkenlerde
	üstel fonksiyon çağrılır.

	Büyüklüğü 0.5'ten küçük olanlarda bir seri kullanılır.

	Cosh(x) her bağımsız değişken için üstel fonksiyondan hesaplanır.
*/

// Sinh, x'in hiperbolik sinüsünü verir.
//
// Özel durumlar:
//
//	Sinh(±0) = ±0
//	Sinh(±Inf) = ±Inf
//	Sinh(NaN) = NaN
func Sinh(x float64) float64 {
	return sinh(x)
}

func sinh(x float64) float64 {
	// Katsayılar Hart ve Cheney'nin 2029 numaralı tablosundandır (20.36D).
	const (
		P0 = -0.6307673640497716991184787251e+6
		P1 = -0.8991272022039509355398013511e+5
		P2 = -0.2894211355989563807284660366e+4
		P3 = -0.2630563213397497062819489e+2
		Q0 = -0.6307673640497716991212077277e+6
		Q1 = 0.1521517378790019070696485176e+5
		Q2 = -0.173678953558233699533450911e+3
	)

	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	var temp float64
	switch {
	case x > 21:
		temp = float64(Exp(x) * 0.5)

	case x > 0.5:
		ex := Exp(x)
		temp = float64((ex - 1/ex) * 0.5)

	default:
		sq := float64(x * x)
		temp = float64((float64((float64((float64(P3*sq)+P2)*sq)+P1)*sq) + P0) * x)
		temp = temp / (float64((float64((sq+Q2)*sq)+Q1)*sq) + Q0)
	}

	if sign {
		temp = -temp
	}
	return temp
}

// Cosh, x'in hiperbolik kosinüsünü verir.
//
// Özel durumlar:
//
//	Cosh(±0) = 1
//	Cosh(±Inf) = +Inf
//	Cosh(NaN) = NaN
func Cosh(x float64) float64 {
	return cosh(x)
}

func cosh(x float64) float64 {
	x = math.Abs(x)
	if x > 21 {
		return float64(Exp(x) * 0.5)
	}
	ex := Exp(x)
	return float64((ex + 1/ex) * 0.5)
}

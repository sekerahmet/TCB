package tensor

import "math"

// Sinüs ve kosinüs (konum kodlamaları, dönel konum gömmesi gibi açılı hesaplar için).

// Sin eleman bazlı sinüs (radyan); türevi cos(x).
func (t *Tensor) Sin() *Tensor { return unary(t, OpSin, 0) }

// Cos eleman bazlı kosinüs (radyan); türevi -sin(x).
func (t *Tensor) Cos() *Tensor { return unary(t, OpCos, 0) }

// Aralık indirgemesi float64'te yapılır: x = n·(π/2) + r, |r| ≤ π/4 (π/2 iki
// parçalı sabitle; n·pio2Yuksek tam gösterilebilir). r üzerinde sin için 9.,
// cos için 10. dereceye kadar Taylor polinomları float64'te hesaplanır (kesme
// hatası 3·10⁻⁹'dan küçük, float32 yarım ulp'unun altında); sonuç float32'ye
// bir kez yuvarlanır. Çok büyük |x| (indirgemenin kesinliğini yitirdiği
// bölge), NaN ve sonsuzlar kütüphane işlevine düşer.
const (
	twoOverPi   = 0.63661977236758134308
	pio2Hi      = 1.57079632673412561417e+00 // π/2'nin ilk 33 biti
	pio2Lo      = 6.07710050650619224932e-11 // π/2 - pio2Yuksek
	sinCosLimit = 1 << 17                    // n·pio2Yuksek bunun altında tam
	round64     = 6755399441055744.0         // 1.5·2^52
)

// sinCosReduce x'i çeyreğe (0-3) ve [-π/4, π/4] aralığındaki r'ye indirger.
// ok yanlışsa x indirgeme bölgesinin dışındadır (ya da NaN'dır).
func sinCosReduce(x float32) (r float64, quarter int, ok bool) {
	xd := float64(x)
	if !(xd > -sinCosLimit && xd < sinCosLimit) {
		return 0, 0, false
	}
	fn := (xd*twoOverPi + round64) - round64
	r = (xd - fn*pio2Hi) - fn*pio2Lo
	return r, int(int64(fn)) & 3, true
}

// sinPoly ve cosPoli |r| ≤ π/4 için sin r ve cos r.
func sinPoly(r float64) float64 {
	z := r * r
	return r + r*z*(-1.0/6+z*(1.0/120+z*(-1.0/5040+z*(1.0/362880))))
}

func cosPoly(r float64) float64 {
	z := r * r
	return 1 + z*(-0.5+z*(1.0/24+z*(-1.0/720+z*(1.0/40320+z*(-1.0/3628800)))))
}

// sinf32 sin(x).
func sinf32(x float32) float32 {
	r, c, ok := sinCosReduce(x)
	if !ok {
		return float32(math.Sin(float64(x)))
	}
	switch c {
	case 0:
		return float32(sinPoly(r))
	case 1:
		return float32(cosPoly(r))
	case 2:
		return float32(-sinPoly(r))
	}
	return float32(-cosPoly(r))
}

// cosf32 cos(x).
func cosf32(x float32) float32 {
	r, c, ok := sinCosReduce(x)
	if !ok {
		return float32(math.Cos(float64(x)))
	}
	switch c {
	case 0:
		return float32(cosPoly(r))
	case 1:
		return float32(-sinPoly(r))
	case 2:
		return float32(-cosPoly(r))
	}
	return float32(sinPoly(r))
}

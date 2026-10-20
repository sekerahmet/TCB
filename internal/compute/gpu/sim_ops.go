package gpu

import (
	"math"
)

// PTX benzetimi: kayan noktalı işlemler ve dönüşümler. Yaklaşık komutlar (ex2, lg2, sin,
// cos) tam doğrulukla hesaplanır; donanım birkaç ulp farklı olabilir, sınamalar bu yüzden
// toleranslıdır. Normal altı sayılar korunur (donanımda .ftz'siz komutlar gibi).

func computeF32(k *simInst, ab, bb, cb uint64) (uint64, error) {
	a, b, c := f32(ab), f32(bb), f32(cb)
	switch k.op {
	case simMov:
		return ab, nil
	case simAdd:
		return f32b(a + b), nil
	case simSub:
		return f32b(a - b), nil
	case simMul:
		return f32b(a * b), nil
	case simFma, simMad:
		return f32b(float32(math.FMA(float64(a), float64(b), float64(c)))), nil
	case simDiv:
		return f32b(a / b), nil
	case simNeg:
		return ab ^ (1 << 31), nil
	case simAbs:
		return ab &^ (1 << 31), nil
	case simMin, simMax:
		if k.mode2 == simNaNProp && (a != a || b != b) {
			return 0x7FFFFFFF, nil // kanonik NaN
		}
		return f32b(minMaxF32(a, b, k.op == simMin)), nil
	case simSqrt:
		return f32b(float32(math.Sqrt(float64(a)))), nil
	case simRcp:
		return f32b(1 / a), nil
	case simEx2:
		return f32b(float32(math.Exp2(float64(a)))), nil
	case simLg2:
		return f32b(float32(math.Log2(float64(a)))), nil
	case simSin:
		return f32b(float32(math.Sin(float64(a)))), nil
	case simCos:
		return f32b(float32(math.Cos(float64(a)))), nil
	case simSelp:
		if cb != 0 {
			return ab, nil
		}
		return bb, nil
	case simSetp:
		return compareF(k.mode2, float64(a), float64(b)), nil
	}
	return 0, internalf("operation unsupported by the simulator for .f32")
}

func computeF64(k *simInst, ab, bb, cb uint64) (uint64, error) {
	a, b, c := f64(ab), f64(bb), f64(cb)
	switch k.op {
	case simMov:
		return ab, nil
	case simAdd:
		return f64b(a + b), nil
	case simSub:
		return f64b(a - b), nil
	case simMul:
		return f64b(a * b), nil
	case simFma:
		return f64b(math.FMA(a, b, c)), nil
	case simDiv:
		return f64b(a / b), nil
	case simSqrt:
		return f64b(math.Sqrt(a)), nil
	case simSelp:
		if cb != 0 {
			return ab, nil
		}
		return bb, nil
	case simSetp:
		return compareF(k.mode2, a, b), nil
	}
	return 0, internalf("operation unsupported by the simulator for .f64")
}

// minMaxF32 PTX min/max: işlenenlerden biri NaN ise diğeri döner.
func minMaxF32(a, b float32, smallest bool) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case (a < b) == smallest:
		return a
	}
	return b
}

// compareF sıralı (eq, lt, ...) ve sırasız (equ, ltu, ...) karşılaştırmalar.
func compareF(mode2 int, a, b float64) uint64 {
	nan := a != a || b != b
	var r bool
	switch mode2 {
	case kEq:
		r = !nan && a == b
	case kNe:
		r = !nan && a != b
	case kLt:
		r = !nan && a < b
	case kLe:
		r = !nan && a <= b
	case kGt:
		r = !nan && a > b
	case kGe:
		r = !nan && a >= b
	case kEqu:
		r = nan || a == b
	case kNeu:
		r = nan || a != b
	case kLtu:
		r = nan || a < b
	case kLeu:
		r = nan || a <= b
	case kGtu:
		r = nan || a > b
	case kGeu:
		r = nan || a >= b
	case kNum:
		r = !nan
	case kNan:
		r = nan
	}
	if r {
		return 1
	}
	return 0
}

// convert cvt: k.tur hedef, k.kaynak kaynak türü; k.kip yuvarlama (1 rni, 2 rzi, 3 rmi,
// 4 rpi).
func convert(k *simInst, a uint64) (uint64, error) {
	switch k.source {
	case simF32, simF64, simF16:
		var f float64
		switch k.source {
		case simF32:
			f = float64(f32(a))
		case simF64:
			f = f64(a)
		default:
			f = float64(f16ToF32Sim(uint16(a)))
		}
		return fromFloating(k, f)
	case simS32:
		return fromInt(k, float64(int32(a)), int64(int32(a)))
	case simS64:
		return fromInt(k, float64(int64(a)), int64(a))
	case simU16:
		return fromInt(k, float64(uint16(a)), int64(uint16(a)))
	case simU64:
		return fromInt(k, float64(a), int64(a))
	}
	return fromInt(k, float64(uint32(a)), int64(uint32(a)))
}

func round(mode2 int, f float64) float64 {
	switch mode2 {
	case 1:
		return math.RoundToEven(f)
	case 2:
		return math.Trunc(f)
	case 3:
		return math.Floor(f)
	case 4:
		return math.Ceil(f)
	}
	return f
}

func fromFloating(k *simInst, f float64) (uint64, error) {
	switch k.dtype {
	case simF32:
		return f32b(float32(round(k.mode2, f))), nil
	case simF64:
		return f64b(round(k.mode2, f)), nil
	case simF16:
		return uint64(f32ToF16Sim(float32(f))), nil
	case simS32:
		if k.mode2 == 0 {
			return 0, internalf("cvt from float to integer needs a rounding mode")
		}
		r := round(k.mode2, f)
		switch {
		case f != f:
			return 0, nil
		case r >= math.MaxInt32:
			return uint64(uint32(math.MaxInt32)), nil
		case r <= math.MinInt32:
			return uint64(uint32(1 << 31)), nil
		}
		return uint64(uint32(int32(r))), nil
	}
	return 0, internalf("cvt target unsupported by the simulator")
}

func fromInt(k *simInst, f float64, s int64) (uint64, error) {
	switch k.dtype {
	case simF32:
		return f32b(float32(f)), nil
	case simF64:
		return f64b(f), nil
	case simU16:
		return uint64(uint16(s)), nil
	case simU32, simS32:
		return uint64(uint32(s)), nil
	case simU64, simS64:
		return uint64(s), nil
	}
	return 0, internalf("cvt target unsupported by the simulator")
}

// f32ToF16Sim IEEE yarım hassasiyete en yakına-çifte yuvarlama (donanımın cvt.rn.f16.f32
// komutunun benzetimi): taşma sonsuza, küçükler normal altına ya da sıfıra gider.
func f32ToF16Sim(f float32) uint16 {
	b := math.Float32bits(f)
	sign := uint16(b>>16) & 0x8000
	if f != f {
		return sign | 0x7E00
	}
	a := math.Abs(float64(f))
	if a >= 65520 {
		return sign | 0x7C00
	}
	// 2^-24 birimlerinde (en küçük normal altı) ya da üsse göre ölçekleyip en yakına-çifte
	// yuvarla; float64 bu aralıkta tam doğrudur.
	if a < 6.103515625e-05 { // normal altı
		m := math.RoundToEven(a * (1 << 24))
		return sign | uint16(m)
	}
	fraction, us := math.Frexp(a) // a = kesir·2^us, kesir ∈ [0,5, 1)
	us--
	m := math.RoundToEven(fraction*2*1024) - 1024
	if m >= 1024 { // anlamlı kısım bir sonraki üsse taştı
		us++
		m = 0
	}
	return sign | uint16(us+15)<<10 | uint16(m)
}

// f16ToF32Sim yarım hassasiyeti float32'ye açar.
func f16ToF32Sim(h uint16) float32 {
	sign := 1.0
	if h&0x8000 != 0 {
		sign = -1
	}
	us := int(h>>10) & 0x1F
	m := float64(h & 0x3FF)
	switch us {
	case 0x1F:
		if m != 0 {
			return float32(math.NaN())
		}
		return float32(math.Inf(int(sign)))
	case 0:
		return float32(sign * m * math.Exp2(-24))
	}
	return float32(sign * (1 + m/1024) * math.Exp2(float64(us-15)))
}

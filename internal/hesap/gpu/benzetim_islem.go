package gpu

import (
	"fmt"
	"math"
)

// PTX benzetimi: kayan noktalı işlemler ve dönüşümler. Yaklaşık komutlar (ex2, lg2, sin,
// cos) tam doğrulukla hesaplanır; donanım birkaç ulp farklı olabilir, sınamalar bu yüzden
// toleranslıdır. Normal altı sayılar korunur (donanımda .ftz'siz komutlar gibi).

func hesaplaF32(k *bzKomut, ab, bb, cb uint64) (uint64, error) {
	a, b, c := f32(ab), f32(bb), f32(cb)
	switch k.islem {
	case bzMov:
		return ab, nil
	case bzAdd:
		return f32b(a + b), nil
	case bzSub:
		return f32b(a - b), nil
	case bzMul:
		return f32b(a * b), nil
	case bzFma, bzMad:
		return f32b(float32(math.FMA(float64(a), float64(b), float64(c)))), nil
	case bzDiv:
		return f32b(a / b), nil
	case bzNeg:
		return ab ^ (1 << 31), nil
	case bzAbs:
		return ab &^ (1 << 31), nil
	case bzMin, bzMax:
		return f32b(minMaxF32(a, b, k.islem == bzMin)), nil
	case bzSqrt:
		return f32b(float32(math.Sqrt(float64(a)))), nil
	case bzRcp:
		return f32b(1 / a), nil
	case bzEx2:
		return f32b(float32(math.Exp2(float64(a)))), nil
	case bzLg2:
		return f32b(float32(math.Log2(float64(a)))), nil
	case bzSin:
		return f32b(float32(math.Sin(float64(a)))), nil
	case bzCos:
		return f32b(float32(math.Cos(float64(a)))), nil
	case bzSelp:
		if cb != 0 {
			return ab, nil
		}
		return bb, nil
	case bzSetp:
		return karsilastirF(k.kip, float64(a), float64(b)), nil
	}
	return 0, fmt.Errorf("benzetimde .f32 için desteklenmeyen işlem")
}

func hesaplaF64(k *bzKomut, ab, bb, cb uint64) (uint64, error) {
	a, b, c := f64(ab), f64(bb), f64(cb)
	switch k.islem {
	case bzMov:
		return ab, nil
	case bzAdd:
		return f64b(a + b), nil
	case bzSub:
		return f64b(a - b), nil
	case bzMul:
		return f64b(a * b), nil
	case bzFma:
		return f64b(math.FMA(a, b, c)), nil
	case bzDiv:
		return f64b(a / b), nil
	case bzSqrt:
		return f64b(math.Sqrt(a)), nil
	case bzSelp:
		if cb != 0 {
			return ab, nil
		}
		return bb, nil
	case bzSetp:
		return karsilastirF(k.kip, a, b), nil
	}
	return 0, fmt.Errorf("benzetimde .f64 için desteklenmeyen işlem")
}

// minMaxF32 PTX min/max: işlenenlerden biri NaN ise diğeri döner.
func minMaxF32(a, b float32, enKucuk bool) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case (a < b) == enKucuk:
		return a
	}
	return b
}

// karsilastirF sıralı (eq, lt, ...) ve sırasız (equ, ltu, ...) karşılaştırmalar.
func karsilastirF(kip int, a, b float64) uint64 {
	nan := a != a || b != b
	var r bool
	switch kip {
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

// donustur cvt: k.tur hedef, k.kaynak kaynak türü; k.kip yuvarlama (1 rni, 2 rzi, 3 rmi,
// 4 rpi).
func donustur(k *bzKomut, a uint64) (uint64, error) {
	switch k.kaynak {
	case bzF32, bzF64, bzF16:
		var f float64
		switch k.kaynak {
		case bzF32:
			f = float64(f32(a))
		case bzF64:
			f = f64(a)
		default:
			f = float64(f16ToF32Bz(uint16(a)))
		}
		return kayanDan(k, f)
	case bzS32:
		return tamdan(k, float64(int32(a)), int64(int32(a)))
	case bzS64:
		return tamdan(k, float64(int64(a)), int64(a))
	case bzU16:
		return tamdan(k, float64(uint16(a)), int64(uint16(a)))
	case bzU64:
		return tamdan(k, float64(a), int64(a))
	}
	return tamdan(k, float64(uint32(a)), int64(uint32(a)))
}

func yuvarla(kip int, f float64) float64 {
	switch kip {
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

func kayanDan(k *bzKomut, f float64) (uint64, error) {
	switch k.tur {
	case bzF32:
		return f32b(float32(yuvarla(k.kip, f))), nil
	case bzF64:
		return f64b(yuvarla(k.kip, f)), nil
	case bzF16:
		return uint64(f32ToF16Bz(float32(f))), nil
	case bzS32:
		if k.kip == 0 {
			return 0, fmt.Errorf("kayan noktadan tam sayıya cvt yuvarlama kipi ister")
		}
		r := yuvarla(k.kip, f)
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
	return 0, fmt.Errorf("benzetimde desteklenmeyen cvt hedefi")
}

func tamdan(k *bzKomut, f float64, s int64) (uint64, error) {
	switch k.tur {
	case bzF32:
		return f32b(float32(f)), nil
	case bzF64:
		return f64b(f), nil
	case bzU16:
		return uint64(uint16(s)), nil
	case bzU32, bzS32:
		return uint64(uint32(s)), nil
	case bzU64, bzS64:
		return uint64(s), nil
	}
	return 0, fmt.Errorf("benzetimde desteklenmeyen cvt hedefi")
}

// f32ToF16Bz IEEE yarım hassasiyete en yakına-çifte yuvarlama (donanımın cvt.rn.f16.f32
// komutunun benzetimi): taşma sonsuza, küçükler normal altına ya da sıfıra gider.
func f32ToF16Bz(f float32) uint16 {
	b := math.Float32bits(f)
	isaret := uint16(b>>16) & 0x8000
	if f != f {
		return isaret | 0x7E00
	}
	a := math.Abs(float64(f))
	if a >= 65520 {
		return isaret | 0x7C00
	}
	// 2^-24 birimlerinde (en küçük normal altı) ya da üsse göre ölçekleyip en yakına-çifte
	// yuvarla; float64 bu aralıkta tam doğrudur.
	if a < 6.103515625e-05 { // normal altı
		m := math.RoundToEven(a * (1 << 24))
		return isaret | uint16(m)
	}
	kesir, us := math.Frexp(a) // a = kesir·2^us, kesir ∈ [0,5, 1)
	us--
	m := math.RoundToEven(kesir*2*1024) - 1024
	if m >= 1024 { // anlamlı kısım bir sonraki üsse taştı
		us++
		m = 0
	}
	return isaret | uint16(us+15)<<10 | uint16(m)
}

// f16ToF32Bz yarım hassasiyeti float32'ye açar.
func f16ToF32Bz(h uint16) float32 {
	isaret := 1.0
	if h&0x8000 != 0 {
		isaret = -1
	}
	us := int(h>>10) & 0x1F
	m := float64(h & 0x3FF)
	switch us {
	case 0x1F:
		if m != 0 {
			return float32(math.NaN())
		}
		return float32(math.Inf(int(isaret)))
	case 0:
		return float32(isaret * m * math.Exp2(-24))
	}
	return float32(isaret * (1 + m/1024) * math.Exp2(float64(us-15)))
}

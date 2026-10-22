package interp

import (
	"math"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerleşiklerin sayı kuralları işleçlerle aynıdır: Tam sayılar tam (int64) olarak
// hesaplanır ve karşılaştırılır, sınırı aşan sonuç sessizce sarmak yerine "Tam sayı
// taşması" hatası verir (3.1); Ondalık'a çevrim yalnızca bir Ondalık işlenen varsa olur.

// intLimit 2^63'tür: int64'e çevrilebilecek Ondalık değerler [-2^63, 2^63) aralığındadır.
const intLimit = 9223372036854775808.0

// floatToInt bir Ondalık değeri (kesirli kısmı atılmış) Tam'a çevirir; NaN, sonsuz ya
// da int64 aralığı dışındaki değerde hata verir.
func floatToInt(name string, f float64) (int64, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f >= intLimit || f < -intLimit {
		t := codes.Name(codes.A200)
		return 0, messages.New(messages.H2484, name, (&FloatVal{Val: f}).Inspect(), t)
	}
	return int64(f), nil
}

// compareIntFloat bir Tam ile (NaN olmayan) bir Ondalığı tam olarak karşılaştırır
// (Ondalığa çevirmek 2^53 üstünde eşitlik uydururdu): a < f ise -1, eşitse 0, büyükse 1.
func compareIntFloat(a int64, f float64) int {
	if f >= intLimit {
		return -1
	}
	if f < -intLimit {
		return 1
	}
	t := math.Trunc(f)
	ti := int64(t)
	switch {
	case a < ti:
		return -1
	case a > ti:
		return 1
	case f > t:
		return -1
	case f < t:
		return 1
	}
	return 0
}

// compareNumbers iki sayıyı (Tam ya da Ondalık) tam olarak karşılaştırır. ok yanlışsa
// değerlerden biri sayı değildir ya da NaN'dır (karşılaştırılamaz).
func compareNumbers(a, b Value) (int, bool) {
	switch x := a.(type) {
	case *IntVal:
		switch y := b.(type) {
		case *IntVal:
			switch {
			case x.Val < y.Val:
				return -1, true
			case x.Val > y.Val:
				return 1, true
			}
			return 0, true
		case *FloatVal:
			if math.IsNaN(y.Val) {
				return 0, false
			}
			return compareIntFloat(x.Val, y.Val), true
		}
	case *FloatVal:
		if math.IsNaN(x.Val) {
			return 0, false
		}
		switch y := b.(type) {
		case *IntVal:
			return -compareIntFloat(y.Val, x.Val), true
		case *FloatVal:
			switch {
			case math.IsNaN(y.Val):
				return 0, false
			case x.Val < y.Val:
				return -1, true
			case x.Val > y.Val:
				return 1, true
			}
			return 0, true
		}
	}
	return 0, false
}

// isNumber değerin Tam ya da Ondalık olup olmadığını söyler.
func isNumber(v Value) bool {
	switch v.(type) {
	case *IntVal, *FloatVal:
		return true
	}
	return false
}

// sumNumbers bir listenin elemanlarını işleçlerle aynı kuralla toplar: hepsi Tam ise sonuç
// Tam'dır ve taşma hata verir; bir Ondalık varsa sonuç Ondalık'tır. Sayı olmayan eleman
// hatadır. ad hata mesajındaki yerleşiğin adıdır.
func sumNumbers(name string, elems []Value) (Value, error) {
	var intNum int64
	var float float64
	hasFloat := false
	for i, el := range elems {
		switch x := el.(type) {
		case *IntVal:
			if hasFloat {
				float += float64(x.Val)
				continue
			}
			s := intNum + x.Val
			if (s > intNum) != (x.Val > 0) {
				return nil, messages.New(messages.H2485, name, codes.DisplayName(codes.A200), intNum, x.Val, i+1, codes.DisplayName(codes.G211))
			}
			intNum = s
		case *FloatVal:
			if !hasFloat {
				hasFloat = true
				float = float64(intNum)
			}
			float += x.Val
		default:
			return nil, ListElementError(name, i, numberType, el)
		}
	}
	if hasFloat {
		return &FloatVal{Val: float}, nil
	}
	return NewInt(intNum), nil
}

// ListElementError reports that element i (0-based) of a list has the wrong type.
func ListElementError(name string, i int, expected string, given Value) error {
	return messages.New(messages.H2486, name, i+1, expected, TypeAndSummary(given))
}

// extreme min/maks'ın (kd) ortak gövdesidir: değerlerin en küçüğünü (buyuk yanlışsa) ya da en
// büyüğünü, değerin kendisini (tipi korunarak) döndürür. Tam'lar tam karşılaştırılır; sayı
// olmayan değer hatadır; NaN varsa sonuç NaN'dır. Boş girdide yok döner. liste, değerlerin
// tek bir liste argümanının elemanları olduğunu söyler (hata mesajı için).
func extreme(kd codes.Code, elems []Value, upper bool, listVal bool) (Value, error) {
	if len(elems) == 0 {
		return Null, nil
	}
	var picked Value
	for i, el := range elems {
		if !isNumber(el) {
			if listVal {
				return nil, ListElementError(CallPrefix(kd), i, numberType, el)
			}
			return nil, ArgTypeError(kd, i+1, el, numberTypes...)
		}
		if f, ok := el.(*FloatVal); ok && math.IsNaN(f.Val) {
			return f, nil
		}
		if picked == nil {
			picked = el
			continue
		}
		c, _ := compareNumbers(el, picked)
		if (upper && c > 0) || (!upper && c < 0) {
			picked = el
		}
	}
	return picked, nil
}

// rangeSteps bir aralığın gezilecek ilk ve son değerini ve adımını verir; bos doğruysa
// aralıkta hiç değer yoktur. Son değer dahil hesaplandığı için int64 sınırındaki aralıklar
// (0..=9223372036854775807) taşmadan gezilir.
func rangeSteps(r *RangeVal) (first, last, step int64, empty bool) {
	first, last, step = r.Start, r.End, 1
	switch {
	case r.Start < r.End:
		if !r.Inclusive {
			last = r.End - 1
		}
	case r.Start > r.End:
		step = -1
		if !r.Inclusive {
			last = r.End + 1
		}
	default: // Start == End
		empty = !r.Inclusive
	}
	return first, last, step, empty
}

// SliceBounds a..b (ya da a..=b) dilimini n uzunluklu bir kabın [bas, son) aralığına
// çevirir: negatif uç, indekste olduğu gibi sondan sayar (-1 son eleman); uçlar kabın
// sınırlarına kırpılır, bas > son ise dilim boştur.
func SliceBounds(r *RangeVal, n int) (begin, last int) {
	b, s := r.Start, r.End
	if b < 0 {
		b += int64(n)
	}
	if s < 0 {
		s += int64(n)
	}
	if r.Inclusive && s < math.MaxInt64 {
		s++
	}
	trim := func(x int64) int {
		if x < 0 {
			return 0
		}
		if x > int64(n) {
			return n
		}
		return int(x)
	}
	begin, last = trim(b), trim(s)
	if begin > last {
		begin = last
	}
	return begin, last
}

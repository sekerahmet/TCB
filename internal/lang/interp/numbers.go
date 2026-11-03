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
func floatToInt(name any, f float64) (int64, error) {
	if math.IsNaN(f) {
		return 0, messages.New(messages.H2667, name) // nan taşma değildir: sayı değildir
	}
	if math.IsInf(f, 0) || f >= intLimit || f < -intLimit {
		t := codes.NameRef(codes.TypeInt)
		return 0, messages.New(messages.H2484, name, (&FloatVal{Val: f}).Text(codes.Display()), t)
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
// Tam'dır ve taşma hata verir; bir Ondalık varsa sonuç Ondalık'tır, ilk Ondalık'tan önceki
// Tam'ların toplamından başlayıp soldan sağa telafili toplanır (compensated, 13.1). Sayı olmayan
// eleman hatadır. ad hata mesajındaki yerleşiğin adıdır.
func sumNumbers(name any, elems []Value) (Value, error) {
	var intNum int64
	var float compensated
	hasFloat := false
	for i, el := range elems {
		switch x := el.(type) {
		case *IntVal:
			if hasFloat {
				float.add(float64(x.Val))
				continue
			}
			s := intNum + x.Val
			if (s > intNum) != (x.Val > 0) {
				return nil, messages.New(messages.H2485, name, codes.NameRef(codes.TypeInt), intNum, x.Val, i+1, codes.NameRef(codes.TypeFloat))
			}
			intNum = s
		case *FloatVal:
			if !hasFloat {
				hasFloat = true
				float.add(float64(intNum))
			}
			float.add(x.Val)
		default:
			return nil, ListElementError(name, i, numberType, el)
		}
	}
	if hasFloat {
		return &FloatVal{Val: float.value()}, nil
	}
	return NewInt(intNum), nil
}

// compensated ondalık toplamın birikimidir (13.1): soldan sağa, Neumaier'in telafisiyle; her
// toplamanın yuvarlama hatası ayrıca biriktirilip sonda eklenir, sonuç öğelerin sırasından az
// etkilenir ([1e16, 1.0, -1e16] → 1.0). Çarpma yoktur: FMA'ya birleşecek işlem yok. Ara toplam
// sonlu değilse (TR_inf, TR_nan, taşma) sonuç düz toplamdır, IEEE 754 gibi ([1.0, inf] → inf,
// [inf, -inf] → nan).
type compensated struct{ sum, err float64 }

func (s *compensated) add(x float64) {
	t := s.sum + x
	if math.Abs(s.sum) >= math.Abs(x) {
		s.err += (s.sum - t) + x
	} else {
		s.err += (x - t) + s.sum
	}
	s.sum = t
}

func (s *compensated) value() float64 {
	if math.IsInf(s.sum, 0) || math.IsNaN(s.sum) {
		return s.sum
	}
	return s.sum + s.err
}

// ListElementError reports that element i (0-based) of a list has the wrong type.
func ListElementError(name any, i int, expected any, given Value) error {
	return messages.New(messages.H2486, name, i+1, expected, TypeAndSummary(given))
}

// extreme min/max'ın (kd) ortak gövdesidir: değerlerin en küçüğünü (buyuk yanlışsa) ya da en
// büyüğünü, değerin kendisini (tipi korunarak) döndürür. Tam'lar tam karşılaştırılır; sayı
// olmayan değer hatadır; NaN varsa sonuç NaN'dır. Boş girdide yok döner. liste, değerlerin
// tek bir liste argümanının elemanları olduğunu söyler (hata mesajı için).
func extreme(kd codes.Code, elems []Value, upper bool, listVal bool) (Value, error) {
	if len(elems) == 0 {
		return Null, nil
	}
	if _, ok := elems[0].(*TupleVal); ok {
		return extremeTuple(kd, elems, upper)
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
		if c == 0 {
			c = zeroSignOrder(el, picked) // -0.0 0.0'dan küçük sayılır (13.1, 13.10)
		}
		if (upper && c > 0) || (!upper && c < 0) {
			picked = el
		}
	}
	return picked, nil
}

// rangeSteps bir aralığın gezilecek ilk ve son değerini ve adımını verir; empty doğruysa
// aralıkta hiç değer yoktur: a >= b ise a..b, a > b ise a..=b boştur (3.8; geriye sayan aralık
// .reverse() ile yazılır). Son değer dahil hesaplandığı için int64 sınırındaki aralıklar
// (0..=9223372036854775807) taşmadan gezilir.
func rangeSteps(r *RangeVal) (first, last, step int64, empty bool) {
	first, last, step = r.Start, r.End, 1
	switch {
	case r.Start < r.End:
		if !r.Inclusive {
			last = r.End - 1
		}
	case r.Start > r.End:
		empty = true
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

// zeroSignOrder eşit iki sayının işarete göre sırasıdır: ikisi de sıfırsa -0.0 0.0'dan küçüktür
// (13.10'un sıralama sırası; TR_max(-0.0, 0.0) 0.0, TR_min(0.0, -0.0) -0.0). Tam sayı sıfırı artı
// sayılır. Öbür eşitliklerde 0.
func zeroSignOrder(a, b Value) int {
	neg := func(v Value) bool {
		f, ok := v.(*FloatVal)
		return ok && f.Val == 0 && math.Signbit(f.Val)
	}
	switch na, nb := neg(a), neg(b); {
	case na && !nb:
		return -1
	case nb && !na:
		return 1
	}
	return 0
}

package interp

import (
	"math"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// registerStats ortalama, standart_sapma ve ortanca yerleşiklerini kaydeder. Tensör
// yolu (ortalama(t, eksen: ...)) internal/stdlib/ml'dadır; o, bu yerleşiği sarar. Üçü
// de aynı kuralla girdi alır (13.1): tek bir liste ya da birden çok sayı; boş girdi ve sayı
// olmayan eleman hatadır (toplam gibi).
func (v *VM) registerStats() {
	v.RegisterNoNamed(codes.G410, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		elems, _, err := statsInput(codes.G410, args)
		if err != nil {
			return nil, err
		}
		return NumberMean(CallPrefix(codes.G410), elems)
	})

	// standart_sapma: sayıların popülasyon standart sapması (n'ye bölerek).
	v.RegisterNoNamed(codes.G411, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		nums, err := statsNumbers(codes.G411, messages.H2505, args)
		if err != nil {
			return nil, err
		}
		mean := 0.0
		for _, x := range nums {
			mean += x
		}
		mean /= float64(len(nums))
		variance := 0.0
		for _, x := range nums {
			d := x - mean
			variance += d * d
		}
		variance /= float64(len(nums))
		return &FloatVal{Val: math.Sqrt(variance)}, nil
	})

	// ortanca: çift sayıda elemanda ortadaki ikisinin ortalaması.
	v.RegisterNoNamed(codes.G412, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		sorted, err := statsNumbers(codes.G412, messages.H2506, args)
		if err != nil {
			return nil, err
		}
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		if len(sorted)%2 == 1 {
			return &FloatVal{Val: sorted[mid]}, nil
		}
		return &FloatVal{Val: (sorted[mid-1] + sorted[mid]) / 2.0}, nil
	})
}

// statsInput istatistik yerleşiğinin elemanlarını verir: tek bir liste argümanının
// elemanları (fromList) ya da argümanların kendileri. Argümansız çağrı hatadır.
func statsInput(kd codes.Code, args []Value) (elems []Value, fromList bool, err error) {
	if len(args) == 0 {
		return nil, false, ArgCountError(kd, AtLeast(1), 0, messages.Text(messages.H2304), messages.Text(messages.H2310))
	}
	if l, ok := args[0].(*ListVal); ok && len(args) == 1 {
		return l.Elements, true, nil
	}
	return args, false, nil
}

// statsNumbers elemanları sayıya çevirir (yeni bir dilime): boş girdi ve sayı olmayan
// eleman ortalama() ile aynı biçimde hatadır. emptyMessage boş girdide verilen, sonucun adını da anlatan tam cümlelik iletidir.
func statsNumbers(kd codes.Code, emptyMessage messages.Code, args []Value) ([]float64, error) {
	elems, fromList, err := statsInput(kd, args)
	if err != nil {
		return nil, err
	}
	if len(elems) == 0 {
		return nil, messages.New(emptyMessage, CallPrefix(kd))
	}
	nums := make([]float64, len(elems))
	for i, el := range elems {
		f, ok := ToFloat(el)
		switch {
		case !ok && fromList:
			return nil, ListElementError(CallPrefix(kd), i, numberType, el)
		case !ok:
			return nil, argTypeErrorByName(CallName(kd), i+1, numberType, el) // liste değil: argümanın kendisi
		}
		nums[i] = f
	}
	return nums, nil
}

package interp

import (
	"fmt"
	"math"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// registerIstatistik ortalama, standart_sapma ve ortanca yerleşiklerini kaydeder. Tensör
// yolu (ortalama(t, eksen: ...)) internal/stdlib/ml'dadır; o, bu yerleşiği sarar. Üçü
// de aynı kuralla girdi alır (13.1): tek bir liste ya da birden çok sayı; boş girdi ve sayı
// olmayan eleman hatadır (toplam gibi).
func (v *VM) registerStats() {
	v.RegisterNoNamed(codes.G410, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		elems, err := statsInput(codes.G410, args)
		if err != nil {
			return nil, err
		}
		return NumberMean(CallPrefix(codes.G410), elems)
	})

	// standart_sapma: sayıların popülasyon standart sapması (n'ye bölerek).
	v.RegisterNoNamed(codes.G411, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		nums, err := statsNumbers(codes.G411, "standart sapması", args)
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
		sorted, err := statsNumbers(codes.G412, "ortancası", args)
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

// istatistikGirdisi istatistik yerleşiğinin elemanlarını verir: tek bir liste argümanının
// elemanları ya da argümanların kendileri. Argümansız çağrı hatadır.
func statsInput(kd codes.Code, args []Value) ([]Value, error) {
	if len(args) == 0 {
		return nil, ArgCountError(kd, "en az 1", 0, "liste", "a, b, ...")
	}
	if l, ok := args[0].(*ListVal); ok && len(args) == 1 {
		return l.Elements, nil
	}
	return args, nil
}

// istatistikSayilari elemanları sayıya çevirir (yeni bir dilime): boş girdi ve sayı olmayan
// eleman ortalama() ile aynı biçimde hatadır. ne hata mesajındaki sonucun adıdır.
func statsNumbers(kd codes.Code, ne string, args []Value) ([]float64, error) {
	elems, err := statsInput(kd, args)
	if err != nil {
		return nil, err
	}
	if len(elems) == 0 {
		return nil, fmt.Errorf("%s: boş listenin %s yoktur", CallPrefix(kd), ne)
	}
	nums := make([]float64, len(elems))
	for i, el := range elems {
		f, ok := ToFloat(el)
		if !ok {
			return nil, ListElementError(CallPrefix(kd), i, numberType, el)
		}
		nums[i] = f
	}
	return nums, nil
}

package yurut

import (
	"math"
	"sort"

	"tr_dili/internal/dil/kod"
)

// registerIstatistik ortalama, standart_sapma ve ortanca yerleşiklerini kaydeder. Tensör
// yolu (ortalama(t, eksen: ...)) internal/motor/ogrenme'dadır; o, bu yerleşiği sarar.
func (v *VM) registerIstatistik() {
	// ortalama(liste) ya da ortalama(a, b, ...): boş girdi ve sayı olmayan eleman hatadır
	// (toplam gibi).
	v.AdlisizKaydet(kod.G410, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) == 0 {
			return nil, ArgumanSayisiHatasi("ortalama", "en az 1", 0, "ortalama(liste) ya da ortalama(a, b, ...)")
		}
		if l, ok := args[0].(*ListVal); ok && len(args) == 1 {
			return SayiOrtalamasi("ortalama()", l.Elements)
		}
		return SayiOrtalamasi("ortalama()", args)
	})

	// standart_sapma: sayıların popülasyon standart sapması (n'ye bölerek).
	v.AdlisizKaydet(kod.G411, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		nums := ExtractNumericSlice(args)
		if len(nums) == 0 {
			return &FloatVal{Val: 0}, nil
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
	v.AdlisizKaydet(kod.G412, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		nums := ExtractNumericSlice(args)
		if len(nums) == 0 {
			return &FloatVal{Val: 0}, nil
		}
		sorted := make([]float64, len(nums))
		copy(sorted, nums)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		if len(sorted)%2 == 1 {
			return &FloatVal{Val: sorted[mid]}, nil
		}
		return &FloatVal{Val: (sorted[mid-1] + sorted[mid]) / 2.0}, nil
	})
}

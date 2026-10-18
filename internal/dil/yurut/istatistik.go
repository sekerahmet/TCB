package yurut

import (
	"fmt"
	"math"
	"sort"

	"tr_dili/internal/dil/kod"
)

// registerIstatistik ortalama, standart_sapma ve ortanca yerleşiklerini kaydeder. Tensör
// yolu (ortalama(t, eksen: ...)) internal/motor/ogrenme'dadır; o, bu yerleşiği sarar. Üçü
// de aynı kuralla girdi alır (13.1): tek bir liste ya da birden çok sayı; boş girdi ve sayı
// olmayan eleman hatadır (toplam gibi).
func (v *VM) registerIstatistik() {
	v.AdlisizKaydet(kod.G410, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		elems, err := istatistikGirdisi(kod.G410, args)
		if err != nil {
			return nil, err
		}
		return SayiOrtalamasi(Cagri(kod.G410), elems)
	})

	// standart_sapma: sayıların popülasyon standart sapması (n'ye bölerek).
	v.AdlisizKaydet(kod.G411, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		nums, err := istatistikSayilari(kod.G411, "standart sapması", args)
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
	v.AdlisizKaydet(kod.G412, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		sorted, err := istatistikSayilari(kod.G412, "ortancası", args)
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
func istatistikGirdisi(kd kod.Kod, args []Value) ([]Value, error) {
	if len(args) == 0 {
		return nil, ArgumanSayisiHatasi(kd, "en az 1", 0, "liste", "a, b, ...")
	}
	if l, ok := args[0].(*ListVal); ok && len(args) == 1 {
		return l.Elements, nil
	}
	return args, nil
}

// istatistikSayilari elemanları sayıya çevirir (yeni bir dilime): boş girdi ve sayı olmayan
// eleman ortalama() ile aynı biçimde hatadır. ne hata mesajındaki sonucun adıdır.
func istatistikSayilari(kd kod.Kod, ne string, args []Value) ([]float64, error) {
	elems, err := istatistikGirdisi(kd, args)
	if err != nil {
		return nil, err
	}
	if len(elems) == 0 {
		return nil, fmt.Errorf("%s: boş listenin %s yoktur", Cagri(kd), ne)
	}
	nums := make([]float64, len(elems))
	for i, el := range elems {
		f, ok := ToFloat(el)
		if !ok {
			return nil, ListeElemaniHatasi(Cagri(kd), i, sayiTipi, el)
		}
		nums[i] = f
	}
	return nums, nil
}

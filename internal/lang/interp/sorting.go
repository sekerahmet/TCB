package interp

import (
	"cmp"
	"math"
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// sortValues değerleri sırala() kuralıyla kararlı (eşitlerin sırası korunarak) sıralar.
// Liste ya yalnızca sayılardan (Tam ve Ondalık) ya da yalnızca metinlerden oluşmalıdır;
// başka bir tip ya da sayı ile metin karışımı karşılaştırılamaz ve hata verir. Sayılar
// tam olarak karşılaştırılır (iki Tam int64 olarak; Tam ile Ondalık Ondalığa yuvarlanmadan),
// NaN'lar sona konur. Metinler karakter kodu sırasıyla dizilir. ad hata mesajındaki adıdır.
func sortValues(name any, d []Value) error {
	if len(d) < 2 {
		if len(d) == 1 && !isNumber(d[0]) {
			if _, text := d[0].(*StringVal); !text {
				return sortTypeError(name, d[0], nil)
			}
		}
		return nil
	}
	switch d[0].(type) {
	case *IntVal, *FloatVal:
		// Hızlı yol: hepsi Tam (en sık durum: anahtarlar bir kez çıkarılır)
		ints := true
		for _, x := range d {
			switch x.(type) {
			case *IntVal:
			case *FloatVal:
				ints = false
			default:
				return sortTypeError(name, d[0], x)
			}
		}
		if ints {
			type keyed struct {
				k int64
				v Value
			}
			pair := make([]keyed, len(d))
			for i, x := range d {
				pair[i] = keyed{x.(*IntVal).Val, x}
			}
			// Eşit Tam'lar ayırt edilemez; kararsız sıralama aynı sonucu verir.
			slices.SortFunc(pair, func(a, b keyed) int { return cmp.Compare(a.k, b.k) })
			for i := range pair {
				d[i] = pair[i].v
			}
			return nil
		}
		slices.SortStableFunc(d, func(a, b Value) int {
			c, ok := compareNumbers(a, b)
			if ok {
				return c
			}
			// NaN'lar sona (kendi aralarında eşit)
			an, bn := isNaN(a), isNaN(b)
			switch {
			case an && bn:
				return 0
			case an:
				return 1
			}
			return -1
		})
		return nil
	case *StringVal:
		for _, x := range d {
			if _, ok := x.(*StringVal); !ok {
				return sortTypeError(name, d[0], x)
			}
		}
		slices.SortStableFunc(d, func(a, b Value) int { return cmp.Compare(a.(*StringVal).Val, b.(*StringVal).Val) })
		return nil
	}
	return sortTypeError(name, d[0], nil)
}

func isNaN(v Value) bool {
	f, ok := v.(*FloatVal)
	return ok && math.IsNaN(f.Val)
}

// sortTypeError sıralanamayan bir listenin hatasıdır: ilk elemanın tipi sıralanamıyorsa
// (b nil) ya da iki elemanın tipleri birbiriyle karşılaştırılamıyorsa.
func sortTypeError(name any, a, b Value) error {
	if b == nil {
		return messages.New(messages.H2492, name, TypeAndSummary(a))
	}
	return messages.New(messages.H2493, name, TypeAndSummary(a), TypeAndSummary(b))
}

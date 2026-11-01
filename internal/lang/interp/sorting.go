package interp

import (
	"cmp"
	"math"
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// listSort .TR_sort() ve .TR_sort(f) metotlarının gövdesidir (13.3): listenin sıralanmış yeni bir
// kopyasını verir. Anahtar fonksiyonu f tek parametre alır ve her öğe için bir kez çağrılır; öğeler
// anahtarların sıralama sırasıyla (13.10) kararlı dizilir. f'nin dolaşılan listeye öğe eklemesi ya da
// ondan öğe çıkarması 5.7'deki gibi hatadır.
func listSort(y *VM, inst Value, args []Value, line int) (Value, error) {
	list := inst.(*ListVal)
	n := len(list.Elements)
	copied := make([]Value, n)
	copy(copied, list.Elements)
	if len(args) == 0 {
		if err := sortValues(CallPrefix(codes.MemSort), copied); err != nil {
			return nil, err
		}
		return &ListVal{Elements: copied}, nil
	}
	if fn, ok := args[0].(*FuncVal); ok && len(fn.Parameters) != 1 {
		return nil, messages.New(messages.H2900, len(fn.Parameters))
	}
	keys := make([]Value, n)
	for i, item := range copied {
		key, err := y.ApplyFunction(args[0], []Value{item}, nil, line)
		if err != nil {
			return nil, err
		}
		if err := sizeChanged(list, n, line); err != nil {
			return nil, err
		}
		keys[i] = key
	}
	if a, b := sortFault(keys); a != nil {
		if b == nil {
			return nil, messages.New(messages.H2902, TypeAndSummary(a))
		}
		return nil, messages.New(messages.H2903, TypeAndSummary(a), TypeAndSummary(b))
	}
	sortByKeys(keys, copied)
	return &ListVal{Elements: copied}, nil
}

// sortValues değerleri 13.10'un sıralama sırasıyla kararlı (eşitlerin sırası korunarak) sıralar.
// Liste ya yalnızca sayılardan (Tam ve Ondalık), ya yalnızca metinlerden ya da yalnızca öğeleri
// sıralanabilen demetlerden oluşmalıdır; başka bir tip ya da karışım karşılaştırılamaz ve hata verir.
// Sayılar tam olarak karşılaştırılır (iki Tam int64 olarak; Tam ile Ondalık Ondalığa yuvarlanmadan),
// -0.0 0.0'dan önce gelir, NaN'lar sona konur. ad hata mesajındaki adıdır.
func sortValues(name any, d []Value) error {
	if a, b := sortFault(d); a != nil {
		return sortTypeError(name, a, b)
	}
	if allInts(d) {
		// Hızlı yol: hepsi Tam (en sık durum: anahtarlar bir kez çıkarılır)
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
	slices.SortStableFunc(d, sortOrder)
	return nil
}

// sortByKeys vals'i keys'in (aynı sırada, sortFault'tan geçmiş) sıralama sırasıyla kararlı dizer.
func sortByKeys(keys, vals []Value) {
	type keyed struct{ k, v Value }
	pair := make([]keyed, len(vals))
	for i := range vals {
		pair[i] = keyed{keys[i], vals[i]}
	}
	slices.SortStableFunc(pair, func(a, b keyed) int { return sortOrder(a.k, b.k) })
	for i := range pair {
		vals[i] = pair[i].v
	}
}

// sortFault sıralanamayan bir listenin hatalı öğeleridir (13.10): ilk öğe sıralanamıyorsa (a, nil),
// iki öğe birbiriyle karşılaştırılamıyorsa (a, b); liste sıralanabilirse (nil, nil).
func sortFault(d []Value) (a, b Value) {
	if len(d) == 0 {
		return nil, nil
	}
	first := d[0]
	var same func(Value) bool
	switch f := first.(type) {
	case *IntVal, *FloatVal:
		same = isNumber
	case *StringVal:
		same = func(x Value) bool { _, ok := x.(*StringVal); return ok }
	case *TupleVal:
		// öğeleri sıralanabilen demetler (3.6a): öğe çiftleri ilk demetinkilerle sıralanabilir
		same = func(x Value) bool { t, ok := x.(*TupleVal); return ok && orderedPair("<", f, t) == nil }
	default:
		return first, nil
	}
	for _, x := range d {
		if !same(x) {
			if x == first {
				return first, nil
			}
			return first, x
		}
	}
	return nil, nil
}

func allInts(d []Value) bool {
	for _, x := range d {
		if _, ok := x.(*IntVal); !ok {
			return false
		}
	}
	return true
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

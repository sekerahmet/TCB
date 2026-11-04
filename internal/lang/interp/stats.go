package interp

import (
	"math"
	"math/big"
	"slices"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// registerStats ortalama, standart_sapma ve ortanca yerleşiklerini kaydeder. Tensör
// yolu (ortalama(t, eksen: ...)) internal/stdlib/ml'dadır; o, bu yerleşiği sarar. Üçü
// de aynı kuralla girdi alır (13.1): yalnız liste, değerleri tek tek almaz; boş girdi ve sayı
// olmayan eleman hatadır (toplam gibi).
func (v *VM) registerStats() {
	v.RegisterNoNamed(codes.FnMean, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		elems, err := statsInput(codes.FnMean, args)
		if err != nil {
			return nil, err
		}
		return NumberMean(CallPrefix(codes.FnMean), elems)
	})

	// standart_sapma: örneklem standart sapması (13.1), n−1'e böler; iki öğeden azda hatadır. Tam
	// sayı listesinde kesindir (exactStd). Öbür listede iki geçişlidir: önce ortalama (TR_mean'inki),
	// sonra farkların karelerinin telafili toplamı. Karenin açık dönüşümü çarpmayı toplamaya
	// birleştirmez (FMA'sız; Go dil tanımı: açık dönüşüm hedef tipin hassasiyetine yuvarlar).
	v.RegisterNoNamed(codes.FnStd, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		nums, err := statsNumbers(codes.FnStd, messages.H2505, args)
		if err != nil {
			return nil, err
		}
		if len(nums) < 2 {
			return nil, messages.New(messages.H2620, CallPrefix(codes.FnStd))
		}
		elems, _ := statsInput(codes.FnStd, args)
		if ints, ok := integers(elems); ok {
			return &FloatVal{Val: exactStd(ints)}, nil
		}
		mean, err := numberMean(CallPrefix(codes.FnStd), elems)
		if err != nil {
			return nil, err
		}
		var squares compensated
		for _, x := range nums {
			d := x - mean
			squares.add(float64(d * d))
		}
		return &FloatVal{Val: math.Sqrt(squares.value() / float64(len(nums)-1))}, nil
	})

	// ortanca: çift sayıda elemanda ortadaki ikisinin ortalaması; tam sayı listesinde kesindir
	// (exactMedian). Listede NaN varsa NaN'dır (13.1), nerede olursa olsun: sıralama NaN'ı başa alır.
	v.RegisterNoNamed(codes.FnMedian, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		sorted, err := statsNumbers(codes.FnMedian, messages.H2506, args)
		if err != nil {
			return nil, err
		}
		elems, _ := statsInput(codes.FnMedian, args)
		if ints, ok := integers(elems); ok {
			return &FloatVal{Val: exactMedian(ints)}, nil
		}
		sort.Float64s(sorted)
		if math.IsNaN(sorted[0]) {
			return &FloatVal{Val: math.NaN()}, nil
		}
		mid := len(sorted) / 2
		if len(sorted)%2 == 1 {
			return &FloatVal{Val: sorted[mid]}, nil
		}
		return &FloatVal{Val: (sorted[mid-1] + sorted[mid]) / 2.0}, nil
	})
}

// NumberMean returns the arithmetic mean of numbers as Ondalık; empty input and non-number
// elements are errors (used by the ortalama(liste) builtin).
func NumberMean(name any, elems []Value) (Value, error) {
	m, err := numberMean(name, elems)
	if err != nil {
		return nil, err
	}
	return &FloatVal{Val: m}, nil
}

// numberMean ortalamanın kendisidir (13.1; TR_std'nin ilk geçişi de): tam sayı listesinde kesindir
// (exactMean), öbür listede toplam TR_sum'ınki gibi telafilidir (compensated).
func numberMean(name any, elems []Value) (float64, error) {
	if len(elems) == 0 {
		return 0, messages.New(messages.H2477, name)
	}
	if ints, ok := integers(elems); ok {
		return exactMean(ints), nil
	}
	var s compensated
	for i, el := range elems {
		f, ok := ToFloat(el)
		if !ok {
			return 0, ListElementError(name, i, numberType, el)
		}
		s.add(f)
	}
	return s.value() / float64(len(elems)), nil
}

// integers listenin bütün öğeleri Tam ise değerlerini verir. Yalnız bu liste kesin yoldan gider
// (13.1): Ondalık ve karışık listenin yolu ayrıdır.
func integers(elems []Value) ([]int64, bool) {
	ints := make([]int64, len(elems))
	for i, el := range elems {
		n, ok := el.(*IntVal)
		if !ok {
			return nil, false
		}
		ints[i] = n.Val
	}
	return ints, true
}

// exactSum tam sayıların toplamıdır; ara değer taşmaz.
func exactSum(ints []int64) *big.Int {
	sum, x := new(big.Int), new(big.Int)
	for _, n := range ints {
		sum.Add(sum, x.SetInt64(n))
	}
	return sum
}

// exactInFloat, n'nin ondalıkta kesin durduğunu söyler: |n| ≤ 2^53 (yalnızca bu sınır sınanır).
func exactInFloat(n *big.Int) bool { return n.BitLen() <= 53 }

// nearestFloat n/m'nin en yakın ondalığıdır, eşitte çift olana: kesin değer bir kez yuvarlanır (13.1).
// İki sayı da ondalıkta kesinse IEEE 754 bölmesi tam bunu verir (küçük listenin yolu); değilse hesap
// yalnız tam sayıdır. İkisi de her makinede aynıdır.
func nearestFloat(n, m *big.Int) float64 {
	if exactInFloat(n) && exactInFloat(m) {
		return float64(n.Int64()) / float64(m.Int64())
	}
	f, _ := new(big.Rat).SetFrac(n, m).Float64()
	return f
}

// exactMean tam sayı listesinin ortalamasıdır (13.1): kesin değerin en yakın ondalığı.
func exactMean(ints []int64) float64 {
	return nearestFloat(exactSum(ints), big.NewInt(int64(len(ints))))
}

// exactMedian tam sayı listesinin ortancasıdır (13.1): sıralama kesindir, çift sayıda öğede ortadaki
// ikisinin toplamı taşmaz ve bir kez yuvarlanır.
func exactMedian(ints []int64) float64 {
	slices.Sort(ints)
	mid := len(ints) / 2
	if len(ints)%2 == 1 {
		return float64(ints[mid])
	}
	return nearestFloat(exactSum(ints[mid-1:mid+1]), big.NewInt(2))
}

// exactStd tam sayı listesinin örneklem sapmasıdır (13.1): varyans kesir olarak kesindir, karekök de
// tam sayıyla alınır; varyansı ondalığa çevirip kök almak iki yuvarlama olurdu.
func exactStd(ints []int64) float64 {
	n := big.NewInt(int64(len(ints)))
	sum, squares := exactSum(ints), new(big.Int)
	x := new(big.Int)
	for _, v := range ints {
		x.SetInt64(v)
		squares.Add(squares, x.Mul(x, x))
	}
	// varyans = (n·Σx² − (Σx)²) / (n·(n−1))
	top := new(big.Int).Mul(n, squares)
	top.Sub(top, sum.Mul(sum, sum))
	bottom := new(big.Int).Sub(n, big.NewInt(1))
	return sqrtOfFraction(top, bottom.Mul(bottom, n))
}

// sqrtBits kökün tam sayı olarak tutulan bit sayısıdır: float64'ün 53 bitinin iki katı ve üç koruma
// biti; bundan sonra tek yönlü yuvarlama sonucu değiştirmez.
const sqrtBits = 2*53 + 3

// sqrtOfFraction √(n/m)'nin en yakın ondalığıdır (n ≥ 0, m > 0), bir kez yuvarlanır. Yöntem CPython'un
// statistics._float_sqrt_of_frac'ıdır: kök sqrtBits/2 bite kadar tam sayı olarak alınır, tek sayıya
// yuvarlanır (round to odd); en yakın ondalığa çevirmek o zaman kesin kökü çevirmekle aynıdır. Kök 64
// bite sığar, dönüşüm tek yuvarlamadır; 2'nin kuvvetiyle çarpmak kesindir.
func sqrtOfFraction(n, m *big.Int) float64 {
	shift := (n.BitLen() - m.BitLen() - sqrtBits) >> 1 // aşağı yuvarlayan bölme
	var root *big.Int
	if shift >= 0 {
		root = sqrtToOdd(n, new(big.Int).Lsh(m, uint(2*shift)))
	} else {
		root = sqrtToOdd(new(big.Int).Lsh(n, uint(-2*shift)), m)
	}
	return math.Ldexp(float64(root.Int64()), shift)
}

// sqrtToOdd √(n/m)'nin tam kısmıdır; kök kesin değilse son biti 1 yapılır.
func sqrtToOdd(n, m *big.Int) *big.Int {
	root := new(big.Int).Quo(n, m)
	root.Sqrt(root)
	square := new(big.Int).Mul(root, root)
	if square.Mul(square, m).Cmp(n) != 0 {
		root.SetBit(root, 0, 1)
	}
	return root
}

// statsInput istatistik yerleşiğinin listesinin elemanlarını verir (13.1): yalnız tek bir liste
// alır; ileti değerleri listeye koyan yazımı gösterir.
func statsInput(kd codes.Code, args []Value) ([]Value, error) {
	if len(args) != 1 {
		return nil, ArgCountErrorWriting(kd, 1, len(args), "[a, b]")
	}
	l, ok := args[0].(*ListVal)
	if !ok {
		return nil, ArgTypeError(kd, 1, args[0], codes.TypeList)
	}
	return l.Elements, nil
}

// statsNumbers elemanları sayıya çevirir (yeni bir dilime): boş girdi ve sayı olmayan
// eleman ortalama() ile aynı biçimde hatadır. emptyMessage boş girdide verilen, sonucun adını da anlatan tam cümlelik iletidir.
func statsNumbers(kd codes.Code, emptyMessage messages.Code, args []Value) ([]float64, error) {
	elems, err := statsInput(kd, args)
	if err != nil {
		return nil, err
	}
	if len(elems) == 0 {
		return nil, messages.New(emptyMessage, CallPrefix(kd))
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

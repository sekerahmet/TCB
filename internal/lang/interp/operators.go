package interp

import (
	"math"
	"math/big"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İşleç kodları: ayrıştırılmış işleç metinleri derleme anında bu tamsayılara çevrilir; böylece
// her işlemde metin karşılaştırması yapılmaz.
const (
	_ uint8 = iota // 0 kullanılmaz
	opUnknown
	opAdd
	opSub
	opMul
	opDiv
	opMod
	opPow
	opLt
	opGt
	opLe
	opGe
	opEq
	opNe
	opAnd
	opOr
	opCoalesce // ??
	opMatMul
	opAssign   // = (düz atama)
	opFloorDiv // //
	opBitAnd   // &
	opBitOr    // |
	opBitXor   // ^
	opShl      // <<
	opShr      // >>
)

// opCode bir işleç metninin kodunu döndürür. Bileşik atamalar (+=, ...) temel
// işlecin koduna, düz atama opAta'ya çevrilir.
func opCode(op string) uint8 {
	switch op {
	case "+", "+=":
		return opAdd
	case "-", "-=":
		return opSub
	case "*", "*=":
		return opMul
	case "/", "/=":
		return opDiv
	case "%", "%=":
		return opMod
	case "//", "//=":
		return opFloorDiv
	case "&", "&=":
		return opBitAnd
	case "|", "|=":
		return opBitOr
	case "^", "^=":
		return opBitXor
	case "<<", "<<=":
		return opShl
	case ">>", ">>=":
		return opShr
	case "**", "**=":
		return opPow
	case "<":
		return opLt
	case ">":
		return opGt
	case "<=":
		return opLe
	case ">=":
		return opGe
	case "==":
		return opEq
	case "!=":
		return opNe
	case "&&":
		return opAnd
	case "||":
		return opOr
	case "??":
		return opCoalesce
	case "@":
		return opMatMul
	case "=":
		return opAssign
	}
	return opUnknown
}

// assignOpText bileşik atamanın kullandığı ikili işlecin metnini döndürür (hata mesajları için).
func assignOpText(codeVal uint8) string {
	switch codeVal {
	case opAdd:
		return "+"
	case opSub:
		return "-"
	case opMul:
		return "*"
	case opDiv:
		return "/"
	case opFloorDiv:
		return "//"
	case opMod:
		return "%"
	case opPow:
		return "**"
	case opBitAnd:
		return "&"
	case opBitOr:
		return "|"
	case opBitXor:
		return "^"
	case opShl:
		return "<<"
	case opShr:
		return ">>"
	}
	return ""
}

// evalInfixKod ikili işlemi işleç koduyla yapar; op yalnızca hata mesajları içindir.
func (v *VM) evalInfixKod(codeVal uint8, op string, left Value, right Value, line int) (Value, error) {
	// Hızlı yol: Tam ve Ondalık sayılar. Sayılar işleç kancası uygulamadığından bu denetim
	// kancalardan önce yapılabilir.
	switch l := left.(type) {
	case *IntVal:
		switch r := right.(type) {
		case *IntVal:
			if val, ok, err := intOp(codeVal, op, l.Val, r.Val, line); ok {
				return val, err
			}
		case *FloatVal:
			if val, ok := mixedCompare(codeVal, left, right); ok {
				return val, nil
			}
			if val, ok, err := floatOp(codeVal, float64(l.Val), r.Val, line); ok {
				return val, err
			}
		}
	case *FloatVal:
		switch r := right.(type) {
		case *IntVal:
			if val, ok := mixedCompare(codeVal, left, right); ok {
				return val, nil
			}
			if val, ok, err := floatOp(codeVal, l.Val, float64(r.Val), line); ok {
				return val, err
			}
		case *FloatVal:
			if val, ok, err := floatOp(codeVal, l.Val, r.Val, line); ok {
				return val, err
			}
		}
	case *StringVal:
		// Metin + Metin (aşağıdaki genel metin birleştirmeyle aynı sonuç; arayüz denetimleri atlanır)
		if r, ok := right.(*StringVal); ok && codeVal == opAdd {
			return &StringVal{Val: l.Val + r.Val}, nil
		}
	}

	// Değerlerin kendi işleç kancaları (ör. tensör @ tensör, 2 - tensör, tensör < 0);
	// hatalarına satır bilgisi eklenir.
	switch codeVal {
	case opEq, opNe:
	default:
		if val, ok, err := v.hookOperator(op, left, right, line); ok {
			return val, err
		}
	}

	if codeVal == opEq || codeVal == opNe {
		return equalityResult(codeVal == opEq, op, left, right, line)
	}

	if val, ok, err := sequenceOp(codeVal, left, right, line); ok {
		return val, err
	}

	if bitOperator(codeVal) {
		return nil, messages.At(line, 0, messages.New(messages.H2607, op, left.Type(), right.Type()))
	}
	return nil, messages.At(line, 0, messages.New(messages.H2469, left.Type(), right.Type(), op))
}

// floatBase a'nın ondalık yazımıdır, '**'in tabanı olarak: eksi taban paranteze alınır, çünkü
// '-2.0 ** -2' 4.1'e göre '-(2.0 ** -2)' okunur (H2603'ün önerisi kopyalanınca doğru sonuç vermeli).
func floatBase(a int64) string { return powBase(a, ".0") }

// powBase a'yı '**'in tabanı olarak yazar (suffix: ".0" ondalık yazım için).
func powBase(a int64, suffix string) string {
	if a < 0 {
		return "(" + strconv.FormatInt(a, 10) + suffix + ")"
	}
	return strconv.FormatInt(a, 10) + suffix
}

// intOp iki Tam sayı arasındaki işlemi yapar; işleç burada ele alınmıyorsa ok=false döner.
// Hesap intFastFunc ile ortaktır; burada yalnızca hızlı yolun düştüğü durumun hatası ayrılır.
func intOp(codeVal uint8, op string, a, b int64, line int) (Value, bool, error) {
	if fast := intFastFunc(codeVal); fast != nil {
		if val, ok := fast(a, b); ok {
			return val, true, nil
		}
		switch {
		case (codeVal == opDiv || codeVal == opFloorDiv) && b == 0:
			return nil, true, messages.At(line, 0, messages.New(messages.H2470))
		case codeVal == opMod:
			return nil, true, messages.At(line, 0, messages.New(messages.H2471))
		case codeVal == opShl || codeVal == opShr:
			return nil, true, messages.At(line, 0, messages.New(messages.H2606, op, b))
		}
		return nil, true, overflowError(line, a, op, b)
	}
	if codeVal == opPow {
		if b < 0 {
			// İki TR_Int'in üssü TR_Int'tir; eksi üs ondalık yazımla istenir (4.3).
			return nil, true, messages.At(line, 0, messages.New(messages.H2603, powBase(a, ""), b, floatBase(a)))
		}
		res, ok := intPow(a, b)
		if !ok {
			return nil, true, messages.At(line, 0, messages.New(messages.H2472, ValInt, a, b))
		}
		return NewInt(res), true, nil
	}
	return nil, false, nil
}

// floatOp Ondalık (veya Tam + Ondalık) sayı işlemlerini yapar; işleç burada ele
// alınmıyorsa ok=false döner.
func floatOp(codeVal uint8, lf, rf float64, line int) (Value, bool, error) {
	switch codeVal {
	case opAdd:
		return &FloatVal{Val: lf + rf}, true, nil
	case opSub:
		return &FloatVal{Val: lf - rf}, true, nil
	case opMul:
		return &FloatVal{Val: lf * rf}, true, nil
	case opDiv:
		if rf == 0 {
			return nil, true, messages.At(line, 0, messages.New(messages.H2470))
		}
		return &FloatVal{Val: lf / rf}, true, nil
	case opMod:
		if rf == 0 {
			return nil, true, messages.At(line, 0, messages.New(messages.H2471))
		}
		_, mod := floatDivMod(lf, rf)
		return &FloatVal{Val: mod}, true, nil
	case opFloorDiv:
		if rf == 0 {
			return nil, true, messages.At(line, 0, messages.New(messages.H2470))
		}
		div, _ := floatDivMod(lf, rf)
		return &FloatVal{Val: div}, true, nil
	case opPow:
		// Tanımsız üs sessizce TR_nan ya da sonsuz olmaz (4.3).
		if lf == 0 && rf < 0 {
			return nil, true, messages.At(line, 0, messages.New(messages.H2604, numberText(lf), numberText(rf)))
		}
		if lf < 0 && !math.IsInf(rf, 0) && !math.IsNaN(rf) && rf != math.Trunc(rf) {
			return nil, true, messages.At(line, 0, messages.New(messages.H2605, numberText(lf), numberText(rf)))
		}
		return &FloatVal{Val: math.Pow(lf, rf)}, true, nil
	case opLt:
		return BoolToVal(lf < rf), true, nil
	case opGt:
		return BoolToVal(lf > rf), true, nil
	case opLe:
		return BoolToVal(lf <= rf), true, nil
	case opGe:
		return BoolToVal(lf >= rf), true, nil
	case opEq:
		return BoolToVal(lf == rf), true, nil
	case opNe:
		return BoolToVal(lf != rf), true, nil
	}
	return nil, false, nil
}

// trueDiv iki tam sayının bölümünü en yakın ondalığa yuvarlar (4.2). 2^53'e kadar iki işlenen de
// ondalıkta tamdır ve IEEE bölmesi doğru yuvarlar; daha büyüklerde kesir tam hesaplanır (Python'un
// tam sayı bölmesi gibi), yoksa iki yuvarlama birikirdi.
func trueDiv(a, b int64) float64 {
	const exact = 1 << 53
	if -exact <= a && a <= exact && -exact <= b && b <= exact {
		return float64(a) / float64(b)
	}
	f, _ := new(big.Rat).SetFrac(big.NewInt(a), big.NewInt(b)).Float64()
	return f
}

// floatDivMod ondalığın tam bölümü ve kalanıdır (4.2a), Python'un float_divmod'u gibi: kalan
// bölenin işaretini alır, bölüm kalanla tutarlı olacak biçimde aşağı yuvarlanır (1 // 0.1 = 9.0,
// 1 % 0.1 ≈ 0.1). b sıfır değildir.
func floatDivMod(a, b float64) (div, mod float64) {
	mod = math.Mod(a, b)
	div = (a - mod) / b
	if mod != 0 {
		if (b < 0) != (mod < 0) {
			mod += b
			div--
		}
	} else {
		mod = math.Copysign(0, b)
	}
	if div != 0 {
		floor := math.Floor(div)
		if div-floor > 0.5 {
			floor++
		}
		div = floor
	} else {
		div = math.Copysign(0, a/b)
	}
	return div, mod
}

// numberText bir sayının iletideki metnidir (değerin metniyle aynı: 0.0, -8.0).
func numberText(f float64) string { return (&FloatVal{Val: f}).Inspect() }

// bitOperator codeVal bit işleçlerinden biri mi (4.12: yalnız TR_Int).
func bitOperator(codeVal uint8) bool {
	switch codeVal {
	case opBitAnd, opBitOr, opBitXor, opShl, opShr:
		return true
	}
	return false
}

// overflowError int64 sınırını aşan tam sayı işlemleri için hata üretir (sessizce sarmak yerine).
func overflowError(line int, a int64, op string, b int64) error {
	return messages.At(line, 0, messages.New(messages.H2473, ValInt, a, op, b))
}

func ToFloat(v Value) (float64, bool) {
	switch n := v.(type) {
	case *IntVal:
		return float64(n.Val), true
	case *FloatVal:
		return n.Val, true
	default:
		return 0, false
	}
}

func BoolToVal(b bool) Value {
	if b {
		return True
	}
	return False
}

// equalityResult == (esit) ya da != işleminin sonucudur. Tip değeri metinle
// karşılaştırılamaz (3.11): sessizce yanlış dönmek yerine hata verir.
func equalityResult(equal bool, op string, left, right Value, line int) (Value, error) {
	if err := typeTextError(op, left, right); err != nil {
		return nil, messages.At(line, 0, err)
	}
	if (isFunction(left) || isFunction(right)) && left.Type() != ValNull && right.Type() != ValNull {
		// Fonksiyonlar karşılaştırılamaz (4.5): kimlikle karşılaştırmak görünüşü aynı iki
		// fonksiyonu eşit saymaz, ajanı yanıltır. Yoklukla karşılaştırma isteğe bağlı değeri
		// açar (3.5: 'f == TR_none'), serbesttir.
		return nil, messages.At(line, 0, messages.New(messages.H2613, op))
	}
	return BoolToVal(ValuesEqual(left, right) == equal), nil
}

// isFunction x bir fonksiyon değeri mi: kullanıcı fonksiyonu, ok fonksiyonu, bağlı metot ya da
// yerleşik.
func isFunction(x Value) bool {
	switch x.(type) {
	case *FuncVal, *BuiltinVal:
		return true
	}
	return false
}

func ValuesEqual(a, b Value) bool {
	return valuesEqual(a, b, 0)
}

// equalityDepth iç içe kap karşılaştırmasının en fazla derinliğidir; kendini içeren
// kaplarda (l == l dışında) sonsuz özyinelemeyi önler, aşılırsa eşit sayılmaz.
const equalityDepth = 10000

func valuesEqual(a, b Value, nestDepth int) bool {
	if a.Type() == ValNull && b.Type() == ValNull {
		return true
	}
	if ai, ok := a.(*IntVal); ok {
		if bi, ok := b.(*IntVal); ok {
			return ai.Val == bi.Val
		}
	}
	// A Tam and an Ondalık are compared exactly; converting the Tam to Ondalık would make
	// 9007199254740993 == 9007199254740992.0 true. NaN equals nothing.
	if c, ok := compareNumbers(a, b); ok {
		return c == 0
	}
	if _, ok1 := ToFloat(a); ok1 {
		if _, ok2 := ToFloat(b); ok2 {
			return false
		}
	}
	// Değerin kendi eşitlik kuralı (ör. tensör: şekil ve tüm elemanlar)
	if e, ok := a.(Equaler); ok {
		return e.Equal(b)
	}
	if e, ok := b.(Equaler); ok {
		return e.Equal(a)
	}
	if a.Type() != b.Type() {
		return false
	}
	// Kaplar eleman eleman karşılaştırılır: görünen metinleri aynı olan farklı değerler
	// (ör. yazdırırken yuvarlanan elemanlar) eşit sayılmaz. Aynı kap kendisine eşittir.
	switch a.(type) {
	case *ListVal, *DictVal, *StructInstanceVal:
		if a == b {
			return true
		}
		if nestDepth >= equalityDepth {
			return false
		}
	}
	switch x := a.(type) {
	case *StringVal:
		if y, ok := b.(*StringVal); ok {
			return x.Val == y.Val
		}
	case *ListVal:
		if y, ok := b.(*ListVal); ok {
			if len(x.Elements) != len(y.Elements) {
				return false
			}
			for i := range x.Elements {
				if !valuesEqual(x.Elements[i], y.Elements[i], nestDepth+1) {
					return false
				}
			}
			return true
		}
	case *DictVal:
		if y, ok := b.(*DictVal); ok {
			if len(x.Pairs) != len(y.Pairs) {
				return false
			}
			for k, va := range x.Pairs {
				vb, ok := y.Pairs[k]
				if !ok || !valuesEqual(va, vb, nestDepth+1) {
					return false
				}
			}
			return true
		}
	case *StructInstanceVal:
		return structsEqual(x, b, nestDepth)
	case *BoolVal:
		return x.Val == b.(*BoolVal).Val
	case *RangeVal:
		y := b.(*RangeVal)
		return x.Start == y.Start && x.End == y.End && x.Inclusive == y.Inclusive
	case *FuncVal:
		// Kimlik: aynı değer ya da aynı tanımın aynı ortamdaki (ve metotlarda aynı nesneye
		// bağlı) kopyası. Görünüşü (Inspect) aynı olan farklı fonksiyonlar eşit değildir.
		y := b.(*FuncVal)
		return x == y || (x.codeVal != nil && x.codeVal == y.codeVal && x.Env == y.Env && x.Body == y.Body && sameValue(x.BoundSelf, y.BoundSelf))
	}
	// Diğer değerler (yerleşikler, yapı tanımları, motor nesneleri) kimlikle karşılaştırılır.
	return a == b
}

// sameValue iki değerin (nil olabilir) aynı nesne olup olmadığını söyler.
func sameValue(a, b Value) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ap, aOK := a.(*StructInstanceVal)
	bp, bOK := b.(*StructInstanceVal)
	if aOK && bOK {
		return ap == bp
	}
	return aOK == bOK && a == b
}

// intPow tam sayı üssünü kare-çarp yöntemiyle hesaplar; int64 taşmasında ok=false döner.
func intPow(base, exp int64) (int64, bool) {
	result := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			if !mulFits(result, base) {
				return 0, false
			}
			result *= base
		}
		exp >>= 1
		if exp > 0 {
			if !mulFits(base, base) {
				return 0, false
			}
			base *= base
		}
	}
	return result, true
}

func mulFits(a, b int64) bool {
	if a == 0 || b == 0 {
		return true
	}
	c := a * b
	return c/b == a && !(a == -1 && b == math.MinInt64) && !(b == -1 && a == math.MinInt64)
}

// mixedCompare compares a Tam with an Ondalık exactly (4.5) for the comparison operators;
// converting the Tam to Ondalık loses precision above 2^53. With NaN only != is true.
// ok is false for other operators (arithmetic stays in Ondalık).
func mixedCompare(codeVal uint8, left, right Value) (Value, bool) {
	switch codeVal {
	case opLt, opGt, opLe, opGe, opEq, opNe:
	default:
		return nil, false
	}
	c, ok := compareNumbers(left, right)
	if !ok {
		return BoolToVal(codeVal == opNe), true
	}
	var r bool
	switch codeVal {
	case opLt:
		r = c < 0
	case opGt:
		r = c > 0
	case opLe:
		r = c <= 0
	case opGe:
		r = c >= 0
	case opEq:
		r = c == 0
	default:
		r = c != 0
	}
	return BoolToVal(r), true
}

package interp

import (
	"fmt"
	"math"
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
	opPipe     // |>
	opMatMul
	opAssign // = (düz atama)
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
	case "%":
		return opMod
	case "**":
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
	case "|>":
		return opPipe
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

	if codeVal == opAdd {
		// Metin birleştirme
		if ls, ok := left.(*StringVal); ok {
			return &StringVal{Val: ls.Val + right.Inspect()}, nil
		}
		if rs, ok := right.(*StringVal); ok {
			return &StringVal{Val: left.Inspect() + rs.Val}, nil
		}

		// Liste birleştirme
		if ll, ok1 := left.(*ListVal); ok1 {
			if rl, ok2 := right.(*ListVal); ok2 {
				combined := make([]Value, 0, len(ll.Elements)+len(rl.Elements))
				combined = append(combined, ll.Elements...)
				combined = append(combined, rl.Elements...)
				return &ListVal{Elements: combined}, nil
			}
		}
	}

	return nil, fmt.Errorf("Satır %d: %s ve %s tipleri arasında %q işlemi desteklenmiyor", line, left.Type(), right.Type(), op)
}

// intOp iki Tam sayı arasındaki işlemi yapar; işleç burada ele alınmıyorsa ok=false döner.
func intOp(codeVal uint8, op string, a, b int64, line int) (Value, bool, error) {
	switch codeVal {
	case opAdd:
		outcome := a + b
		if (outcome > a) != (b > 0) {
			return nil, true, overflowError(line, a, op, b)
		}
		return NewInt(outcome), true, nil
	case opSub:
		outcome := a - b
		if (outcome < a) != (b > 0) {
			return nil, true, overflowError(line, a, op, b)
		}
		return NewInt(outcome), true, nil
	case opMul:
		if !mulFits(a, b) {
			return nil, true, overflowError(line, a, op, b)
		}
		return NewInt(a * b), true, nil
	case opDiv:
		if b == 0 {
			return nil, true, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
		}
		if a == math.MinInt64 && b == -1 {
			return nil, true, overflowError(line, a, op, b)
		}
		if a%b == 0 {
			return NewInt(a / b), true, nil
		}
		return &FloatVal{Val: float64(a) / float64(b)}, true, nil
	case opMod:
		if b == 0 {
			return nil, true, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
		}
		return NewInt(a % b), true, nil
	case opPow:
		if b < 0 {
			return &FloatVal{Val: math.Pow(float64(a), float64(b))}, true, nil
		}
		res, ok := intPow(a, b)
		if !ok {
			return nil, true, fmt.Errorf("Satır %d: %s sayı taşması (%d ** %d); ondalık sonuç için örn. %d.0 ** %d yazın", line, VAL_INT, a, b, a, b)
		}
		return NewInt(res), true, nil
	case opLt:
		return BoolToVal(a < b), true, nil
	case opGt:
		return BoolToVal(a > b), true, nil
	case opLe:
		return BoolToVal(a <= b), true, nil
	case opGe:
		return BoolToVal(a >= b), true, nil
	case opEq:
		// İki Tam doğrudan karşılaştırılır (Ondalığa çevirmek 2^53 üstünde yanlış eşitlik verirdi)
		return BoolToVal(a == b), true, nil
	case opNe:
		return BoolToVal(a != b), true, nil
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
			return nil, true, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
		}
		return &FloatVal{Val: lf / rf}, true, nil
	case opMod:
		if rf == 0 {
			return nil, true, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
		}
		return &FloatVal{Val: math.Mod(lf, rf)}, true, nil
	case opPow:
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

// overflowError int64 sınırını aşan tam sayı işlemleri için hata üretir (sessizce sarmak yerine).
func overflowError(line int, a int64, op string, b int64) error {
	return fmt.Errorf("Satır %d: %s sayı taşması (%d %s %d); ondalık sonuç için örn. %d.0 %s %d yazın", line, VAL_INT, a, op, b, a, op, b)
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
		return TRUE
	}
	return FALSE
}

// equalityResult == (esit) ya da != işleminin sonucudur. Tip değeri metinle
// karşılaştırılamaz (3.11): sessizce yanlış dönmek yerine hata verir.
func equalityResult(equal bool, op string, left, right Value, line int) (Value, error) {
	if err := typeTextError(op, left, right); err != nil {
		return nil, fmt.Errorf("Satır %d: %w", line, err)
	}
	return BoolToVal(ValuesEqual(left, right) == equal), nil
}

func ValuesEqual(a, b Value) bool {
	return valuesEqual(a, b, 0)
}

// equalityDepth iç içe kap karşılaştırmasının en fazla derinliğidir; kendini içeren
// kaplarda (l == l dışında) sonsuz özyinelemeyi önler, aşılırsa eşit sayılmaz.
const equalityDepth = 10000

func valuesEqual(a, b Value, nestDepth int) bool {
	if a.Type() == VAL_NULL && b.Type() == VAL_NULL {
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
		if y, ok := b.(*StructInstanceVal); ok && x.Def == y.Def {
			for _, f := range x.Def.Fields {
				va, okA := x.Fields[f.Name]
				vb, okB := y.Fields[f.Name]
				if okA != okB || (okA && !valuesEqual(va, vb, nestDepth+1)) {
					return false
				}
			}
			return true
		}
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

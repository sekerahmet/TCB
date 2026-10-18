package vm

import (
	"fmt"
	"math"
)

// İşleç kodları: ayrıştırılmış işleç metinleri derleme anında bu tamsayılara çevrilir; böylece
// her işlemde metin karşılaştırması yapılmaz.
const (
	_ uint8 = iota // 0 kullanılmaz
	opBilinmeyen
	opTopla
	opCikar
	opCarp
	opBol
	opMod
	opUs
	opKucuk
	opBuyuk
	opKucukEsit
	opBuyukEsit
	opEsit
	opEsitDegil
	opVe
	opVeya
	opYoksa // ??
	opBoru  // |>
	opMatCarp
	opAta // = (düz atama)
)

// islecKodu bir işleç metninin kodunu döndürür. Bileşik atamalar (+=, ...) temel
// işlecin koduna, düz atama opAta'ya çevrilir.
func islecKodu(op string) uint8 {
	switch op {
	case "+", "+=":
		return opTopla
	case "-", "-=":
		return opCikar
	case "*", "*=":
		return opCarp
	case "/", "/=":
		return opBol
	case "%":
		return opMod
	case "**":
		return opUs
	case "<":
		return opKucuk
	case ">":
		return opBuyuk
	case "<=":
		return opKucukEsit
	case ">=":
		return opBuyukEsit
	case "==":
		return opEsit
	case "!=":
		return opEsitDegil
	case "&&":
		return opVe
	case "||":
		return opVeya
	case "??":
		return opYoksa
	case "|>":
		return opBoru
	case "@":
		return opMatCarp
	case "=":
		return opAta
	}
	return opBilinmeyen
}

// atamaIslecMetni bileşik atamanın kullandığı ikili işlecin metnini döndürür (hata mesajları için).
func atamaIslecMetni(kod uint8) string {
	switch kod {
	case opTopla:
		return "+"
	case opCikar:
		return "-"
	case opCarp:
		return "*"
	case opBol:
		return "/"
	}
	return ""
}

// evalInfixKod ikili işlemi işleç koduyla yapar; op yalnızca hata mesajları içindir.
func (v *VM) evalInfixKod(kod uint8, op string, left Value, right Value, line int) (Value, error) {
	// Hızlı yol: Tam ve Ondalık sayılar. Sayılar MatrixOperator olmadığından bu denetim
	// matris denetiminden önce yapılabilir.
	switch l := left.(type) {
	case *IntVal:
		switch r := right.(type) {
		case *IntVal:
			if val, ok, err := tamIslem(kod, op, l.Val, r.Val, line); ok {
				return val, err
			}
		case *FloatVal:
			if val, ok, err := ondalikIslem(kod, float64(l.Val), r.Val, line); ok {
				return val, err
			}
		}
	case *FloatVal:
		switch r := right.(type) {
		case *IntVal:
			if val, ok, err := ondalikIslem(kod, l.Val, float64(r.Val), line); ok {
				return val, err
			}
		case *FloatVal:
			if val, ok, err := ondalikIslem(kod, l.Val, r.Val, line); ok {
				return val, err
			}
		}
	}

	// Matris / Tensör operatörleri
	if mop, ok := left.(MatrixOperator); ok {
		switch kod {
		case opTopla:
			return mop.Add(right)
		case opCikar:
			return mop.Sub(right)
		case opCarp:
			return mop.Mul(right)
		case opBol:
			return mop.Div(right)
		case opMatCarp:
			return mop.MatMul(right)
		}
	}
	if mop, ok := right.(MatrixOperator); ok {
		switch kod {
		case opTopla:
			return mop.Add(left)
		case opCarp:
			return mop.Mul(left)
		}
	}

	switch kod {
	case opEsit:
		if ValuesEqual(left, right) {
			return TRUE, nil
		}
		return FALSE, nil
	case opEsitDegil:
		if !ValuesEqual(left, right) {
			return TRUE, nil
		}
		return FALSE, nil
	}

	if kod == opTopla {
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

// tamIslem iki Tam sayı arasındaki işlemi yapar; işleç burada ele alınmıyorsa ok=false döner.
func tamIslem(kod uint8, op string, a, b int64, line int) (Value, bool, error) {
	switch kod {
	case opTopla:
		sonuc := a + b
		if (sonuc > a) != (b > 0) {
			return nil, true, tasmaHatasi(line, a, op, b)
		}
		return NewInt(sonuc), true, nil
	case opCikar:
		sonuc := a - b
		if (sonuc < a) != (b > 0) {
			return nil, true, tasmaHatasi(line, a, op, b)
		}
		return NewInt(sonuc), true, nil
	case opCarp:
		if !mulFits(a, b) {
			return nil, true, tasmaHatasi(line, a, op, b)
		}
		return NewInt(a * b), true, nil
	case opBol:
		if b == 0 {
			return nil, true, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
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
	case opUs:
		if b < 0 {
			return &FloatVal{Val: math.Pow(float64(a), float64(b))}, true, nil
		}
		res, ok := intPow(a, b)
		if !ok {
			return nil, true, fmt.Errorf("Satır %d: Tam sayı taşması (%d ** %d); ondalık sonuç için örn. %d.0 ** %d yazın", line, a, b, a, b)
		}
		return NewInt(res), true, nil
	case opKucuk:
		return BoolToVal(a < b), true, nil
	case opBuyuk:
		return BoolToVal(a > b), true, nil
	case opKucukEsit:
		return BoolToVal(a <= b), true, nil
	case opBuyukEsit:
		return BoolToVal(a >= b), true, nil
	case opEsit:
		// ValuesEqual ile aynı kural: sayılar Ondalık olarak karşılaştırılır
		return BoolToVal(float64(a) == float64(b)), true, nil
	case opEsitDegil:
		return BoolToVal(float64(a) != float64(b)), true, nil
	}
	return nil, false, nil
}

// ondalikIslem Ondalık (veya Tam + Ondalık) sayı işlemlerini yapar; işleç burada ele
// alınmıyorsa ok=false döner.
func ondalikIslem(kod uint8, lf, rf float64, line int) (Value, bool, error) {
	switch kod {
	case opTopla:
		return &FloatVal{Val: lf + rf}, true, nil
	case opCikar:
		return &FloatVal{Val: lf - rf}, true, nil
	case opCarp:
		return &FloatVal{Val: lf * rf}, true, nil
	case opBol:
		if rf == 0 {
			return nil, true, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
		}
		return &FloatVal{Val: lf / rf}, true, nil
	case opMod:
		if rf == 0 {
			return nil, true, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
		}
		return &FloatVal{Val: math.Mod(lf, rf)}, true, nil
	case opUs:
		return &FloatVal{Val: math.Pow(lf, rf)}, true, nil
	case opKucuk:
		return BoolToVal(lf < rf), true, nil
	case opBuyuk:
		return BoolToVal(lf > rf), true, nil
	case opKucukEsit:
		return BoolToVal(lf <= rf), true, nil
	case opBuyukEsit:
		return BoolToVal(lf >= rf), true, nil
	case opEsit:
		return BoolToVal(lf == rf), true, nil
	case opEsitDegil:
		return BoolToVal(lf != rf), true, nil
	}
	return nil, false, nil
}

// tasmaHatasi int64 sınırını aşan tam sayı işlemleri için hata üretir (sessizce sarmak yerine).
func tasmaHatasi(line int, a int64, op string, b int64) error {
	return fmt.Errorf("Satır %d: Tam sayı taşması (%d %s %d); ondalık sonuç için örn. %d.0 %s %d yazın", line, a, op, b, a, op, b)
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

func ValuesEqual(a, b Value) bool {
	if a.Type() == VAL_NULL && b.Type() == VAL_NULL {
		return true
	}
	if af, ok1 := ToFloat(a); ok1 {
		if bf, ok2 := ToFloat(b); ok2 {
			return af == bf
		}
	}
	if a.Type() != b.Type() {
		return false
	}
	return a.Inspect() == b.Inspect()
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

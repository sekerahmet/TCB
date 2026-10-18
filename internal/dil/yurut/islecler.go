package yurut

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
	// Hızlı yol: Tam ve Ondalık sayılar. Sayılar işleç kancası uygulamadığından bu denetim
	// kancalardan önce yapılabilir.
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
	case *StringVal:
		// Metin + Metin (aşağıdaki genel metin birleştirmeyle aynı sonuç; arayüz denetimleri atlanır)
		if r, ok := right.(*StringVal); ok && kod == opTopla {
			return &StringVal{Val: l.Val + r.Val}, nil
		}
	}

	// Değerlerin kendi işleç kancaları (ör. tensör @ tensör, 2 - tensör, tensör < 0);
	// hatalarına satır bilgisi eklenir.
	switch kod {
	case opEsit, opEsitDegil:
	default:
		if val, ok, err := v.kancaIslec(op, left, right, line); ok {
			return val, err
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
		if a == math.MinInt64 && b == -1 {
			return nil, true, tasmaHatasi(line, a, op, b)
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
		// İki Tam doğrudan karşılaştırılır (Ondalığa çevirmek 2^53 üstünde yanlış eşitlik verirdi)
		return BoolToVal(a == b), true, nil
	case opEsitDegil:
		return BoolToVal(a != b), true, nil
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
	return degerlerEsit(a, b, 0)
}

// esitlikDerinligi iç içe kap karşılaştırmasının en fazla derinliğidir; kendini içeren
// kaplarda (l == l dışında) sonsuz özyinelemeyi önler, aşılırsa eşit sayılmaz.
const esitlikDerinligi = 10000

func degerlerEsit(a, b Value, derinlik int) bool {
	if a.Type() == VAL_NULL && b.Type() == VAL_NULL {
		return true
	}
	if ai, ok := a.(*IntVal); ok {
		if bi, ok := b.(*IntVal); ok {
			return ai.Val == bi.Val
		}
	}
	if af, ok1 := ToFloat(a); ok1 {
		if bf, ok2 := ToFloat(b); ok2 {
			return af == bf
		}
	}
	// Değerin kendi eşitlik kuralı (ör. tensör: şekil ve tüm elemanlar)
	if e, ok := a.(Esitlik); ok {
		return e.Esit(b)
	}
	if e, ok := b.(Esitlik); ok {
		return e.Esit(a)
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
		if derinlik >= esitlikDerinligi {
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
				if !degerlerEsit(x.Elements[i], y.Elements[i], derinlik+1) {
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
				if !ok || !degerlerEsit(va, vb, derinlik+1) {
					return false
				}
			}
			return true
		}
	case *StructInstanceVal:
		if y, ok := b.(*StructInstanceVal); ok && x.Def == y.Def {
			for _, f := range x.Def.Fields {
				va, oka := x.Fields[f.Name]
				vb, okb := y.Fields[f.Name]
				if oka != okb || (oka && !degerlerEsit(va, vb, derinlik+1)) {
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
		return x == y || (x.kod != nil && x.kod == y.kod && x.Env == y.Env && x.Body == y.Body && ayniDeger(x.BoundSelf, y.BoundSelf))
	}
	// Diğer değerler (yerleşikler, yapı tanımları, motor nesneleri) kimlikle karşılaştırılır.
	return a == b
}

// ayniDeger iki değerin (nil olabilir) aynı nesne olup olmadığını söyler.
func ayniDeger(a, b Value) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ap, aok := a.(*StructInstanceVal)
	bp, bok := b.(*StructInstanceVal)
	if aok && bok {
		return ap == bp
	}
	return aok == bok && a == b
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

package interp

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TR_round'un sayı kuralları (13.1): yarım çifte gider (IEEE 754'ün varsayılanı; Python, NumPy,
// PyTorch): TR_round(2.5) → 2, TR_round(-2.5) → -2. Basamaklı yuvarlama sayının yazılan ondalık
// metnine göredir (3.2): TR_round(2.675, 2) → 2.68, TR_round(0.125, 2) → 0.12. Tensör yolu ayrıdır (15.16a).

// roundIntDigits bir Tam'ı basamağa kesin yuvarlar, sonuç Tam'dır (13.1, 3.1: Ondalık'a çevrilmez):
// basamak 0 ya da artıysa sayının kendisi, eksiyse 10^-basamak'ın en yakın katı, yarım çifte
// (TR_round(1234, -2) → 1200, TR_round(1250, -2) → 1200). Sonuç int64'e sığmazsa hata.
func roundIntDigits(iv *IntVal, digits int64) (Value, error) {
	if digits >= 0 {
		return iv, nil
	}
	v := iv.Val
	if digits < -18 { // 10^19 int64'e sığmaz: |v| < 10^19 iken en yakın kat 0 ya da ±10^19'dur
		if digits == -19 && (v > 5e18 || v < -5e18) { // tam 5·10^18 yarımdır, çifte (0) gider
			exact := new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil)
			if v < 0 {
				exact.Neg(exact)
			}
			return nil, roundIntOverflow(exact)
		}
		return NewInt(0), nil
	}
	p := int64(1)
	for range -digits {
		p *= 10
	}
	q, r := v/p, v%p
	if twice := 2 * max(r, -r); twice > p || twice == p && q%2 != 0 {
		if v < 0 {
			q--
		} else {
			q++
		}
	}
	if q > math.MaxInt64/p || q < math.MinInt64/p {
		return nil, roundIntOverflow(new(big.Int).Mul(big.NewInt(q), big.NewInt(p)))
	}
	return NewInt(q * p), nil
}

// roundIntOverflow yuvarlanan Tam'ın (exact, kesin değeri) int64'ü aştığı hatadır.
func roundIntOverflow(exact *big.Int) error {
	return messages.New(messages.H2484, CallPrefix(codes.FnRound), exact.String(), codes.NameRef(codes.TypeInt))
}

// roundFloatDigits bir Ondalık'ı basamağa yuvarlar, sonuç Ondalık'tır (13.1; TR_round(3.7, 0) → 4.0,
// TR_round(1234.0, -2) → 1200.0). NaN ve sonsuz hatadır; sayının yazılan metninden çok basamak sayıyı
// değiştirmez (TR_round(2.5, 400) → 2.5), sayıdan büyük basamak sıfır verir (işaret korunur).
func roundFloatDigits(fv float64, digits int64) (Value, error) {
	if math.IsNaN(fv) || math.IsInf(fv, 0) {
		return nil, messages.New(messages.H2626, CallPrefix(codes.FnRound), (&FloatVal{Val: fv}).Text(codes.Display()))
	}
	rounded, ok := roundDecimalText(fv, digits)
	if !ok {
		return nil, messages.New(messages.H2627, CallPrefix(codes.FnRound), (&FloatVal{Val: fv}).Text(codes.Display()), digits)
	}
	return &FloatVal{Val: rounded}, nil
}

// roundDecimalText fv'yi yazılan ondalık metni üzerinde (en kısa geri dönen yazım, 3.2) noktadan sonra
// digits basamağa yuvarlar, yarımı çifte götürür ve en yakın Ondalık'ı verir. ok yanlışsa sonuç
// Ondalık'ın sınırını aşar.
func roundDecimalText(fv float64, digits int64) (float64, bool) {
	switch { // Ondalık'ın en uzun yazımı 330 basamaktan kısadır; uçlar int64 toplamını taşırmasın
	case digits > 400:
		return fv, true
	case digits < -400:
		return math.Copysign(0, fv), true
	}
	text := strconv.FormatFloat(math.Abs(fv), 'f', -1, 64)
	whole, frac, _ := strings.Cut(text, ".")
	all := whole + frac
	cut := int64(len(whole)) + digits // tutulan basamak sayısı (all'ın başından)
	if cut >= int64(len(all)) {
		return fv, true // yuvarlanacak basamak yok
	}
	kept, next, rest := "0", byte('0'), ""
	if cut >= 0 {
		kept, next, rest = "0"+all[:cut], all[cut], all[cut+1:]
	}
	n, _ := new(big.Int).SetString(kept, 10)
	odd := n.Bit(0) == 1
	if next > '5' || next == '5' && (strings.Trim(rest, "0") != "" || odd) {
		n.Add(n, big.NewInt(1))
	}
	sign := ""
	if math.Signbit(fv) {
		sign = "-"
	}
	shift := int64(len(whole)) - cut // sonuç n × 10^shift
	f, err := strconv.ParseFloat(sign+n.String()+"e"+strconv.FormatInt(shift, 10), 64)
	return f, err == nil
}

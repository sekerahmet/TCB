package messages

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// isExact "=N" anahtarıdır; N sayının tek (kanonik) yazımıdır: =0.0 ya da =+1 hiçbir değerle
// eşleşmezdi.
func isExact(key string) bool {
	if len(key) < 2 || key[0] != '=' {
		return false
	}
	f, err := strconv.ParseFloat(key[1:], 64)
	return err == nil && strconv.FormatFloat(f, 'f', -1, 64) == key[1:]
}

// pluralKey değerin dalını seçer: önce "=N" tam eşleşme, sonra dilin CLDR kategorisi, yoksa
// "other". Sayı olmayan değer "other"dır.
func (c *Catalog) pluralKey(branches map[string][]segment, v any) string {
	text, ok := exactText(v)
	if !ok {
		return "other"
	}
	if _, exact := branches["="+text]; exact {
		return "=" + text
	}
	rule := pluralRules[c.Language]
	if rule == nil {
		return "other"
	}
	o, _ := operands(v)
	if k := rule(o); branches[k] != nil {
		return k
	}
	return "other"
}

// exactText sayının "=N" anahtarındaki yazımıdır; değer sayı değilse ok false'tur.
func exactText(v any) (string, bool) {
	if x, ok := integerValue(v); ok {
		return x.String(), true
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	}
	return "", false
}

// operands değerin CLDR işlenenleridir. Tam sayı türünde v = 0'dır; ondalık türü (float) en
// az bir ondalık basamaklı sayılır: CLDR'de 1.0 yazılan sayı İngilizcede "one" değildir.
func operands(v any) (pluralOperands, bool) {
	if x, ok := integerValue(v); ok {
		x.Abs(x)
		n, _ := new(big.Float).SetInt(x).Float64()
		return pluralOperands{n: n, i: x.Uint64()}, true
	}
	var text string
	switch x := v.(type) {
	case float64:
		text = strconv.FormatFloat(math.Abs(x), 'f', -1, 64)
	case float32:
		text = strconv.FormatFloat(math.Abs(float64(x)), 'f', -1, 32)
	default:
		return pluralOperands{}, false
	}
	whole, frac, _ := strings.Cut(text, ".")
	if frac == "" {
		frac = "0"
	}
	o := pluralOperands{v: len(frac)}
	o.n, _ = strconv.ParseFloat(text, 64)
	o.i, _ = strconv.ParseUint(whole, 10, 64)
	o.f, _ = strconv.ParseUint(frac, 10, 64)
	return o, true
}

// integerValue Go'nun bütün tam sayı türlerini kayıpsız okur.
func integerValue(v any) (*big.Int, bool) {
	switch x := v.(type) {
	case int:
		return big.NewInt(int64(x)), true
	case int8:
		return big.NewInt(int64(x)), true
	case int16:
		return big.NewInt(int64(x)), true
	case int32:
		return big.NewInt(int64(x)), true
	case int64:
		return big.NewInt(x), true
	case uint:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint8:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint16:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint32:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint64:
		return new(big.Int).SetUint64(x), true
	}
	return nil, false
}

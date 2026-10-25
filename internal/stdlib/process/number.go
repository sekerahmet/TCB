package process

import (
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Komut satırı sayıları TR'nin sayı yazımıyla okunur: isteğe bağlı işaret, rakamlar (rakam
// grupları _ ile ayrılabilir: 5_000), Ondalık'ta nokta ve üs (1e-3, 2.5E4). Virgül ondalık
// ayırıcı değildir (0,001 yol gösteren hata), NaN ve sonsuz kabul edilmez. Hatalar, mesajın
// sonuna eklenecek açıklamadır.

// errNotNumber sayı olmayan yazımın hatasıdır; mesaja açıklama eklemez (boş metin).
var errNotNumber error = notNumber{}

type notNumber struct{}

func (notNumber) Error() string { return "" }

// numberFormat s'nin katı sayı yazımı olup olmadığını denetler ve alt çizgileri atılmış
// biçimini verir; ondalik, nokta ya da üs içerip içermediğidir.
func numberFormat(s string) (clean string, float bool, err error) {
	if strings.Contains(s, ",") {
		return "", false, messages.New(messages.H7215)
	}
	var b strings.Builder
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		b.WriteByte(s[i])
		i++
	}
	digits := func() bool {
		start := i
		for i < len(s) {
			c := s[i]
			if c >= '0' && c <= '9' {
				b.WriteByte(c)
				i++
				continue
			}
			// _ yalnızca iki rakamın arasında
			if c == '_' && i > start && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
				i++
				continue
			}
			break
		}
		return i > start
	}
	if !digits() {
		return "", false, errNotNumber
	}
	if i < len(s) && s[i] == '.' {
		b.WriteByte('.')
		i++
		float = true
		if !digits() {
			return "", false, errNotNumber
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		b.WriteByte('e')
		i++
		float = true
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			b.WriteByte(s[i])
			i++
		}
		if !digits() {
			return "", false, errNotNumber
		}
	}
	if i != len(s) {
		return "", false, errNotNumber
	}
	return b.String(), float, nil
}

// parseInt bir Tam değer okur.
func parseInt(s string) (int64, error) {
	clean, float, err := numberFormat(s)
	if err != nil {
		return 0, err
	}
	if float {
		return 0, messages.New(messages.H7216)
	}
	n, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0, messages.New(messages.H7217, codes.DisplayName(codes.A200))
	}
	return n, nil
}

// parseFloat bir Ondalık değer okur (tam sayı yazımı da kabul edilir: 3 → 3.0).
func parseFloat(s string) (float64, error) {
	if f, ok := nonFiniteName(s); ok {
		return f, nil // yardımın gösterdiği sonsuz ve tanımsız sayı adları (sonsuz, inf, NaN, nan)
	}
	clean, _, err := numberFormat(s)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, messages.New(messages.H7217, codes.DisplayName(codes.A201))
	}
	return f, nil
}

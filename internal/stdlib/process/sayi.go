package process

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Komut satırı sayıları TR'nin sayı yazımıyla okunur: isteğe bağlı işaret, rakamlar (rakam
// grupları _ ile ayrılabilir: 5_000), Ondalık'ta nokta ve üs (1e-3, 2.5E4). Virgül ondalık
// ayırıcı değildir (0,001 yol gösteren hata), NaN ve sonsuz kabul edilmez. Hatalar, mesajın
// sonuna eklenecek açıklamadır.

var errSayiDegil = errors.New("")

// sayiBicimi s'nin katı sayı yazımı olup olmadığını denetler ve alt çizgileri atılmış
// biçimini verir; ondalik, nokta ya da üs içerip içermediğidir.
func sayiBicimi(s string) (temiz string, ondalik bool, err error) {
	if strings.Contains(s, ",") {
		return "", false, errors.New("; ondalık ayırıcı nokta olmalı (0.001), binlik ayırıcı için _ yazın (5_000)")
	}
	var b strings.Builder
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		b.WriteByte(s[i])
		i++
	}
	rakamlar := func() bool {
		bas := i
		for i < len(s) {
			c := s[i]
			if c >= '0' && c <= '9' {
				b.WriteByte(c)
				i++
				continue
			}
			// _ yalnızca iki rakamın arasında
			if c == '_' && i > bas && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
				i++
				continue
			}
			break
		}
		return i > bas
	}
	if !rakamlar() {
		return "", false, errSayiDegil
	}
	if i < len(s) && s[i] == '.' {
		b.WriteByte('.')
		i++
		ondalik = true
		if !rakamlar() {
			return "", false, errSayiDegil
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		b.WriteByte('e')
		i++
		ondalik = true
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			b.WriteByte(s[i])
			i++
		}
		if !rakamlar() {
			return "", false, errSayiDegil
		}
	}
	if i != len(s) {
		return "", false, errSayiDegil
	}
	return b.String(), ondalik, nil
}

// tamCoz bir Tam değer okur.
func tamCoz(s string) (int64, error) {
	temiz, ondalik, err := sayiBicimi(s)
	if err != nil {
		return 0, err
	}
	if ondalik {
		return 0, errors.New("; tam sayı nokta ya da üs içermez")
	}
	n, err := strconv.ParseInt(temiz, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("; sayı %s aralığının dışında", codes.Name(codes.A200))
	}
	return n, nil
}

// ondalikCoz bir Ondalık değer okur (tam sayı yazımı da kabul edilir: 3 → 3.0).
func ondalikCoz(s string) (float64, error) {
	temiz, _, err := sayiBicimi(s)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(temiz, 64)
	if err != nil {
		return 0, fmt.Errorf("; sayı %s aralığının dışında", codes.Name(codes.A201))
	}
	return f, nil
}

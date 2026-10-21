package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestJSONDecodeRejectsBadInput(t *testing.T) {
	deep := strings.Repeat("[", maxJSONDepth+5) + strings.Repeat("]", maxJSONDepth+5)
	for input, want := range map[string]string{
		deep:             "çok derin",
		`{"a": 1} fazla`: "beklenmeyen veri",
		`{"a": }`:        "invalid",
	} {
		if _, err := JSONToTR([]byte(input)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%.30q için %q içeren hata bekleniyordu, alınan: %v", input, want, err)
		}
	}
	// Tam sayı ve ondalık ayrımı korunur; tam değerli ondalık eskisi gibi Tam olur
	v, err := JSONToTR([]byte(`[1, 2.5, 3.0, 9007199254740993]`))
	if err != nil || v.Inspect() != "[1, 2.5, 3, 9007199254740993]" {
		t.Fatalf("sayılar korunmalı: %v %v", v, err)
	}
}

// Kural: Tam aralığının (64 bit işaretli) dışındaki JSON tam sayısı açık hata verir; sessizce
// başka bir Tam'a kırpılmaz ya da mimariye göre farklı değer olmaz. Ondalık/üslü yazımlı
// büyük sayılar Ondalık kalır. Neden: float64'e yuvarlanınca -2^63-1 gibi sayılar MinInt64'e
// eşit çıkıyordu.
func TestJSONIntegerOutsideIntRangeIsAnError(t *testing.T) {
	for _, in := range []string{"-9223372036854775809", "-9223372036854776000", "9223372036854775808", "99999999999999999999"} {
		if v, err := JSONToTR([]byte(in)); err == nil || !messages.Has(err, messages.H2521) {
			t.Errorf("JSONToTR(%q) = %v, err=%v; H2521 bekleniyordu", in, v, err)
		}
	}
	for _, in := range []string{"-9223372036854775808", "9223372036854775807"} {
		if v, err := JSONToTR([]byte(in)); err != nil || v.Inspect() != in {
			t.Errorf("JSONToTR(%q) = %v, err=%v; sınırdaki Tam kabul edilmeliydi", in, v, err)
		}
	}
	for _, in := range []string{"1e30", "9223372036854775808.0", "-1e19"} {
		v, err := JSONToTR([]byte(in))
		if _, isInt := v.(*IntVal); err != nil || isInt {
			t.Errorf("JSONToTR(%q) = %v, err=%v; Ondalık bekleniyordu", in, v, err)
		}
	}
}

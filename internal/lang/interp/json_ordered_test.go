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
	// Tam sayı ve ondalık yazımı korunur (3.7a): tam değerli ondalık Ondalık kalır
	v, err := JSONToTR([]byte(`[1, 2.5, 3.0, 9007199254740993]`))
	if err != nil || v.Inspect() != "[1, 2.5, 3.0, 9007199254740993]" {
		t.Fatalf("sayılar korunmalı: %v %v", v, err)
	}
}

// Kural (3.7a): JSON çözmede en çok 1000 kap iç içe açılır; sınır içteki değere bağlı değildir (boş
// en içteki dizi de, sayı taşıyan da 1001. kapta düşer). Nesne de bir kaptır.
func TestJSONDepthCountsContainers(t *testing.T) {
	nest := func(n int, open, inner, close string) string {
		return strings.Repeat(open, n) + inner + strings.Repeat(close, n)
	}
	for _, inner := range []string{"", "1"} {
		if _, err := JSONToTR([]byte(nest(maxJSONDepth, "[", inner, "]"))); err != nil {
			t.Errorf("%d iç içe dizi (içte %q) geçmeli: %v", maxJSONDepth, inner, err)
		}
		if _, err := JSONToTR([]byte(nest(maxJSONDepth+1, "[", inner, "]"))); !messages.Has(err, messages.H2500) {
			t.Errorf("%d iç içe dizi (içte %q) H2500 vermeli: %v", maxJSONDepth+1, inner, err)
		}
	}
	// en içteki {} de bir kaptır: n sarmalayıcı + 1
	if _, err := JSONToTR([]byte(nest(maxJSONDepth-1, `{"a":`, "{}", "}"))); err != nil {
		t.Errorf("%d iç içe nesne geçmeli: %v", maxJSONDepth, err)
	}
	if _, err := JSONToTR([]byte(nest(maxJSONDepth, `{"a":`, "{}", "}"))); !messages.Has(err, messages.H2500) {
		t.Errorf("%d iç içe nesne H2500 vermeli: %v", maxJSONDepth+1, err)
	}
}

// Kural (3.7a): Tam aralığının (64 bit işaretli) dışındaki JSON tam sayısının türü Ondalık'tır; sessizce
// başka bir Tam'a kırpılmaz ya da mimariye göre farklı değer olmaz. Ondalık yazımlı sayı tam değerli
// olsa da Ondalık kalır (2.0); sonsuza taşan sayı hatadır.
func TestJSONIntegerOutsideIntRangeIsAFloat(t *testing.T) {
	for _, in := range []string{"-9223372036854775809", "9223372036854775808", "99999999999999999999", "2.0"} {
		if v, err := JSONToTR([]byte(in)); err != nil || v.Type() != ValFloat {
			t.Errorf("JSONToTR(%q) = %v, err=%v; Ondalık bekleniyordu", in, v, err)
		}
	}
	if _, err := JSONToTR([]byte("1e999")); !messages.Has(err, messages.H2503) {
		t.Errorf("JSONToTR(1e999): %v; H2503 bekleniyordu", err)
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

// Kural (3.7a, 13.8): nesnede yinelenen anahtar çözmede hatadır; ileti anahtarı ve iki yerini (satır,
// sütun; sütun karakter sayar) söyler. Aynı anahtar başka nesnelerde serbesttir.
func TestJSONDuplicateKeyIsAnError(t *testing.T) {
	_, err := JSONToTR([]byte("{\"ç\": 1,\n  \"ç\": 2}"))
	if !messages.Has(err, messages.H7011) {
		t.Fatalf("H7011 bekleniyordu: %v", err)
	}
	if want := `"ç" anahtarı iki kez yazılmış (satır 1, sütun 2 ve satır 2, sütun 3)`; !strings.Contains(err.Error(), want) {
		t.Errorf("ileti %q içermeli: %v", want, err)
	}
	_, err = JSONToTR([]byte(`[{"a": {"x": 1}, "b": {"x": 2, "y": 3, "x": 4}}]`))
	if !messages.Has(err, messages.H7011) || !strings.Contains(err.Error(), "sütun 24 ve satır 1, sütun 40") {
		t.Errorf("iç nesnedeki yinelenen anahtar: %v", err)
	}
	if _, err := JSONToTR([]byte(`{"a": {"a": 1}, "b": {"a": 2}, "c": [{"a": 3}, {"a": 4}]}`)); err != nil {
		t.Errorf("başka nesnedeki aynı anahtar serbest: %v", err)
	}
}

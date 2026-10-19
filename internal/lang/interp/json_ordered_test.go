package interp

import (
	"strings"
	"testing"
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

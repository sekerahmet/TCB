package vm

import (
	"strings"
	"testing"
	"time"
)

// json_çöz sözlük anahtarlarının sırasını korumuyordu (Go map'i sırasızdır).
func TestJSONDecodePreservesKeyOrder(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		değişken d = json_çöz("{\"zeta\": 1, \"alfa\": 2.5, \"orta\": [true, null, \"x\"], \"iç\": {\"b\": 1, \"a\": 2}}")
		sonuç [d.anahtarlar(), d["iç"].anahtarlar(), d["orta"], d["alfa"], json_metin(d)]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["zeta", "alfa", "orta", "iç"], ["b", "a"], [doğru, yok, "x"], 2.5, `
	if !strings.HasPrefix(val.Inspect(), want) {
		t.Fatalf("%s… bekleniyordu, %s alındı", want, val.Inspect())
	}
}

func TestJSONDecodeRejectsBadInput(t *testing.T) {
	derin := strings.Repeat("[", maxJSONDepth+5) + strings.Repeat("]", maxJSONDepth+5)
	for girdi, want := range map[string]string{
		derin:            "çok derin",
		`{"a": 1} fazla`: "beklenmeyen veri",
		`{"a": }`:        "invalid",
	} {
		if _, err := jsonToTR([]byte(girdi)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%.30q için %q içeren hata bekleniyordu, alınan: %v", girdi, want, err)
		}
	}
	// Tam sayı ve ondalık ayrımı korunur; tam değerli ondalık eskisi gibi Tam olur
	v, err := jsonToTR([]byte(`[1, 2.5, 3.0, 9007199254740993]`))
	if err != nil || v.Inspect() != "[1, 2.5, 3, 9007199254740993]" {
		t.Fatalf("sayılar korunmalı: %v %v", v, err)
	}
}

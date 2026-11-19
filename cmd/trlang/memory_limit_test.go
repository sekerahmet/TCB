package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (19.12a): TR_MEMORY_LIMIT'in değeri bayt cinsinden, sıfırdan büyük, int64'e sığan ASCII
// rakamlarıdır. Birimli değer bayt yazımını önerir: tek okunan birimde (B, KiB ... TiB) H8891, iki
// okunan birimde (K, KB ... TB) iki yazımla H8892. Öbür her değer H8890'dır; int64'e sığmayan değer
// üst sınırı söyleyen koludur.
func TestMemoryLimitValue(t *testing.T) {
	for v, want := range map[string]int64{
		"1": 1, "6442450944": 6442450944, "0123": 123, "9223372036854775807": 9223372036854775807,
	} {
		if n, err := parseMemoryLimit(v); err != nil || n != want {
			t.Errorf("%q: %d, %v; %d bekleniyordu", v, n, err, want)
		}
	}
	for _, c := range []struct {
		value string
		code  messages.Code
		args  []any // ilk ikisinden (ad, değer) sonraki değerler
	}{
		{"", messages.H8890, []any{limitInvalid}},
		{"0", messages.H8890, []any{limitInvalid}},
		{"-1", messages.H8890, []any{limitInvalid}},
		{"+5", messages.H8890, []any{limitInvalid}},
		{"1.5", messages.H8890, []any{limitInvalid}},
		{" 6442450944", messages.H8890, []any{limitInvalid}},
		{"altı", messages.H8890, []any{limitInvalid}},
		{"٣", messages.H8890, []any{limitInvalid}},
		{"6 GiB fazla", messages.H8890, []any{limitInvalid}},
		{"6EiB", messages.H8890, []any{limitInvalid}},
		{"1.5B", messages.H8890, []any{limitInvalid}},
		{"0GiB", messages.H8890, []any{limitInvalid}},
		{"1.0001G", messages.H8890, []any{limitInvalid}}, // 2'nin kuvvetiyle tam sayı değil
		{"9223372036854775808", messages.H8890, []any{limitTooLarge}},
		{"99999999999999999999999", messages.H8890, []any{limitTooLarge}},
		{"9000000TiB", messages.H8890, []any{limitTooLarge}},
		{"9000000T", messages.H8890, []any{limitTooLarge}}, // 10'un kuvvetiyle sığar, 2'ninkiyle sığmaz
		{"6GiB", messages.H8891, []any{int64(6442450944)}},
		{" 6 gib ", messages.H8891, []any{int64(6442450944)}},
		{"1.5GiB", messages.H8891, []any{int64(1610612736)}},
		{"1,5 GiB", messages.H8891, []any{int64(1610612736)}},
		{"512B", messages.H8891, []any{int64(512)}},
		{"6GB", messages.H8892, []any{int64(6000000000), int64(6442450944)}},
		{"1.5g", messages.H8892, []any{int64(1500000000), int64(1610612736)}},
		{"2 K", messages.H8892, []any{int64(2000), int64(2048)}},
	} {
		n, err := parseMemoryLimit(c.value)
		e, ok := messages.As(err)
		if !ok || e.Code != c.code || n != 0 {
			t.Errorf("%q: %d, %v; %v bekleniyordu", c.value, n, err, c.code)
			continue
		}
		if len(e.Args) < 2 || e.Args[1] != c.value || !slices.Equal(e.Args[2:], c.args) {
			t.Errorf("%q: değerler %v, sonu %v bekleniyordu", c.value, e.Args, c.args)
		}
		if !strings.Contains(err.Error(), "TR_MEMORY_LIMIT") {
			t.Errorf("%q: ileti kimlik adını yazmıyor: %s", c.value, err)
		}
	}
}

// trlang'ın uyarı listesi testlerin listesiyle aynıdır: testler alt süreçten tam bu değişkenleri çıkarır.
func TestRuntimeVariableListsAgree(t *testing.T) {
	if !slices.Equal(runtimeVariables, testkit.RuntimeVariables) {
		t.Fatalf("trlang %v, testkit %v", runtimeVariables, testkit.RuntimeVariables)
	}
}

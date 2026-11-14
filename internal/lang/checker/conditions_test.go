package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (12.3, 15.5): tensör koşulunda H2612 tensörün yazımlarını önerir (TR_assert_equal, TR_all,
// 0 boyutluda .TR_item(), TR_assert_close), öbür tipte karşılaştırmayı. Önerilerin koştuğunu
// tests/matematik_test.tr'nin 15.5 testi sınar.
func TestAssertTensorConditionSuggestsTensorSpellings(t *testing.T) {
	_, errs := checkErrs(t, "TR_var a = TR_Tensor([1.0])\nTR_assert(a == a)")
	if len(errs) != 1 || !messages.Has(errs[0], messages.H2612) {
		t.Fatalf("tek H2612 beklenirdi, gelen %v", errs)
	}
	for _, lang := range []string{"tr", "en"} {
		text := messages.ErrorIn(errs[0], lang)
		for _, want := range []string{"`TR_assert_equal(", "`TR_assert(TR_all(x == ", "`TR_assert(x.TR_item() == ", "`TR_assert_close(x, "} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: ileti %q yazımını önermeli: %q", lang, want, text)
			}
		}
	}
	_, errs = checkErrs(t, "TR_assert([1])")
	if len(errs) != 1 || strings.Contains(messages.ErrorIn(errs[0], "tr"), "TR_all") {
		t.Errorf("tensör olmayan koşulda tensör önerisi olmamalı: %v", errs)
	}
}

// Kural (3.10): tipi bilinen koşul TR_Bool değilse denetim hatasıdır; kod çalışma anındakidir:
// koşul H2600, TR_and/TR_or/TR_not işleneni H2601, TR_assert'in koşulu H2612. Tipi bilinmeyen ve
// isteğe bağlı koşul susar (daraltma 4b'nin).
func TestConditionMustBeBool(t *testing.T) {
	cases := []struct {
		source string
		code   messages.Code
	}{
		{"TR_var l = [1]\nTR_if l { TR_print(1) }", messages.H2600},
		{"TR_if (1, 2) { TR_print(1) }", messages.H2600},
		{"TR_if TR_false { } TR_else TR_if \"a\" { }", messages.H2600},
		{"TR_if 5 { }", messages.H2600},
		{"TR_for \"x\" { TR_break }", messages.H2600},
		{"TR_var c = [x TR_for x TR_in [1] TR_if x]", messages.H2600},
		{"TR_var z = 1 TR_and TR_true", messages.H2601},
		{"TR_var z = TR_true TR_or [1]", messages.H2601},
		{"TR_var z = TR_not 0", messages.H2601},
		{"TR_assert([1])", messages.H2612},
		{"TR_if TR_Tensor([0.0]) { }", messages.H2600}, // tensör koşul değildir (yerleşiğin dönüşü)
	}
	for _, tc := range cases {
		_, errs := checkErrs(t, tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], tc.code) {
			t.Errorf("tek %s beklenirdi, gelen %v\nkaynak:\n%s", tc.code, errs, tc.source)
		}
	}
	for _, source := range []string{
		"TR_var l = [1]\nTR_if l.TR_length == 0 TR_and TR_not TR_false { TR_print(1) }",
		"TR_var b: TR_Bool? = TR_none\nTR_if b == TR_true { }",
		`TR_var j = ` + unknownValue + "\nTR_if j == 1 { }",
		"TR_assert(1 < 2)",
	} {
		noErrors(t, source)
	}
}

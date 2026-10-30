package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

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
		`TR_var j = TR_json_parse("1")` + "\nTR_if j == 1 { }",
		"TR_assert(1 < 2)",
	} {
		noErrors(t, source)
	}
}

package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (4.10, 5.4): '+' iki sayıyı toplar, iki metni ya da iki listeyi uç uca ekler; başka tiple metin
// ya da liste denetim hatasıdır, çalışma anının koduyla: metinde H2609, metinsiz listede H2614. x += y
// aynı kurala uyar. Tipi bilinmeyen işlenen, isteğe bağlı tip, tensör ve sayılar hatasızdır.
func TestPlusOperands(t *testing.T) {
	cases := []struct {
		source string
		code   messages.Code
	}{
		{`TR_var z = "a" + 1`, messages.H2609},
		{`TR_var z = 1.5 + "a"`, messages.H2609},
		{`TR_var z = "a" + [1]`, messages.H2609},
		{`TR_var z = [1] + 2`, messages.H2614},
		{`TR_var z = {"a": 1} + [1]`, messages.H2614},
		{"TR_var l = [1]\nl += 2", messages.H2614},
	}
	for _, tc := range cases {
		_, errs := checkErrs(t, tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], tc.code) {
			t.Errorf("tek %s beklenirdi, gelen %v\nkaynak:\n%s", tc.code, errs, tc.source)
		}
	}
	noErrors(t, "TR_var a = \"a\" + \"b\"\nTR_var b = [1] + [2]\nTR_var c = 1 + 2.5\nTR_var j = "+unknownValue+"\nTR_var d = j + 1\nTR_var l = [1]\nl += [2]\nTR_var t = TR_Tensor([1.0]) + 1\nTR_print(a, b, c, d, l, t)")
}

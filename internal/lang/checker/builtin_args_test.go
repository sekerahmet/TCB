package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (13.9, 13.11): denetlenen yerleşik fonksiyonun (FunctionArgCodes) seçenek dışı tipte argümanı
// çalışma anının koduyla denetim hatasıdır: TR_abs("s") H2423, TR_sin("s") H6103, TR_zeros("a")
// H6021; açılmamış isteğe bağlı değer yokluğun koduyla (TR_abs(l.TR_first) H2423). Her seçenek
// (TR_abs(1), TR_abs(1.5), TR_abs(t)), TR_NoneType seçeneğine TR_none, liste ve Tam verilen argüman
// (önerili çalışma anı kodu), tipi bilinmeyen değer ve ad geçer.
func TestBuiltinArgOptions(t *testing.T) {
	const l = "TR_var l: TR_List[TR_Int] = []\n"
	cases := []struct {
		name, src string
		code      messages.Code
		line      int
	}{
		{"metin", "TR_print(TR_abs(\"s\"))", messages.H2423, 1},
		{"mantık", "TR_print(TR_sqrt(TR_true))", messages.H2423, 1},
		{"alanın kodu", "TR_print(TR_sin(\"s\"))", messages.H6103, 1},
		{"şekil", "TR_var z = TR_zeros(\"a\")", messages.H6021, 1},
		{"açılmamış", l + "TR_print(TR_abs(l.TR_first))", messages.H2423, 2},
		{"yokluk", "TR_print(TR_abs(TR_none))", messages.H2423, 1},
		{"ikinci argüman", "TR_print(TR_round(1.5, \"a\"))", messages.H2423, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, tc.code, tc.line) })
	}
	noErrors(t, "TR_var t = TR_Tensor([1.0])\nTR_print(TR_abs(1), TR_abs(1.5), TR_abs(t), TR_sin(1), TR_round(1.5, 1))")
	noErrors(t, l+"TR_print(TR_abs([1]), TR_sin([1.0]))\nTR_var z = TR_zeros(3)\nTR_var u = [1, \"a\"][0]\nTR_print(TR_abs(u))\nTR_var x = l.TR_first\nTR_print(TR_abs(x))")
}

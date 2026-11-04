package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (6.2, 6.6): dönüş tipi beklenen tipten gelmeyen blok gövdesinde bir yol değer döndürüyorsa
// her yol değerle ya da 6.2'nin öbür bitişleriyle biter; değersiz TR_return ve bitmeyen yol hatadır.
func TestArrowBlockWithAValueEndsEveryPath(t *testing.T) {
	for _, body := range []string{
		"TR_if x { TR_return }\n  TR_return 1",
		"TR_if x { TR_return TR_none }\n  TR_return 1",
		"TR_if x { TR_return 1 }",
		"TR_if x { TR_return 1 } TR_else { TR_print(2) }",
		"TR_match x {\n    TR_true => { TR_return 1 }\n    TR_false => { TR_print(2) }\n  }",
		"TR_for { TR_if x { TR_break }\n    TR_return 1 }",
		"TR_try { TR_return 1 } TR_catch e { TR_print(e) }",
	} {
		singleError(t, "TR_var h = (x: TR_Bool) => {\n  "+body+"\n}", messages.H1145, 1)
		_, errs := checkErrs(t, "TR_var y = [TR_true].TR_filter((x) => {\n  "+body+"\n})")
		if !hasCode(errs, messages.H1145) {
			t.Errorf("yerleşiğin argümanında H1145 beklenirdi, gelen %v\ngövde:\n%s", errs, body)
		}
	}
	for _, body := range []string{
		"TR_return 1",
		"TR_if x { TR_return 2 }\n  TR_return 1",
		"TR_if x { TR_return 1 }\n  TR_throw \"e\"",
		"TR_if x { TR_return 1 }\n  TR_exit(1)",
		"TR_if x { TR_return 1 } TR_else { TR_throw \"e\" }",
		"TR_if x { TR_return 1 } TR_else TR_if TR_true { TR_return 2 } TR_else { TR_return 3 }",
		"TR_match x {\n    TR_true => { TR_return 1 }\n    TR_false => TR_exit(1)\n  }",
		"TR_for {\n    TR_if x { TR_return 1 }\n  }",
		"TR_for {\n    TR_for { TR_break }\n    TR_return 1\n  }",
		"TR_try { TR_return 1 } TR_catch e { TR_return 2 }",
		"TR_if x { TR_return }\n  TR_print(1)",
		"TR_print(1)",
	} {
		noErrors(t, "TR_var h = (x: TR_Bool) => {\n  "+body+"\n}")
	}
}

// Kural (6.6): beklenen dönüş tipi biliniyorsa karışım geçerlidir (TR_Int? yokluğu verir): fonksiyon
// tipi yazılmış her yerde. Yerleşiğin argümanında ve tipi yazılmamış yerde bilinmez, kural geçerlidir.
func TestArrowBlockWithAKnownReturnMayMix(t *testing.T) {
	const mixed = "(x: TR_Bool) => {\n  TR_if x { TR_return }\n  TR_return 1\n}"
	const fn = "(TR_Bool) -> TR_Int?"
	for _, source := range []string{
		"TR_var h: " + fn + " = " + mixed,
		"TR_var h: " + fn + " = (x: TR_Bool) => {\n  TR_if x { TR_return TR_none }\n  TR_return 1\n}",
		"TR_var h: " + fn + " = (x: TR_Bool) => 1\nh = " + mixed,
		"TR_var l: TR_List[" + fn + "] = [" + mixed + "]",
		"TR_func w(g: " + fn + ") { }\nw(" + mixed + ")",
		"TR_func w(a: TR_Int, g: " + fn + ") { }\nw(1, g: " + mixed + ")",
		"TR_func w(g: " + fn + " = " + mixed + ") { }",
		"TR_func w() -> " + fn + " {\n  TR_return " + mixed + "\n}",
		"TR_struct P {\n  g: " + fn + " = " + mixed + "\n}",
		"TR_struct P {\n  g: " + fn + "\n}\nTR_var p = P(" + mixed + ")",
		"TR_struct P {\n  TR_func m(g: " + fn + ") { }\n}\nTR_var p = P()\np.m(" + mixed + ")",
	} {
		noErrors(t, source)
	}
}

// Kural (6.6; DENETİM 109): yerleşiğin imzası dönüş tipi söylemez: .TR_map'e verilen bitmeyen yollu
// gövde sessiz yokluk bırakırdı ([none, 1]).
func TestArrowBlockGivenToABuiltinEndsEveryPath(t *testing.T) {
	for _, source := range []string{
		"TR_var l = [1, 2]\nTR_print(l.TR_map((x: TR_Int) => { TR_if x > 1 { TR_return 1 } }))",
		"TR_var l = [1, 2]\nTR_print(l.TR_map((x) => { TR_if x > 1 { TR_return 1 } }))",
		"TR_var m = " + unknownValue + "\nm.yok((x: TR_Int) => { TR_if x > 1 { TR_return 1 } })",
	} {
		_, errs := checkErrs(t, source)
		if !hasCode(errs, messages.H1145) {
			t.Errorf("H1145 beklenirdi, gelen %v\nkaynak:\n%s", errs, source)
		}
	}
}

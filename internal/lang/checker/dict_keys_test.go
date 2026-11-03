package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.7, H1140): anahtar tipi bilinen sözlüğü başka tipte anahtarla okumak ya da ona yazmak
// denetim hatasıdır: d[1], d[2] = 3, .TR_get(5), .TR_contains(5). Ondalık anahtar (indeks, değişmez,
// TR_Dict[TR_Float, V] yazımı) çalışma anının koduyla H2646. Tipi bilinmeyen ya da isteğe bağlı
// anahtar ve öğe tipsiz sözlük geçer.
func TestDictKeyType(t *testing.T) {
	const d = "TR_var d = {\"a\": 1}\n"
	cases := []struct {
		name, src string
		code      messages.Code
		line      int
	}{
		{"okuma", d + "TR_print(d[1])", messages.H1140, 2},
		{"yazma", d + "d[2] = 3", messages.H1140, 2},
		{"bileşik", d + "d[2] += 3", messages.H1140, 2},
		{"varsa al", d + "TR_print(d.TR_get(5))", messages.H1140, 2},
		{"içerir", d + "TR_print(d.TR_contains(5))", messages.H1140, 2},
		{"demet anahtar", "TR_var e = {(1, 2): \"a\"}\nTR_print(e[(1, \"x\")])", messages.H1140, 2},
		{"ondalık indeks", "TR_var e = {1: \"a\"}\nTR_print(e[1.5])", messages.H2646, 2},
		{"ondalık değişmez", "TR_var e = {1.5: \"a\"}", messages.H2646, 1},
		{"ondalık yazım", "TR_var e: TR_Dict[TR_Float, TR_Int] = {}", messages.H2646, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, tc.code, tc.line) })
	}
	noErrors(t, d+"TR_print(d[\"a\"], d.TR_get(\"b\"), d.TR_contains(\"a\"))\nd[\"b\"] = 2\nTR_var e = {(1, 2): \"a\"}\nTR_print(e[(3, 4)])")
	noErrors(t, "TR_var d = {}\nd[1] = \"a\"\nd[\"b\"] = 2\nTR_var k = [1, \"a\"][0]\nTR_var e = {\"a\": 1}\nTR_print(e[k])")
}

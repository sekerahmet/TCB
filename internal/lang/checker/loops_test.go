package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Desen (5.7) yalnız demeti açar: adlar demetin öğe tiplerini alır (yanlış tipe atama H1001), sözlükte
// tek adın yerinde anahtarın, indeksli biçimde değerin. Tipi bilinen öğe demet değilse (liste, sayı,
// isteğe bağlı demet) H2731, iletinin önerisi öğeyi tek adla alır ve listede gövdede açmayı söyler;
// demetin öğe sayısı tutmazsa H1026, örnek düzeltilmiş döngü başlığıdır. Tipi bilinmeyen öğe
// (fonksiyon değerinin dönüşü) denetimde hatasızdır, çalışırken denetlenir (interp). İki adlı biçimde
// öğenin tipi demetse (sözlük dışında) H2730, ileti deseni indekssiz ve indeksli önerir; sözlükte ikinci
// ad değerdir, isteğe bağlı demet ve iç liste bu kuralın dışındadır.
func TestLoopPatternIsChecked(t *testing.T) {
	const pairs = "TR_var l = [(1, \"p\")]\n"
	cases := []struct {
		src  string
		code messages.Code
		text string
	}{
		{pairs + "TR_for (a, b) TR_in l { TR_var s: TR_String = b\nTR_print(a, s) }", 0, ""},
		{pairs + "TR_for i, (a, b) TR_in l { TR_var n: TR_Int = i + a\nTR_print(n, b) }", 0, ""},
		{pairs + "TR_for (a, b) TR_in l { TR_var s: TR_String = a }", messages.H1001, ""},
		{"TR_var d = {(1, 2): \"v\"}\nTR_for (a, b) TR_in d { TR_var s: TR_String = a }", messages.H1001, ""},
		{"TR_var d = {\"k\": (1, 2)}\nTR_for k, (a, b) TR_in d { TR_var s: TR_String = b }", messages.H1001, ""},
		{pairs + "TR_for (a, b, c) TR_in l { TR_print(a) }", messages.H1026, "(TR_for (a, b) TR_in l)"},
		{"TR_for i, (a, b) TR_in [[1, 2]] { TR_print(a) }", messages.H2731, "`TR_for i, a TR_in [[1, 2]]` ve listeyi gövdede açın"},
		{"TR_for (_, b) TR_in 0..3 { TR_print(b) }", messages.H2731, "`TR_for b TR_in 0..3`"},
		{"TR_var o: (TR_Int, TR_Int)? = (1, 2)\nTR_for (a, b) TR_in [o] { TR_print(a) }", messages.H2731, "öğenin tipi (TR_Int, TR_Int)?"},
		{"TR_var g = (n: TR_Int) => [n]\nTR_for (a, b) TR_in g(1) { TR_print(a) }", 0, ""},
		{"TR_for a, b TR_in [(1, 2)] { TR_print(a, b) }", messages.H2730, "`TR_for (a, b) TR_in [(1, 2)]`, indeksiyle birlikte `TR_for i, (a, b) TR_in [(1, 2)]`"},
		{"TR_for i, x TR_in [(1, 2, 3)] { TR_print(i, x) }", messages.H2730, "3 öğeli bir demet; demeti açmak için `TR_for (i, x, _) TR_in [(1, 2, 3)]`, indeksiyle birlikte `TR_for j, (i, x, _) TR_in [(1, 2, 3)]`"},
		{"TR_for k, v TR_in {\"a\": (1, 2)} { TR_print(k, v) }", 0, ""},
		{"TR_var o: (TR_Int, TR_Int)? = (1, 2)\nTR_for i, x TR_in [o] { TR_print(i, x) }", 0, ""},
		{"TR_for i, x TR_in [[1, 2]] { TR_print(i, x) }", 0, ""},
	}
	for _, c := range cases {
		errs := checkCoded(t, c.src)
		if c.code == 0 {
			if len(errs) != 0 {
				t.Errorf("%q: hata beklenmiyordu: %v", c.src, errs)
			}
			continue
		}
		if len(errs) != 1 || !messages.Has(errs[0], c.code) {
			t.Errorf("%q: tek bir %s bekleniyordu: %v", c.src, c.code, errs)
			continue
		}
		if !strings.Contains(errs[0].Error(), c.text) {
			t.Errorf("%q: ileti %q'yi taşımıyor: %v", c.src, c.text, errs[0])
		}
	}
}

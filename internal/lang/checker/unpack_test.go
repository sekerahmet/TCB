package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (5.3, 6.3): sağ yanı bir demet yazımı ya da dönüşü '-> (A, B)' diye yazılmış fonksiyonun
// doğrudan çağrısı olan açmada öğe sayısı ad sayısını tutmazsa denetim hatasıdır. Sayının bilinmediği
// yerler (fonksiyon değeri, dönüş tipi yazılmamış fonksiyon, isteğe bağlı demet, liste) denetimden
// geçer; sayıyı çalışırken H2709 ve H2700 denetler.
func TestUnpackCountOfAKnownTuple(t *testing.T) {
	const (
		iki = "TR_func iki() -> (TR_Int, TR_Int) { TR_return 1, 2 }\n"
		g2  = "TR_func g(n: TR_Int, s: TR_String) -> (TR_Int, TR_String) { TR_return n, s }\n"
		g3  = "TR_func g(n: TR_Int, s: TR_String) -> (TR_Int, TR_String, TR_Int) { TR_return n, s, n + 1 }\n"
	)
	for _, tc := range []struct {
		prefix, bad, wantExample, wantTR, wantEN string
		line                                     int
	}{
		{
			"", "TR_var a, b, c = (1, 2)", "TR_var a, b = (1, 2)",
			"3 ad 2 öğeli bir demeti açıyor: sayılar tutmalı; fazla adı silin (TR_var a, b = (1, 2))",
			"3 names unpack a tuple of 2 elements: the counts must match; remove the extra names (TR_var a, b = (1, 2))",
			1,
		},
		{
			g2, `TR_var a, b, c = g(1, "x")`, `TR_var a, b = g(1, "x")`,
			`fazla adı silin (TR_var a, b = g(1, "x"))`,
			`remove the extra names (TR_var a, b = g(1, "x"))`,
			2,
		},
		{
			iki + "TR_var a = 0\nTR_var b = 0\nTR_var c = 0\n", "a, b, c = iki()", "a, b = iki()",
			"fazla adı silin (a, b = iki())",
			"remove the extra names (a, b = iki())",
			5,
		},
		{
			g3, `TR_var a, b = g(1, "x")`, `TR_var a, b, _ = g(1, "x")`,
			`2 ad 3 öğeli bir demeti açıyor: sayılar tutmalı; her öğeye bir ad, gerekmeyenin yerine _ yazın (TR_var a, b, _ = g(1, "x"))`,
			`2 names unpack a tuple of 3 elements: the counts must match; write a name for each element, and _ for one that is not needed (TR_var a, b, _ = g(1, "x"))`,
			2,
		},
		{
			"TR_func üç() -> (TR_Int, (TR_Int, TR_Int), TR_Int) { TR_return 1, (2, 3), 4 }\n",
			"TR_var a, b = üç()", "TR_var a, b, _ = üç()",
			"her öğeye bir ad, gerekmeyenin yerine _ yazın (TR_var a, b, _ = üç())",
			"write a name for each element, and _ for one that is not needed (TR_var a, b, _ = üç())",
			2,
		},
	} {
		src := tc.prefix + tc.bad
		singleErrorIn(t, "tr", src, messages.H1026, tc.line, tc.wantTR)
		singleErrorIn(t, "en", src, messages.H1026, tc.line, tc.wantEN)
		_, coded := checkErrs(t, src)
		me, ok := messages.As(coded[0])
		if !ok {
			t.Fatalf("messages.Error bekleniyordu: %v", coded[0])
		}
		gotExample, _ := me.Args[3].(string)
		if gotExample != tc.wantExample || !strings.HasSuffix(tc.wantTR, "("+gotExample+")") {
			t.Fatalf("öneri %q bekleniyordu, alınan %q", tc.wantExample, gotExample)
		}
		// Önerilen yazım denetimden hatasız geçer.
		noErrors(t, tc.prefix+gotExample)
	}
	for _, src := range []string{
		"TR_var a, b = (1, 2)",
		iki + "TR_var a, b = iki()",
		iki + "TR_var g = iki\nTR_var a, b, c = g()",
		"TR_func f() -> (TR_Int, TR_Int)? { TR_return TR_none }\nTR_var a, b, c = f()",
		"TR_func f() -> (TR_Int) { TR_return 1 }\nTR_var a, b = f()",
		"TR_var a, b, c = [1, 2]",
		"TR_var a, b = 1, 2",
	} {
		noErrors(t, src)
	}
	// Dönüş tipi yazılmamış fonksiyonun açılması sayıdan hata vermez; tek hata kapınındır (6.1)
	singleError(t, "TR_func f() { TR_return 1, 2 } # ERROR H1112\nTR_var a, b, c = f()", messages.H1112, 1, "(TR_Int, TR_Int)")
}

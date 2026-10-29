package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (5.3, 6.3): sağ yanı bir demet yazımı ya da dönüşü '-> (A, B)' diye yazılmış fonksiyonun
// doğrudan çağrısı olan açmada öğe sayısı ad sayısını tutmazsa denetim hatasıdır. Sayının bilinmediği
// yerler (fonksiyon değeri, dönüş tipi yazılmamış fonksiyon, isteğe bağlı demet, liste) denetimden
// geçer; sayıyı çalışırken H2709 ve H2700 denetler.
func TestUnpackCountOfAKnownTuple(t *testing.T) {
	const iki = "TR_func iki() -> (TR_Int, TR_Int) { TR_return 1, 2 }\n"
	singleError(t, "TR_var a, b, c = (1, 2)", messages.H1026, 1, "2", "3")
	singleError(t, iki+"TR_var a, b, c = iki()", messages.H1026, 2, "2", "3")
	singleError(t, iki+"TR_var a = 0\nTR_var b = 0\nTR_var c = 0\na, b, c = iki()", messages.H1026, 5)
	singleError(t, "TR_func üç() -> (TR_Int, (TR_Int, TR_Int), TR_Int) { TR_return 1, (2, 3), 4 }\nTR_var a, b = üç()", messages.H1026, 2, "3", "2")
	for _, src := range []string{
		"TR_var a, b = (1, 2)",
		iki + "TR_var a, b = iki()",
		iki + "TR_var g = iki\nTR_var a, b, c = g()",
		"TR_func f() { TR_return 1, 2 }\nTR_var a, b, c = f()",
		"TR_func f() -> (TR_Int, TR_Int)? { TR_return TR_none }\nTR_var a, b, c = f()",
		"TR_func f() -> (TR_Int) { TR_return 1 }\nTR_var a, b = f()",
		"TR_var a, b, c = [1, 2]",
		"TR_var a, b = 1, 2",
	} {
		noErrors(t, src)
	}
}

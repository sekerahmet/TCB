package interp

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.7a "Kurmak"): beklenen tipi TR_JSON olan yerdeki liste ve sözlük değişmezi, iç içe
// değişmezleriyle, JSON değeri olarak kurulur: bildirimde, varsayılanda, parametrede, dönüşte, alanda
// ve sonraki atamada. JSON değeri JSON'la karşılaştırılır (sözlük kalsaydı bu H2661 olurdu).
func TestAJSONPlaceBuildsItsLiteralAsJSON(t *testing.T) {
	const source = `TR_var g: TR_JSON = {"a": 1, "l": [1, "x", TR_none], "o": {"k": TR_true}}
TR_print(g == TR_JSON({"a": 1, "l": [1, "x", TR_none], "o": {"k": TR_true}}))
TR_func f(j: TR_JSON) -> TR_Bool {
    TR_return j == TR_JSON({"a": 1})
}
TR_print(f({"a": 1}))
TR_func d(j: TR_JSON = [1]) -> TR_Bool {
    TR_return j == TR_JSON([1])
}
TR_print(d())
TR_func r() -> TR_JSON {
    TR_return [1, 2.5]
}
TR_print(r() == TR_JSON([1, 2.5]))
TR_struct P {
    j: TR_JSON
}
TR_var p = P(j: {"a": 1})
TR_print(p.j == TR_JSON({"a": 1}))
p.j = ["b"]
TR_print(p.j == TR_JSON(["b"]))
g = {"b": 2}
TR_print(g == TR_JSON({"b": 2}))
TR_var m: TR_JSON? = TR_none
TR_print(m)
`
	out, _, err := runTROut(t, source)
	want := strings.Repeat("true\n", 7) + "none\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (3.13): TR_Float beklenen yerde tam sayı ondalıktır (r: TR_Float = 1 → 1.0): bildirimde,
// parametrede (argüman ve varsayılan; TR_Int değişkenin değeri de), dönüşte, alanda ve sonraki atamada.
// Tipi yazılmamış değişken ilk değerinin tipindedir: TR_var i = 1 TR_Int kalır.
func TestAFloatPlaceMakesAnIntegerDecimal(t *testing.T) {
	const source = `TR_var r: TR_Float = 1
TR_func f(x: TR_Float) -> TR_Float {
    TR_return x
}
TR_func h(x: TR_Float = 3) -> TR_Type {
    TR_return TR_Type(x)
}
TR_func d() -> TR_Float {
    TR_return 4
}
TR_struct P {
    x: TR_Float
}
TR_var p = P(x: 5)
TR_var i = 1
TR_print(TR_Type(r), TR_Type(f(2)), TR_Type(f(i)), h(), TR_Type(d()), TR_Type(p.x), TR_Type(i))
TR_print(1.0, r, f(2))
r = 6
p.x = 7
TR_var o: TR_Float? = 8
TR_print(r, p.x, o)
r += 1
TR_print(TR_Type(r), r)
`
	out, _, err := runTROut(t, source)
	const want = "Float Float Float Float Float Float Int\n1.0 1.0 2.0\n6.0 7.0 8.0\nFloat 7.0\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (3.6): öğelerinin hepsi sayı yazımı olan ve birinde ondalık bulunan liste değişmezinde tam
// sayı yazımları ondalıktır. Öğelerden biri sayı yazımı değilse ([1, x]) liste olduğu gibi kurulur.
func TestADecimalInAListLiteralMakesItsIntegersDecimal(t *testing.T) {
	out, _, err := runTROut(t, "TR_var x = 2\nTR_print([1, -2, 2.5], [1, 2], [1, x, 2.5])\n")
	const want = "[1.0, -2.0, 2.5] [1, 2] [1, 2, 2.5]\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (3.7a "Kurmak"): JSON'a giremeyen öğe (TR_inf) kurarken çalışma hatasıdır, satırıyla.
func TestAJSONPlaceRejectsInfinity(t *testing.T) {
	_, _, err := runTROut(t, "TR_var a = 1\nTR_var g: TR_JSON = [1, TR_inf]\n")
	var line *LineError
	if !messages.Has(err, messages.H2653) || !errors.As(err, &line) || line.Line != 2 {
		t.Fatalf("2. satırda H2653 beklenirdi, gelen %v", err)
	}
}

package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (1.2, 3.13): açıklamasız yerel değişkenin tipi ilk değerinden çıkar ve değişmez; sonraki
// atama o tipe uyar. Tam sayı değişkene ondalık atanamaz; ondalık değişkene tam sayı atanabilir.
func TestLocalTypeIsPermanent(t *testing.T) {
	singleError(t, "TR_var toplam = 0\ntoplam += 0.5", messages.H1005, 2, `"toplam" değişkeni (TR_Int)`, `"TR_Float"`)
	singleError(t, "TR_var x = 5\nx = \"beş\"", messages.H1005, 2, `"x" değişkeni (TR_Int)`, `"TR_String"`)
	singleError(t, "TR_var m = \"a\"\nTR_func f() {\n    m = TR_true\n}", messages.H1005, 3, `"m" değişkeni (TR_String)`, `"TR_Bool"`)
	singleError(t, "TR_var x = 10\nx /= 4", messages.H1005, 2, `"TR_Float"`, "'//'")
	noErrors(t, "TR_var toplam = 0.0\ntoplam += 1\ntoplam = 2")
	noErrors(t, "TR_var s = \"a\"\ns += 1\ns = \"b\"")
	noErrors(t, "TR_var n = 0\nn += 1\nn = n * 2 - 1\nn //= 2")
}

// Kural (yanlış alarm vermez): tipi çıkarılamayan ilk değer (bilinmeyen çağrı, TR_none) ve
// denetleyicinin tanımadığı tip (ör. TR_JSON; tip parametresi, arayüz) değişkene verilmez; değişken TR_Any kalır.
func TestLocalTypeUnknownStaysAny(t *testing.T) {
	noErrors(t, "TR_var a = TR_none\na = 5\na = \"x\"")
	noErrors(t, "TR_var a = TR_input()\na = 5\na = \"x\"")
	noErrors(t, "TR_func oku() -> TR_JSON {\n    TR_return TR_json_parse(\"1\")\n}\nTR_var y = oku()\ny = \"a\"")
}

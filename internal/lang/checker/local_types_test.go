package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// firstValueType kaynağın son deyimi olan TR_var'ın ilk değerinin çıkarılan tipidir.
func firstValueType(t *testing.T, source string) string {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.CheckProgram(prog)
	last, ok := prog.Statements[len(prog.Statements)-1].(*ast.VarStatement)
	if !ok {
		t.Fatalf("son deyim TR_var değil:\n%s", source)
	}
	return c.valueTypes[last.Value]
}

// expectTypes her kaynağın son TR_var'ının ilk değerinin tipini sınar.
func expectTypes(t *testing.T, cases map[string]string) {
	t.Helper()
	for source, want := range cases {
		if got := firstValueType(t, source); got != want {
			t.Errorf("tip %q beklenirdi, gelen %q\nkaynak:\n%s", want, got, source)
		}
	}
}

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
	noErrors(t, "TR_var a = TR_json_parse(\"1\")\na = 5\na = \"x\"")
	noErrors(t, "TR_func oku() -> TR_JSON {\n    TR_return TR_json_parse(\"1\")\n}\nTR_var y = oku()\ny = \"a\"")
}

// Kural (4.2, 4.10, 15.6): değişmezin, aritmetiğin ve karşılaştırmanın tipi çalışma anının
// kurallarıyla çıkar; işlenenin tipi bilinmiyorsa ya da işlem tanımsızsa sonuç TR_Any'dir.
func TestOperatorTypes(t *testing.T) {
	const n, f, s, ten = "TR_var n = 2\n", "TR_var f = 0.5\n", "TR_var s = \"a\"\n", "TR_var t = TR_Tensor([1.0]) @ TR_Tensor([1.0])\n"
	expectTypes(t, map[string]string{
		"TR_var v = 1":                        typeInt,
		"TR_var v = 1.5":                      typeFloat,
		"TR_var v = \"a\"":                    typeString,
		"TR_var v = TR_true":                  typeBool,
		n + "TR_var v = -n":                   typeInt,
		f + "TR_var v = -f":                   typeFloat,
		s + "TR_var v = -s":                   typeAny,
		n + "TR_var v = TR_not n > 1":         typeBool,
		n + "TR_var v = n / 2":                typeFloat,
		n + "TR_var v = n // 2":               typeInt,
		n + f + "TR_var v = n * f":            typeFloat,
		n + "TR_var v = n == 2":               typeBool,
		n + "TR_var v = n > 1 TR_and n < 3":   typeBool,
		s + "TR_var v = s * 3":                typeString,
		s + "TR_var v = 3 * s":                typeString,
		"TR_var v = [1] + [2]":                listType(typeInt),
		"TR_var v = [1] + [\"a\"]":            typeList,
		"TR_var v = [0] * 4":                  listType(typeInt),
		ten + "TR_var v = t * 2":              typeTensor,
		ten + "TR_var v = 1.0 - t":            typeTensor,
		ten + "TR_var v = -t":                 typeTensor,
		s + "TR_var v = s - 1":                typeAny,
		"TR_var v = TR_json_parse(\"1\") + 1": typeAny,
	})
}

// shapes yapı ve arayüz testlerinin ortak tanımlarıdır.
const shapes = `TR_interface Şekil {
    TR_func alan() -> TR_Float
}
TR_struct Kare {
    a: TR_Float = 1.0
    TR_func alan() -> TR_Float { TR_return TR_self.a * TR_self.a }
    TR_func adı() { TR_print("kare") }
}
TR_struct Nokta {
    x: TR_Int = 0
}
`

// Kural (7.2, 7.3): yapı kurucusunun çağrısı yapının kendisidir; tipi bilinen yapı nesnesinin
// metodu yazılan dönüş tipini verir. Yapının adı değer olarak bir tip değeridir (3.11).
func TestStructCallTypes(t *testing.T) {
	expectTypes(t, map[string]string{
		shapes + "TR_var v = Kare(2.0)":                      "Kare",
		shapes + "TR_var k = Kare(2.0)\nTR_var v = k.alan()": typeFloat,
		shapes + "TR_var k = Kare(2.0)\nTR_var v = k.adı()":  typeAny,
		shapes + "TR_var k = Kare(2.0)\nTR_var v = k.yok()":  typeAny,
		shapes + "TR_var v = Kare":                           typeType,
	})
	singleError(t, shapes+"TR_var p = Nokta(1)\np = Kare(1.0)", messages.H1005, 13, `"p" değişkeni (Nokta)`, `"Kare"`)
	singleError(t, shapes+"TR_var n: TR_Int = Kare(1.0)", messages.H1001, 12, `"TR_Int"`, `"Kare"`)
	singleError(t, shapes+"TR_var k = Kare(1.0)\nTR_var n: TR_String = k.alan()", messages.H1001, 13, `"TR_String"`, `"TR_Float"`)
}

// Kural (7.8, yanlış alarm vermez): arayüz tipi beklenen yere onu karşılayabilecek bir yapı verilir;
// denetleyici arayüzün karşılanıp karşılanmadığına (bu dilimde) karar vermez.
func TestStructMeetsInterface(t *testing.T) {
	noErrors(t, shapes+"TR_var ş: Şekil = Kare(1.0)\nTR_func ölç(x: Şekil) -> TR_Float {\n    TR_return x.alan()\n}\nTR_print(ölç(Kare(2.0)))")
	noErrors(t, shapes+"TR_func al() -> Şekil {\n    TR_return Kare(1.0)\n}")
}

// Kural (3.6, 3.7): liste ve sözlük değişmezinin tipi öğe tipini taşır; boş, karışık ya da öğesi
// bilinmeyen değişmez öğe tipsizdir. TR_none öğeli değişmezin öğe tipi isteğe bağlıdır (3.5). Değişmezde ondalık varsa tam sayı yazımları ondalıktır; tam
// sayı değişkeni ondalık olmaz (çalışırken TR_Int kalır), o değişmez karışıktır.
func TestCollectionLiteralTypes(t *testing.T) {
	str := typeString
	expectTypes(t, map[string]string{
		"TR_var v = [1, 2]":                               listType(typeInt),
		"TR_var v = [1, 2.5]":                             listType(typeFloat),
		"TR_var v = [-1, 2.5]":                            listType(typeFloat),
		"TR_var a = 1\nTR_var v = [a, 2.5]":               typeList,
		"TR_var b = 2\nTR_var v = {\"a\": b, \"c\": 0.5}": typeDict,
		"TR_var v = [[1], [2]]":                           listType(listType(typeInt)),
		"TR_var v = []":                                   typeList,
		"TR_var v = [1, \"a\"]":                           typeList,
		"TR_var v = [1, TR_none]":                         listType(typeInt + "?"),
		"TR_var v = [1.0, TR_none, 2]":                    listType(typeFloat + "?"),
		"TR_var v = [TR_none, TR_none]":                   typeList,
		"TR_var v = [TR_json_parse(\"1\")]":               typeList,
		"TR_var v = {\"a\": 1}":                           containerType(typeDict, []string{str, typeInt}),
		"TR_var v = {1: [\"a\"]}":                         containerType(typeDict, []string{typeInt, listType(str)}),
		"TR_var v = {}":                                   typeDict,
		"TR_var v = {\"a\": 1, \"b\": \"x\"}":             typeDict,
	})
}

// Kural (3.6, 3.13): kap tip argümanında değişmezdir: TR_List[TR_Int] TR_List[TR_String] ya da
// TR_List[TR_Float] beklenen yere verilmez; değişmezdeki tam sayı yazımı ondalık beklenen yerde
// ondalıktır; öğe tipsiz kap (boş değişmez) asıl adıyla karşılaştırılır.
func TestCollectionElementTypes(t *testing.T) {
	singleError(t, `TR_var l: TR_List[TR_Int] = ["a"]`, messages.H1001, 1, `"TR_List[TR_Int]"`, `"TR_List[TR_String]"`)
	singleError(t, "TR_var a = [1, 2]\nTR_var l: TR_List[TR_Float] = a", messages.H1001, 2, `"TR_List[TR_Float]"`, `"TR_List[TR_Int]"`)
	singleError(t, "TR_var l = [1]\nl = [\"a\"]", messages.H1005, 2, `"l" değişkeni (TR_List[TR_Int])`)
	singleError(t, `TR_var d: TR_Dict[TR_String, TR_Int] = {"a": "b"}`, messages.H1001, 1, `"TR_Dict[TR_String, TR_String]"`)
	singleError(t, "TR_func f(l: TR_List[TR_Int]) { }\nf([\"a\"])", messages.H1004, 2, `"TR_List[TR_String]"`)
	singleErrorIn(t, "en", `TR_var l: TR_List[TR_Int] = ["a"]`, messages.H1001, 1, "List[Int]", "List[String]")
	noErrors(t, "TR_var l: TR_List[TR_Int] = []\nl = [1, 2]\nTR_var f: TR_List[TR_Float] = [1, 2]\nTR_var d: TR_Dict[TR_String, TR_Float] = {\"a\": 1}")
	noErrors(t, "TR_var l = [1]\nl = []\nl += [2]\nl = l + [3]\nl = l * 2")
	noErrors(t, "TR_func f(l: TR_List[TR_Int]) { }\nf([1])\nf([])\nTR_var m = [TR_json_parse(\"1\")]\nf(m)")
	noErrors(t, shapes+"TR_var l: TR_List[Şekil] = [Kare(1.0)]")
}

// Kural (3.6a): demetin tipi öğelerinin tipleridir; t.0 öğenin tipini verir. Öğesinin tipi
// bilinmeyen demet TR_Any'dir; demet tipi yazılan dönüş de demettir.
func TestTupleTypes(t *testing.T) {
	pair := tupleType([]string{typeInt, typeString})
	const f = "TR_func f() -> (TR_Int, TR_String) {\n    TR_return 1, \"a\"\n}\n"
	expectTypes(t, map[string]string{
		"TR_var v = (1, \"a\")":                            pair,
		"TR_var v = (1, (2.5, TR_true))":                   tupleType([]string{typeInt, tupleType([]string{typeFloat, typeBool})}),
		"TR_var v = (1, TR_json_parse(\"1\"))":             typeAny,
		"TR_var p = (1, \"a\")\nTR_var v = p.1":            typeString,
		"TR_var p = (1, (2.5, TR_true))\nTR_var v = p.1.0": typeFloat,
		"TR_var p = (1, \"a\")\nTR_var v = p.2":            typeAny,
		f + "TR_var v = f()":                               pair,
		f + "TR_var v = f().0":                             typeInt,
	})
	singleError(t, "TR_var p = (1, \"a\")\np = (\"a\", 1)", messages.H1005, 2, `(TR_Int, TR_String)`, `(TR_String, TR_Int)`)
	singleError(t, "TR_var p: (TR_Int, TR_Int) = (1, 2, 3)", messages.H1001, 1, `(TR_Int, TR_Int, TR_Int)`)
	singleError(t, "TR_func f() -> (TR_Int, TR_String) {\n    TR_return \"a\", 1\n}", messages.H1003, 2, `(TR_String, TR_Int)`)
	singleError(t, "TR_var n: TR_Int = (1, 2)", messages.H1001, 1, `(TR_Int, TR_Int)`)
	singleErrorIn(t, "en", "TR_var p = (1, \"a\")\np = (\"a\", 1)", messages.H1005, 2, "(Int, String)", "(String, Int)")
	noErrors(t, f+"TR_var p = f()\np = (2, \"b\")\nTR_var q: (TR_Float, TR_String) = (1, \"c\")\nTR_var a, b = f()")
}

// Kural (3.5, 4.7): isteğe bağlı tip T? olarak taşınır: TR_none ilk değerli ve tipi yazılı değişken
// T?'dir, '??'nin sonucu asıl tiptir (T), TR_none yalnız isteğe bağlı yere verilir. İsteğe bağlı
// değerin asıl tipin yerinde kullanılması daraltmayla (4b) denetlenir; bugün serbesttir.
func TestOptionalTypes(t *testing.T) {
	expectTypes(t, map[string]string{
		"TR_var x: TR_Int? = TR_none\nTR_var v = x":           typeInt + "?",
		"TR_var x: TR_Int? = TR_none\nTR_var v = x ?? 0":      typeInt,
		"TR_var x: TR_String? = \"a\"\nTR_var v = x ?? \"b\"": typeString,
		"TR_var v = TR_none ?? 2.5":                           typeFloat,
	})
	singleError(t, "TR_var a: TR_Int = TR_none", messages.H1001, 1, `"TR_NoneType"`)
	singleError(t, "TR_var a = 0\na = TR_none", messages.H1005, 2, `"TR_NoneType"`)
	singleError(t, "TR_func f(x: TR_Int) { }\nf(TR_none)", messages.H1004, 2)
	noErrors(t, "TR_var a: TR_Int? = 0\na = TR_none\nTR_func f(x: TR_Int?) { }\nf(TR_none)\nf(a)")
	noErrors(t, "TR_var a: TR_Int? = 0\nTR_var b: TR_Int = a") // daraltma 4b'nin
}

// Kural (7.3, 7.5, 3.13): tipi bilinen yapı nesnesinin alanı yazılan tipindedir; metotta TR_self
// yapının tipindedir (alanı ve metodunun dönüşü bilinir). Tipsiz alan, metot değeri, ?. ve tipi
// bilinmeyen nesne TR_Any kalır.
func TestFieldTypes(t *testing.T) {
	nokta := "TR_struct Nokta {\n    x: TR_Float\n    ad: TR_String? = TR_none\n    TR_func iki() -> TR_Float {\n        TR_var v = TR_self.x * 2\n        TR_return v\n    }\n}\nTR_var p = Nokta(1.0)\n"
	expectTypes(t, map[string]string{
		nokta + "TR_var v = p.x":                           typeFloat,
		nokta + "TR_var v = p.ad":                          typeString + "?",
		nokta + "TR_var v = p.iki()":                       typeFloat,
		nokta + "TR_var v = p.y":                           typeAny,
		nokta + "TR_var v = p?.x":                          typeAny,
		"TR_var p = TR_json_parse(\"1\")\nTR_var v = p.x":  typeAny,
		"TR_enum D {\n    A(n: TR_Int)\n}\nTR_var v = D.A": typeAny,
	})
	singleError(t, nokta+"TR_var v = p.x\nv = \"a\"", messages.H1005, 11, "TR_Float")
	singleError(t, "TR_struct S {\n    n: TR_Int\n    TR_func m() -> TR_String {\n        TR_return TR_self.n\n    }\n}", messages.H1003, 4, `"TR_Int"`)
}

// Kural (3.6, 3.7, 15.7): tipi bilinen kabın indeksi öğe tipindedir: listenin tam sayıyla indeksi öğe,
// aralıkla dilimi listenin kendisi; metnin indeksi ve dilimi metin; sözlüğün indeksi değer tipi;
// tensörün indeksi tensör. İsteğe bağlı ya da öğe tipi bilinmeyen kap TR_Any.
func TestIndexTypes(t *testing.T) {
	expectTypes(t, map[string]string{
		"TR_var l = [1, 2]\nTR_var v = l[0]":                      typeInt,
		"TR_var l = [[1.5], [2.0]]\nTR_var v = l[-1]":             listType(typeFloat),
		"TR_var l = [1, 2]\nTR_var v = l[0..1]":                   listType(typeInt),
		"TR_var s = \"abc\"\nTR_var v = s[1]":                     typeString,
		"TR_var s = \"abc\"\nTR_var v = s[1..2]":                  typeString,
		"TR_var d = {\"a\": 1.5}\nTR_var v = d[\"a\"]":            typeFloat,
		"TR_var t: TR_Tensor = TR_Tensor([1.0])\nTR_var v = t[0]": typeTensor,
		"TR_var l = [1, \"a\"]\nTR_var v = l[0]":                  typeAny,
		"TR_var l: TR_List[TR_Int]? = [1]\nTR_var v = l[0]":       typeAny,
	})
	singleError(t, "TR_var l = [1, 2]\nTR_var v = l[0]\nv = \"a\"", messages.H1005, 3, "TR_Int")
}

// Kural (5.7, 5.11): döngü değişkeni dolaşılan kaynağın öğe tipindedir: listede öğe, aralıkta tam
// sayı, metinde karakter, sözlükte anahtar; iki değişkenle indeks ve öğe, sözlükte anahtar ve değer.
// Liste üretecinin değişkeni de. Bilinmeyen kaynak TR_Any.
func TestLoopVariableTypes(t *testing.T) {
	for head, want := range map[string]string{
		"x TR_in [1.5, 2.0]":    "TR_Float",
		"x TR_in 0..3":          "TR_Int",
		"x TR_in \"abc\"":       "TR_String",
		"x TR_in {\"a\": 1}":    "TR_String",
		"k, x TR_in {\"a\": 1}": "TR_Int",
		"x, y TR_in [\"a\"]":    "TR_Int",
		"i, x TR_in [\"a\"]":    "TR_String",
	} {
		singleError(t, "TR_for "+head+" {\n    TR_var s: TR_Bool = x\n}", messages.H1001, 2, `"`+want+`"`)
	}
	noErrors(t, "TR_for x TR_in TR_json_parse(\"1\") {\n    TR_var s: TR_Bool = x\n}")
	noErrors(t, "TR_var l = [1, \"a\"]\nTR_for x TR_in l {\n    TR_var s: TR_Bool = x\n}")
	singleError(t, "TR_func f(n: TR_Int) -> TR_Int {\n    TR_return n\n}\nTR_var l = [f(x) TR_for x TR_in [\"a\"]]", messages.H1004, 4, `"TR_String"`)
}

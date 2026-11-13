package internal_test

import (
	"bytes"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/lib"
)

// suggestionCase bir iletinin önerdiği yazımın hem iletide göründüğünü hem de hatalı programa
// uygulandığında programın denetimden ve çalıştırmadan hatasız geçtiğini sınar.
type suggestionCase struct {
	code       messages.Code
	wrong      string
	replace    string
	suggestion string
}

var suggestionCases = []suggestionCase{
	{messages.H0018, "TR_var x: TR_Int = 0X1F", "0X1F", "0x1F"},
	{messages.H0004, "TR_const TR_PI2 = 6.28", "TR_PI2", "PI2"},
	{messages.H0004, "TR_struct TR_Foo { x: TR_Int = 0 }", "TR_Foo", "Foo"},
	{messages.H0004, "TR_func TR_sum(x: TR_Int) -> TR_Int { TR_return x }", "TR_sum", "sum"},
	{messages.H0004, "TR_var TR_yok = 1", "TR_yok", "yok"},
	{messages.H0026, "TR_pritn(1)", "TR_pritn", "TR_print"},
	{messages.H0032, "TR_var x: TR_Float = .5", ".5", "0.5"},
	{messages.H0027, "TR_var x: TR_List[TR_Int] = [1, 2]\nTR_var y: TR_Int = TR_length", "TR_length", "x.TR_length"},
	{messages.H0029, "TR_var x: TR_Int = 1\nTR_var y: TR_Bool = x.TR_true", "x.TR_true", "TR_true"},
	{messages.H0029, "TR_var x: TR_Int = 1\nTR_var y: TR_Int = x.TR_time_ms()", "x.TR_time_ms()", "TR_time_ms()"},
	{messages.H7017, "TR_print(TR_test)", "TR_print(TR_test)", `TR_test "ad" { }`},
	{messages.H7017, "TR_var x = [1]\nTR_print(x.TR_test)", "TR_print(x.TR_test)", `TR_test "ad" { }`},
	{messages.H7017, "TR_print(TR_as)", "TR_print(TR_as)", `TR_import "tr/layers" TR_as ad`},
	{messages.H7017, "TR_print(TR_init)", "TR_print(TR_init)", "TR_struct S { TR_func TR_init() { } }"},
	{messages.H7016, "TR_var x = [1, 2]\nTR_print(x.TR_sum())", "x.TR_sum()", "TR_sum(x)"},
	{messages.H7016, "TR_var x = [1, 2]\nTR_var y = [1, 2]\nx.TR_assert_equal(y)", "x.TR_assert_equal(y)", "TR_assert_equal(x, y)"},
	{messages.H0037, "TR_var x: TR_type_Int = 1", "TR_type_Int", "TR_Int"},
	{messages.H0037, "TR_var x: TR_List[TR_Int] = [1, 2]\nTR_var y: TR_Int = x.TR_mem_length", ".TR_mem_length", ".TR_length"},
	{messages.H0037, "TR_kw_var x: TR_Int = 1", "TR_kw_var", "TR_var"},
	{messages.H0037, "TR_var t: TR_Tensor = TR_zeros([1], TR_arg_requires_grad: TR_true)", "TR_arg_requires_grad:", "TR_requires_grad:"},
	{messages.H1007, "TR_var a: ondalik = 1.5", "ondalik", "TR_Float"},
	{messages.H1012, "TR_var öğrenciSayısı: TR_Int = 1", "öğrenciSayısı", "öğrenci_sayısı"},
	{messages.H1013, "TR_const en_büyük = 10", "en_büyük", "EN_BÜYÜK"},
	{messages.H1013, "TR_const tr_x = 10", "tr_x", "X"},
	{messages.H1014, "TR_struct önbellek_durumu { x: TR_Int = 0 }", "önbellek_durumu", "ÖnbellekDurumu"},
	{messages.H1015, "TR_var ADIM: TR_Int = 300", "ADIM", "adım"},
	{messages.H1026, "TR_var a, b, c = (1, 2)", "TR_var a, b, c = (1, 2)", "TR_var a, b = (1, 2)"},
	{messages.H1026, "TR_var t = (1, 2)\nTR_var a, b, c = t\nTR_print(a, b)", "TR_var a, b, c = t", "TR_var a, b = t"},
	{messages.H1105, `TR_var t: TR_Bool = TR_Type(1) == "Int"`, `"Int"`, "TR_Int"},
	{messages.H1106, "TR_var sayac: TR_Int = 1\nTR_print(sayaç)", "sayaç", "sayac"},
	{messages.H2410, "TR_func f(isim: TR_Int) -> TR_Int { TR_return isim }\nTR_var x: TR_Int = f(isimm: 1)", "isimm", "isim"},
	{messages.H2418, "TR_struct Nokta { konum: TR_Int = 0 }\nTR_var n: Nokta = Nokta(konumm: 1)", "konumm", "konum"},
	{messages.H2437, "TR_var sayac: TR_Int = 1\nsayaç += 2", "sayaç", "sayac"},
	{messages.H2438, "TR_var sayac: TR_Int = 1\nsayaç = 2", "sayaç", "sayac"},
	{messages.H2478, `TR_import "tr/layer"`, "tr/layer", "tr/layers"},
	{messages.H2709, "TR_func g() -> (TR_Int, TR_String) { TR_return 1, \"x\" }\nTR_var f = g\nTR_var a, b, c = f()", "TR_var a, b, c = f()", "TR_var a, b = f()"},
	{messages.H2767, "TR_enum Sonuç { Tamam(değer: TR_Int), Boş }\nTR_var s: Sonuç = Sonuç.Tamam(deger: 1)", "deger", "değer"},
	{messages.H2778, "TR_var t = (10, \"x\")\nTR_var v: TR_Int = t[0]", "t[0]", "t.0"},
	{messages.H6104, "TR_func f(x: TR_List[TR_Float]) -> TR_List[TR_Float] { TR_return x }\nTR_var y = TR_relu(f([1.0, 2.0]))", "TR_relu(f", "TR_relu(TR_Tensor"},
	{messages.H6955, "TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)\nTR_var h = w * 2.0\nTR_var m = TR_Tensor([TR_false, TR_true])\nTR_var d = 0.0\nh[1] = d", "h[1] = d", "h = TR_where(m, d, h)"},
	{messages.H6956, "TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)\nTR_var t = TR_zeros([2])\nTR_var i = 0\nTR_var d = (w * 2.0)[0]\nt[i] = d", "t[i] = d", "TR_no_grad { t[i] = d }"},
	{messages.H6956, "TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)\nTR_var t = TR_zeros([2])\nTR_var m = TR_Tensor([TR_true, TR_false])\nTR_var d = w * 2.0\nt[0..2] = d", "t[0..2] = d", "t = TR_where(m, d, t)"},
	{messages.H6955, "TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)\nTR_var h = w * 2.0\nTR_var d = 1.0\nTR_no_grad { h += d }", "h += d", "h = h + d"},
	{messages.H6958, "TR_var t = TR_zeros([1])\nTR_var u = TR_ones([3])\nt += u", "t += u", "t = t + u"},
	{messages.H6402, "TR_var x = TR_Tensor([3.0], TR_requires_grad: TR_false)\nTR_backward(TR_sum(x * 2.0))", "TR_requires_grad: TR_false", "TR_requires_grad: TR_true"},
	{messages.H6405, "TR_var f = TR_custom_grad((x) => [x * x, [x]])", "TR_custom_grad((x) => [x * x, [x]])", "TR_custom_grad(TR_forward: (x) => [x * x, [x]], TR_backward: (s, g) => [g * 2 * s[0]])"},
	{messages.H6430, "TR_var f = TR_custom_grad(TR_forward: (x) => [x * x, [x]])", "TR_custom_grad(TR_forward: (x) => [x * x, [x]])", "TR_custom_grad(TR_forward: (x) => [x * x, [x]], TR_backward: (s, g) => [g * 2 * s[0]])"},
	{messages.H6431, "TR_var f = TR_custom_grad()", "TR_custom_grad()", "TR_custom_grad(TR_forward: (x) => [x * x, [x]], TR_backward: (s, g) => [g * 2 * s[0]])"},
	{messages.H6514, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_true) * 1.0\nTR_var opt = TR_AdamW([w])", "TR_requires_grad: TR_true) * 1.0", "TR_requires_grad: TR_true)"},
	{messages.H6995, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_false)\nTR_var opt = TR_AdamW([w])", "TR_requires_grad: TR_false", "TR_requires_grad: TR_true"},
	{messages.H6520, "TR_var m = {\"w\": TR_Tensor([1.0], TR_requires_grad: TR_false)}\nTR_load_parameters(m, {\"w\": TR_Tensor([2.0])})", "TR_requires_grad: TR_false", "TR_requires_grad: TR_true"},
	{messages.H6700, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_true) * 1.0\nTR_var g = w.TR_grad", "TR_requires_grad: TR_true) * 1.0", "TR_requires_grad: TR_true)"},
	{messages.H6811, `TR_var t: TR_Tensor = TR_zeros([1], TR_dtype: "float")`, "float", "float32"},
	{messages.H7229, `TR_var ayar = TR_command_line({"adım": 1}, TR_descriptions: {"adim": "x"})`, "adim", "adım"},
	{messages.H7730, `TR_var w: TR_Bool = TR_machine().TR_os == "darwin"`, `"darwin"`, `"macos"`},
	{messages.H7731, `TR_var x: TR_Bool = ["arm64", "amd64"].TR_contains(TR_machine().TR_arch)`, `"amd64"`, `"x86_64"`},
	{messages.H7734, `TR_print(TR_device_info("cpu"))`, `TR_device_info("cpu")`, "TR_machine().TR_cpu_features"},
	{messages.H7737, `TR_var y: TR_Int = TR_Path("olmayan_model.bin").TR_disk_available_bytes`, `TR_Path("olmayan_model.bin").TR_disk_available_bytes`, `TR_Path(".").TR_disk_available_bytes`},
	{messages.H7900, "TR_var s = TR_sum(TR_Tensor([1.0, 2.0]), axis: 0)", "axis", "TR_dim"},
	{messages.H7903, "TR_var t = TR_Tensor([1.0]).tür_", "tür_", "TR_dtype"},
	{messages.H1109, "TR_func f(b = 0) -> TR_Int { # ERROR H1109\n    TR_return b\n}\nTR_print(f())", "f(b = 0)", "f(b: TR_Int = 0)"},
	{messages.H1110, "TR_func f(a) -> TR_Int { # ERROR H1110\n    TR_return a\n}\nTR_print(f(1))", "f(a)", "f(a: TR_Int)"},
	{messages.H1112, "TR_func f(a: TR_Int) { # ERROR H1112\n    TR_return a\n}\nTR_print(f(1))", "f(a: TR_Int) {", "f(a: TR_Int) -> TR_Int {"},
	{messages.H1115, "TR_var l = [3, 1, 2]\nl.TR_sort()\nTR_print(l)", "l.TR_sort()", "l = l.TR_sort()"},
	{messages.H2474, "TR_var l = [\"a\", \"b\"]\nTR_print(\" \".TR_join(l))", "\" \".TR_join(l)", "l.TR_join(\" \")"},
	{messages.H2474, "TR_var l = [1, 2]\nTR_print(l.pop())", "pop", "TR_pop"}, // tanıdık ad, dosyada kimlikle (DENETİM istek 125 N2)
	{messages.H0299, "TR_var x = 1\nTR_func f() -> TR_Int {\n    TR_defer { TR_return 5 }\n    TR_return 1\n}\nTR_print(f())", "TR_defer { TR_return 5 }", "TR_defer { TR_if x > 0 { TR_print(x) } }"},
	{messages.H0300, "TR_func f() {\n    TR_defer TR_print(1)\n}\nf()", "TR_defer TR_print(1)", "TR_defer { TR_print(1) }"},
	{messages.H2910, "TR_try { TR_throw \"x\" } TR_catch { TR_print(1) }", "TR_catch", "TR_catch e"},
	// H2911'in TR_throw'daki önerisi (TR_throw TR_String(sayı)) düzeltilince de fırlatır: tabloya sığmaz,
	// TestNotThrowableSuggestsTheProgramsSpelling (interp) sınar. Burada TR_Error(...) yolu: öneri adı anar.
	{messages.H2911, "TR_var sayı = [1]\nTR_var e = TR_Error(sayı)\nTR_print(e)", "TR_Error", "TR_String"},
	// Öneri programın kendi adlarıyladır (ilk, son; e, f).
	{messages.H2913, "TR_var ilk = TR_Error(\"a\")\nTR_var son = TR_Error(\"b\")\nTR_print(ilk == son)", "ilk == son", "ilk.TR_code == son.TR_code"},
	{messages.H2913, "TR_try { TR_throw \"a\" } TR_catch e {\n    TR_try { TR_throw \"b\" } TR_catch f { TR_print(e != f) }\n}", "e != f", "e.TR_code != f.TR_code"},
	{messages.H2914, "TR_try { raise \"x\" } TR_catch e { TR_print(e) }", "raise", "TR_throw"},
	{messages.H2915, "TR_try { TR_throw \"x\" } except e { TR_print(e) }", "except e", "TR_catch e"},
	{messages.H2916, "TR_try { TR_print(1) } TR_catch e { TR_print(e) }\nfinally { TR_print(2) }", "finally", "TR_defer"},
	{messages.H2917, "TR_try { TR_throw \"x\" } TR_catch (e) { TR_print(e) }", "TR_catch (e)", "TR_catch e"},
	{messages.H2900, "TR_var l = [3, 1, 2]\nTR_print(l.TR_sort((a, b) => b - a))", "l.TR_sort((a, b) => b - a)", "l.TR_sort((x) => -x)"},
	{messages.H1130, "TR_var l: TR_List[TR_Int] = []\nTR_var x: TR_Int = l.TR_first\nTR_print(x)", "l.TR_first", "l.TR_first ?? 0"},
	{messages.H1130, "TR_func f(l: TR_List[TR_Float]) -> TR_Float {\n    TR_return l.TR_get(2)\n}\nTR_print(f([]))", "l.TR_get(2)", "l.TR_get(2) ?? 0.0"},
	{messages.H1130, "TR_var l = [\"a\"]\nTR_var s = \"\"\ns = l.TR_last\nTR_print(s)", "l.TR_last", "l.TR_last ?? \"\""},
	{messages.H1130, "TR_var l: TR_List[TR_List[TR_Int]] = []\nTR_var k: TR_List[TR_Int] = l.TR_first\nTR_print(k)", "l.TR_first", "l.TR_first ?? []"},
	{messages.H1130, "TR_var l: TR_List[TR_Int] = []\nTR_func f(x: TR_Int) -> TR_Int {\n    TR_return x\n}\nTR_print(f(l.TR_first))", "l.TR_first", "l.TR_first ?? 0"},
	{messages.H1130, "TR_struct P { x: TR_Int }\nTR_var l: TR_List[TR_Int] = []\nTR_var p = P(1)\np.x = l.TR_first\nTR_print(p.x)", "l.TR_first", "l.TR_first ?? 0"},
	{messages.H1130, "TR_struct P { x: TR_Float }\nTR_var l: TR_List[TR_Float] = []\nTR_var p = P(l.TR_last)\nTR_print(p.x)", "l.TR_last", "l.TR_last ?? 0.0"},
	{messages.H1130, "TR_var l: TR_List[TR_Int] = []\nTR_var m = [1]\nm.TR_append(l.TR_first)\nTR_print(m)", "l.TR_first", "l.TR_first ?? 0"},
	{messages.H1130, "TR_var l: TR_List[TR_String] = []\nTR_print(\"a,b\".TR_contains(l.TR_first))", "l.TR_first", "l.TR_first ?? \"\""},
	{messages.H1130, "TR_var l: TR_List[TR_Float] = []\nTR_print(TR_abs(l.TR_first))", "l.TR_first", "l.TR_first ?? 0.0"},
	{messages.H1130, "TR_var l: TR_List[TR_String] = []\nTR_print(\"a,b\".TR_split(l.TR_first))", "l.TR_first", "l.TR_first ?? \" \""},
	{messages.H1146, "TR_print(TR_mean(1))", "TR_mean(1)", "TR_mean([1])"},
	{messages.H1146, "TR_var x = 1.5\nTR_print(TR_median(x))", "TR_median(x)", "TR_median([x])"},
	{messages.H1131, "TR_var j = TR_json_parse(`{\"k\": 3}`)\nTR_print(j[\"k\"] == 3)", "j[\"k\"] == 3", "TR_Int(j[\"k\"]) == 3"},
	{messages.H1131, "TR_var j = TR_json_parse(`{\"k\": 3}`)\nTR_print(j[\"k\"] == 3)", "j[\"k\"] == 3", "j[\"k\"] == TR_JSON(3)"},
	{messages.H1131, "TR_var j = TR_json_parse(`[1, 2]`)\nTR_print(TR_List(j) == [1, 2])", "TR_List(j) == [1, 2]", "TR_List(j) == [TR_JSON(1), TR_JSON(2)]"},
	{messages.H1131, "TR_var j = TR_json_parse(`[1, 2]`)\nTR_print(TR_List(j).TR_contains(1))", "TR_List(j).TR_contains(1)", "TR_List(j).TR_contains(TR_JSON(1))"},
	{messages.H1131, "TR_var j = TR_json_parse(`7`)\nTR_print(TR_match j {\n    2 => \"iki\"\n    _ => \"başka\"\n})", "2", "TR_JSON(2)"},
	{messages.H1132, "TR_var j = TR_json_parse(`{\"n\": null}`)\nTR_print(j[\"n\"] == TR_none)", "j[\"n\"] == TR_none", "TR_Type(j[\"n\"]) == TR_NoneType"},
	{messages.H1132, "TR_var j = TR_json_parse(`{\"n\": null}`)\nTR_print(j[\"n\"] == TR_none)", "j[\"n\"] == TR_none", "j.TR_get(\"n\") == TR_none"},
	{messages.H1132, "TR_var j = TR_json_parse(`[1, null]`)\nTR_print(TR_List(j).TR_contains(TR_none))", "TR_List(j).TR_contains(TR_none)", "TR_List(j).TR_contains(TR_JSON(TR_none))"},
	{messages.H1133, "TR_var j: TR_JSON = {\"a\": \"x y\".TR_split(\" \")}\nTR_print(j)", "\"x y\".TR_split(\" \")", "TR_JSON(\"x y\".TR_split(\" \"))"},
	{messages.H1134, "TR_var j = TR_json_parse(`{}`)\nTR_print(TR_Type(j) == TR_JSON)", "TR_Type(j) == TR_JSON", "TR_Type(j) == TR_Dict"},
	{messages.H1135, "TR_var l: TR_List[TR_Int] = []\nTR_var m = [1]\nm[0] = l.TR_first\nTR_print(m)", "l.TR_first", "l.TR_first ?? 0"},
	// 4.7: '??'nin sağ yanı sol yanın asıl tipinde (H1030'un ipucu H1032 ya da H1033); isteğe bağlı
	// olmayan sol yanın '??'si silinir (H1031), zincirde de.
	{messages.H1032, "TR_var l: TR_List[TR_Int] = []\nTR_var x = l.TR_first ?? \"a\"\nTR_print(x)", "l.TR_first ?? \"a\"", "l.TR_first ?? 0"},
	{messages.H1032, "TR_var f: TR_Float? = TR_none\nTR_var x = f ?? 0\nTR_print(x)", "f ?? 0", "f ?? 0.0"},
	{messages.H1032, "TR_var b: TR_Bool? = TR_none\nTR_var x = b ?? 1\nTR_print(x)", "b ?? 1", "b ?? TR_false"},
	{messages.H1033, "TR_var j = TR_json_parse(\"{}\")\nTR_var x = TR_Bool(j.TR_get(\"k\") ?? TR_false)\nTR_print(x)", "j.TR_get(\"k\") ?? TR_false", "j.TR_get(\"k\") ?? TR_JSON(TR_false)"},
	{messages.H1031, "TR_print(1 ?? 2)", "1 ?? 2", "1"},
	{messages.H1031, "TR_var l: TR_List[TR_Int] = []\nTR_print(l.TR_first ?? 0 ?? 1)", "l.TR_first ?? 0 ?? 1", "l.TR_first ?? 0"},
	{messages.H1131, "TR_var j = TR_json_parse(`{\"q\": 3}`)\nTR_print(j.TR_get(\"q\") == 3)", "j.TR_get(\"q\") == 3", "j.TR_get(\"q\") == TR_JSON(3)"},
	{messages.H1142, "TR_var l = [1, 2]\nTR_print(l.TR_contains(\"2\"))", "\"2\"", "TR_Int(\"2\")"},
	{messages.H1142, "TR_var adlar = [\"1\", \"2\"]\nTR_var sayılar = [1, 2]\nTR_print(adlar.TR_contains(sayılar.TR_length))", "sayılar.TR_length", "TR_String(sayılar.TR_length)"},
	{messages.H1144, "TR_func f(x: TR_Int) { TR_print(x) }\nTR_var l = [1]\nTR_print(l.TR_map(f))", "TR_print(l.TR_map(f))", "TR_for x TR_in l { f(x) }"},
	{messages.H1145, "TR_var h = (x: TR_Bool) => {\n    TR_if x { TR_return }\n    TR_return 1\n}\nTR_print(h(TR_true))", "TR_return", "TR_return 0"},
	{messages.H1145, "TR_var h = (x: TR_Bool) => {\n    TR_if x { TR_return 1 }\n    TR_print(2)\n}\nTR_print(h(TR_true))", "TR_print(2)", "TR_return 0"},
	{messages.H1145, "TR_var h = (x: TR_Bool) => {\n    TR_if x { TR_return }\n    TR_return 1\n}\nh(TR_true)", "TR_return 1", "TR_return"},
	{messages.H1145, "TR_var h = (x: TR_Bool) => {\n    TR_return TR_none\n}\nTR_print(h(TR_true))", "TR_return TR_none", "TR_return 0"},
	{messages.H1145, "TR_var h = (x: TR_Bool) => {\n    TR_return TR_none\n}\nh(TR_true)", "TR_return TR_none", "TR_return"},
	{messages.H1143, "TR_func f() { TR_print(1) }\nTR_var y = f()\nTR_print(2)", "TR_var y = f()", "f()"},
	{messages.H1139, "TR_var m = [[0] * 3] * 3\nm[0][0] = 1\nTR_print(m)", "[[0] * 3] * 3", "[[0] * 3 TR_for _ TR_in 0..3]"},
	// Başka dillerin mantık sözcükleri, hangi yolda yakalanırsa yakalansın: sözdizimi hatası (H0164) ya
	// da tanımsız ad (H1106). Türkçe yazım aynı ipucunu alır, hiçbir dilin ayrıcalığı yoktur.
	{messages.H0164, "TR_var a = TR_true\nTR_var b = TR_false\nTR_if a and b { TR_print(1) }", "and", "TR_and"},
	{messages.H0164, "TR_var a = TR_false\nTR_if not a { TR_print(1) }", "not", "TR_not"},
	{messages.H0164, "TR_var a = TR_true\nTR_print(a or a)", "or", "TR_or"},
	{messages.H0164, "TR_var a = TR_true\nTR_var b = TR_false\nTR_var d = {\"k\": a or b}\nTR_print(d)", "or", "TR_or"},
	{messages.H0164, "TR_var a = TR_true\nTR_var b = TR_false\nTR_if a ve b { TR_print(1) }", "ve", "TR_and"},
	{messages.H0164, "TR_var a = TR_true\nTR_print(a veya a)", "veya", "TR_or"},
	{messages.H1106, "TR_var a = TR_true\nTR_var b = a and a\nTR_print(b)", "and", "TR_and"},
	{messages.H1106, "TR_var a = TR_false\nTR_var c = not a\nTR_print(c)", "not", "TR_not"},
	{messages.H1106, "TR_var a = TR_false\nTR_var c = değil a\nTR_print(c)", "değil", "TR_not"},
	// Karşılaştırmalar bütün olarak yazımını alır; iki yol da.
	{messages.H0302, "TR_var l = [1]\nTR_var x = 2\nTR_var c = x not in l\nTR_print(c)", "x not in l", "TR_not l.TR_contains(x)"},
	{messages.H0302, "TR_var l = [1]\nTR_var x = 2\nTR_if x not in l { TR_print(1) }", "x not in l", "TR_not l.TR_contains(x)"},
	{messages.H0302, "TR_var x = 2\nTR_var c = x is not 1\nTR_print(c)", "x is not 1", "x != 1"},
	{messages.H0302, "TR_var x = 2\nTR_if x is not 1 { TR_print(1) }", "x is not 1", "x != 1"},
	{messages.H0302, "TR_var l = [1]\nTR_var x = 1\nTR_var c = x in [1]\nTR_print(c)", "x in [1]", "l.TR_contains(x)"},
	{messages.H0302, "TR_var l = [1]\nTR_var x = 1\nTR_if x in l { TR_print(1) }", "x in l", "l.TR_contains(x)"},
}

// suggestionResultCase önerinin sonucunu da okur: hatalı program iletiyi verir, ileti suggestion'ı
// içerir, suggestion'ı içeren düzeltilmiş program (fixed) hatasız koşar ve out'u basar. Öneri
// programın adını değil iletinin genel adını (t, m) kullandığında düzeltme yer değiştirmeyle
// kurulamaz; fixed açıkça yazılır.
type suggestionResultCase struct {
	code                     messages.Code
	wrong, suggestion, fixed string
	out                      string
}

// Cinsli tensörlerin iletileri (T1 dilim 2, H6960–H6979). H6969'un GPU önerisi
// TestTypedPathsOnGPU'dadır (internal/stdlib/ml): bu tablo GPU'suz koşar.
var suggestionResultCases = []suggestionResultCase{
	// 13.3: boş listede ve olmayan indekste öğe çıkmaz; ileti önce boyuna bakmayı önerir.
	{messages.H2533, "TR_var l: TR_List[TR_Int] = []\nTR_print(l.TR_pop())", "l.TR_length",
		"TR_var l: TR_List[TR_Int] = []\nTR_if l.TR_length > 0 {\n    TR_print(l.TR_pop())\n}\nTR_print(l.TR_length)", "0\n"},
	// 15.18: boş tensör listesinin toplamı yoktur; ileti önce boyuna bakmayı önerir.
	{messages.H2720, "TR_var l: TR_List[TR_Tensor] = []\nTR_print(TR_sum(l))", "l.TR_length",
		"TR_var l: TR_List[TR_Tensor] = []\nTR_if l.TR_length > 0 {\n    TR_print(TR_sum(l))\n}\nTR_print(l.TR_length)", "0\n"},
	// 15.5, 3.6a, 7.6: tensör tutan demetin ve yapının '=='si denetim hatasıdır; ileti parça parça yazımı önerir
	// (tensör TR_equal ile, öbür öğe '==' ile). Tensörler eşitken ve değilken.
	{messages.H1148, "TR_var p = (TR_Tensor([1.0]), 1)\nTR_var q = (TR_Tensor([1.0]), 1)\nTR_print(p == q)",
		"TR_equal(p.0, q.0) TR_and p.1 == q.1",
		"TR_var p = (TR_Tensor([1.0]), 1)\nTR_var q = (TR_Tensor([1.0]), 1)\nTR_print(TR_equal(p.0, q.0) TR_and p.1 == q.1)", "true\n"},
	{messages.H1148, "TR_var p = (TR_Tensor([1.0]), 1)\nTR_var q = (TR_Tensor([2.0]), 1)\nTR_print(p != q)",
		"TR_equal(p.0, q.0) TR_and p.1 == q.1",
		"TR_var p = (TR_Tensor([1.0]), 1)\nTR_var q = (TR_Tensor([2.0]), 1)\nTR_print(TR_equal(p.0, q.0) TR_and p.1 == q.1)", "false\n"},
	{messages.H1148, "TR_struct K {\n    t: TR_Tensor\n    n: TR_Int\n}\nTR_var a = K(TR_Tensor([1.0]), 1)\nTR_var b = K(TR_Tensor([1.0]), 1)\nTR_print(a == b)",
		"TR_equal(a.t, b.t) TR_and a.n == b.n",
		"TR_struct K {\n    t: TR_Tensor\n    n: TR_Int\n}\nTR_var a = K(TR_Tensor([1.0]), 1)\nTR_var b = K(TR_Tensor([1.0]), 1)\nTR_print(TR_equal(a.t, b.t) TR_and a.n == b.n)", "true\n"},
	{messages.H1148, "TR_struct K {\n    t: TR_Tensor\n    n: TR_Int\n}\nTR_var a = K(TR_Tensor([1.0]), 1)\nTR_var b = K(TR_Tensor([1.0, 2.0]), 1)\nTR_print(a == b)",
		"TR_equal(a.t, b.t) TR_and a.n == b.n",
		"TR_struct K {\n    t: TR_Tensor\n    n: TR_Int\n}\nTR_var a = K(TR_Tensor([1.0]), 1)\nTR_var b = K(TR_Tensor([1.0, 2.0]), 1)\nTR_print(TR_equal(a.t, b.t) TR_and a.n == b.n)", "false\n"},
	// 18.7, 19.5: açık günlüğün dosyası silinmez, ileti günlüğü kapatmayı önerir; kapatılmış günlük yeniden açılır.
	{messages.H2723, "TR_var g = TR_Logger(\"g.jsonl\")\ng.TR_append(\"a\")\nTR_print(TR_remove(\"g.jsonl\"))", "g.TR_close()",
		"TR_var g = TR_Logger(\"g.jsonl\")\ng.TR_append(\"a\")\ng.TR_close()\nTR_print(TR_remove(\"g.jsonl\"))", "1\n"},
	{messages.H2724, "TR_var g = TR_Logger(\"k/g.jsonl\")\ng.TR_append(\"a\")\nTR_print(TR_remove(\"k\", TR_recursive: TR_true))", "g.TR_close()",
		"TR_var g = TR_Logger(\"k/g.jsonl\")\ng.TR_append(\"a\")\ng.TR_close()\nTR_print(TR_remove(\"k\", TR_recursive: TR_true))", "1\n"},
	{messages.H2725, "TR_var g = TR_Logger(\"g.jsonl\")\ng.TR_close()\ng.TR_append(\"a\")", "TR_Logger(\"g.jsonl\")",
		"TR_var g = TR_Logger(\"g.jsonl\")\ng.TR_close()\ng = TR_Logger(\"g.jsonl\")\ng.TR_append(\"a\")\nTR_print(g.TR_close())", "true\n"},
	{messages.H2534, "TR_var l = [1, 2]\nTR_print(l.TR_remove_at(5))", "l.TR_length",
		"TR_var l = [1, 2]\nTR_if 5 < l.TR_length {\n    TR_print(l.TR_remove_at(5))\n}\nTR_print(l.TR_length)", "2\n"},
	// 13.4: .TR_remove sözlüğün metodudur; listede öğe konumuyla çıkar, dosya silen işlev önerilmez.
	{messages.H2535, "TR_var l = [1, 2]\nTR_var i = 0\nTR_print(l.TR_remove(i))", "l.TR_remove_at(i)",
		"TR_var l = [1, 2]\nTR_var i = 0\nTR_print(l.TR_remove_at(i), l)", "1 [2]\n"},
	// 2.3a: noktadan önceki değer ilk argüman olur; yol değerinde öneri dosyayı taşır.
	{messages.H7016, "TR_var x = \"a.txt\"\nTR_var y = \"b.txt\"\nTR_write_file(x, \"v\")\nx.TR_move(y)", "TR_move(x, y)",
		"TR_var x = \"a.txt\"\nTR_var y = \"b.txt\"\nTR_write_file(x, \"v\")\nTR_move(x, y)\nTR_print(TR_read_file(y))", "v\n"},
	// 7.5, 2.3a: kimlik yapının alanı olamaz; öneri öneksiz addır.
	{messages.H2922, "TR_struct K { a: TR_Int }\nTR_var k = K(1); k.TR_weight = 2", "weight: TR_Int",
		"TR_struct K { a: TR_Int, weight: TR_Int }\nTR_var k = K(1, 0)\nk.weight = 2\nTR_print(k.weight)", "2\n"},
	{messages.H2923, "TR_struct K { weight: TR_Int }\nTR_var k = K(1); k.TR_weight = 2", ".weight",
		"TR_struct K { weight: TR_Int }\nTR_var k = K(1)\nk.weight = 2\nTR_print(k.weight)", "2\n"},
	// 2.3a: metin alıcıda yol ve içerik karışmaz; öneri izlenince verilen yola içerik yazılır.
	{messages.H0301, "TR_var x = \"v\"\nTR_var y = \"a.txt\"\nx." + "TR_write_file(y)", "TR_write_file(y, x)",
		"TR_var x = \"v\"\nTR_var y = \"a.txt\"\nTR_write_file(y, x)\nTR_print(TR_read_file(y))", "v\n"},
	// 19.6: kayıttaki adım tam değerli olmalıdır; ileti yuvarlamayı önerir, kırpmaz.
	{messages.H7013, "TR_var g = TR_Logger(TR_Path(TR_temp_dir(), \"a.jsonl\"))\ng.TR_append({\"step\": 3.5})", "TR_round(...)",
		"TR_var y = TR_Path(TR_temp_dir(), \"a.jsonl\")\nTR_var g = TR_Logger(y)\ng.TR_append({\"step\": TR_round(3.5)})\nTR_print(TR_read_file(y).TR_contains(\"\\\"step\\\":4\"))", "true\n"},
	// 18.11: geçerli UTF-8 olmayan satır hatadır; ileti dosyayı bayt olarak okumayı önerir.
	{messages.H2638, "TR_write_file(\"b.txt\", TR_Bytes([98, 255, 10]))\nTR_print(TR_read_lines(\"b.txt\").TR_next())", "TR_read_bytes(\"b.txt\")",
		"TR_print(TR_read_bytes(\"b.txt\").TR_length)", "3\n"},
	// 13.2: boş ayırıcı ve değiştirilecek boş alt metin hatadır; iletiler karakterleri alan yazımı önerir.
	{messages.H2635, "TR_print(\"ab\".TR_split(\"\"))", "[c TR_for c TR_in m]",
		"TR_var m = \"ab\"\nTR_print([c TR_for c TR_in m])", "[\"a\", \"b\"]\n"},
	{messages.H2636, "TR_print(\"ab\".TR_replace(\"\", \"-\"))", "[c TR_for c TR_in m].TR_join(y)",
		"TR_var m = \"ab\"\nTR_var y = \"-\"\nTR_print([c TR_for c TR_in m].TR_join(y))", "a-b\n"},
	// 13.1: TR_mean, TR_std ve TR_median yalnız liste alır; TR_max tek değerde listeyi ya da ikinci değeri gösterir.
	{messages.H2011, "TR_var a = 1\nTR_var b = 2\nTR_print(TR_mean(a, b))", "TR_mean([a, b])", "TR_var a = 1\nTR_var b = 2\nTR_print(TR_mean([a, b]))", "1.5\n"},
	{messages.H2011, "TR_var a = 1\nTR_print(TR_max(a))", "TR_max([a, b])", "TR_var a = 1\nTR_var b = 2\nTR_print(TR_max([a, b]))", "2\n"},
	{messages.H2011, "TR_var a = 1\nTR_print(TR_max(a))", "TR_max(a, b)", "TR_var a = 1\nTR_var b = 2\nTR_print(TR_max(a, b))", "2\n"},
	// 13.6: başlangıçsız indirgeme boş listede hatadır; ileti başlangıçlı yazımı önerir.
	{messages.H2637, "TR_var l: TR_List[TR_Int] = []\nTR_print(l.TR_reduce((a, x) => a + x))", "l.TR_reduce(0, (a, x) => a + x)",
		"TR_var l: TR_List[TR_Int] = []\nTR_print(l.TR_reduce(0, (a, x) => a + x))", "0\n"},
	// 19.6: metin adım tam sayıya tipin çağrısıyla çevrilir (TR_round metin almaz).
	{messages.H7013, "TR_var g = TR_Logger(TR_Path(TR_temp_dir(), \"a.jsonl\"))\ng.TR_append({\"step\": \"3\"})", "TR_Int(...)",
		"TR_var y = TR_Path(TR_temp_dir(), \"a.jsonl\")\nTR_var g = TR_Logger(y)\ng.TR_append({\"step\": TR_Int(\"3\")})\nTR_print(TR_read_file(y).TR_contains(\"\\\"step\\\":3\"))", "true\n"},
	// 13.8: anahtarı metin olmayan sözlük JSON'a yazılmaz; ileti anahtarları TR_String ile yeni sözlüğe çeviren döngüyü önerir.
	{messages.H7012, "TR_var d = {1: 2, 3: 4}\nTR_print(TR_json_string(d))", "TR_for k, v TR_in d { m[TR_String(k)] = v }",
		"TR_var d = {1: 2, 3: 4}\nTR_var m = {}\nTR_for k, v TR_in d { m[TR_String(k)] = v }\nTR_print(TR_json_string(m))", "{\"1\":2,\"3\":4}\n"},
	// 13.1: TR_write_file'a verilen JSON değeri JSON metni olarak yazılır (TR_String(j) yalnız JSON metnini açar).
	{messages.H2623, "TR_var y = TR_Path(TR_temp_dir(), \"j.json\")\nTR_var j = TR_json_parse(\"{\\\"a\\\": 1}\")\nTR_write_file(y, j)", "TR_json_string(...)",
		"TR_var y = TR_Path(TR_temp_dir(), \"j.json\")\nTR_var j = TR_json_parse(\"{\\\"a\\\": 1}\")\nTR_write_file(y, TR_json_string(j))\nTR_print(TR_read_file(y), TR_json_parse(TR_read_file(y)) == j)", "{\"a\":1} true\n"},
	{messages.H6960, "TR_var t = TR_Tensor([0, 1])\nTR_print(TR_exp(t).TR_tolist)", "TR_Tensor[TR_Float](t)",
		"TR_var t = TR_Tensor([0, 1])\nTR_print(TR_exp(TR_Tensor[TR_Float](t)).TR_tolist)", "[1.0, 2.7182817]\n"},
	{messages.H6960, "TR_var t = TR_Tensor([TR_true, TR_false])\nTR_print(TR_exp(t).TR_tolist)", "TR_where(t, 1.0, 0.0)",
		"TR_var t = TR_Tensor([TR_true, TR_false])\nTR_print(TR_exp(TR_where(t, 1.0, 0.0)).TR_tolist)", "[2.7182817, 1.0]\n"},
	{messages.H6960, "TR_var t = TR_Tensor([0.0, 1.0], TR_dtype: \"float64\")\nTR_print(TR_exp(t).TR_tolist)", "t.TR_to_dtype(\"float32\")",
		"TR_var t = TR_Tensor([0.0, 1.0], TR_dtype: \"float64\")\nTR_print(TR_exp(t.TR_to_dtype(\"float32\")).TR_tolist)", "[1.0, 2.7182817]\n"},
	{messages.H6961, "TR_var t = TR_Tensor([2000000000], TR_dtype: \"int32\")\nTR_print((t + t).TR_tolist)", "t.TR_to_dtype(\"int64\")",
		"TR_var t = TR_Tensor([2000000000], TR_dtype: \"int32\")\nTR_var u = t.TR_to_dtype(\"int64\")\nTR_print((u + u).TR_tolist, (u + u).TR_dtype)", "[4000000000] int64\n"},
	{messages.H6962, "TR_var a = TR_Tensor([4, 6])\nTR_var b = TR_Tensor([2, 0])\nTR_print((a // b).TR_tolist)", "TR_where(b == 0, 1, b)",
		"TR_var a = TR_Tensor([4, 6])\nTR_var b = TR_Tensor([2, 0])\nTR_print((a // TR_where(b == 0, 1, b)).TR_tolist)", "[2, 6]\n"},
	{messages.H6966, "TR_var x = TR_Tensor([1.0, 2.0])\nTR_var m = x > 1.5\nTR_print((m + x).TR_tolist)", "TR_where(m, 1.0, 0.0)",
		"TR_var x = TR_Tensor([1.0, 2.0])\nTR_var m = x > 1.5\nTR_print((TR_where(m, 1.0, 0.0) + x).TR_tolist)", "[1.0, 3.0]\n"},
	{messages.H6966, "TR_var t = TR_Tensor([0, 0, 5])\nTR_var m = TR_Tensor([TR_true, TR_false])\nt[0..2] = m\nTR_print(t.TR_tolist)", "TR_where(m, 1, 0)",
		"TR_var t = TR_Tensor([0, 0, 5])\nTR_var m = TR_Tensor([TR_true, TR_false])\nt[0..2] = TR_where(m, 1, 0)\nTR_print(t.TR_tolist)", "[1, 0, 5]\n"},
	{messages.H6966, "TR_var t = TR_Tensor([3, 4])\nTR_var p = t\nt /= 2\nTR_print(p.TR_tolist)", "//=",
		"TR_var t = TR_Tensor([3, 4])\nTR_var p = t\nt //= 2\nTR_print(p.TR_tolist)", "[1, 2]\n"},
	{messages.H6967, "TR_var m = TR_Tensor([TR_true, TR_false])\nTR_print((m + m).TR_tolist)", "TR_where(m, 1.0, 0.0)",
		"TR_var m = TR_Tensor([TR_true, TR_false])\nTR_print((TR_where(m, 1.0, 0.0) + TR_where(m, 1.0, 0.0)).TR_tolist)", "[2.0, 0.0]\n"},
	{messages.H6967, "TR_var t = TR_Tensor([1.0, -1.0])\nTR_print((~t).TR_tolist)", "t > 0.0",
		"TR_var t = TR_Tensor([1.0, -1.0])\nTR_print((~(t > 0.0)).TR_tolist)", "[false, true]\n"},
	{messages.H6968, "TR_var t = TR_Tensor([1, 2])\nTR_print((t + [1, 2]).TR_tolist)", "TR_Tensor([...])",
		"TR_var t = TR_Tensor([1, 2])\nTR_print((t + TR_Tensor([1, 2])).TR_tolist)", "[2, 4]\n"},
	{messages.H6968, "TR_var t = TR_Tensor([1, 2])\nTR_print((t * 2.5).TR_tolist)", "TR_Tensor[TR_Float](t)",
		"TR_var t = TR_Tensor([1, 2])\nTR_print((TR_Tensor[TR_Float](t) * 2.5).TR_tolist)", "[2.5, 5.0]\n"},
	// 15.23a: çapraz entropinin hedefi (H6980–H6983; H6312'nin iki önerisi).
	{messages.H6980, "TR_var z = TR_zeros([2, 3])\nTR_print(TR_cross_entropy(z, [0, 2]).TR_tolist)", "TR_Tensor([...])",
		"TR_var z = TR_zeros([2, 3])\nTR_print(TR_cross_entropy(z, TR_Tensor([0, 2])).TR_tolist)", "1.0986123\n"},
	{messages.H6981, "TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([0, -1])\nTR_print(TR_cross_entropy(z, h).TR_tolist)", "TR_where(h < 0, -100, h)",
		"TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([0, -1])\nTR_print(TR_cross_entropy(z, TR_where(h < 0, -100, h)).TR_tolist)", "1.0986123\n"},
	{messages.H6982, "TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([0.0, 2.0])\nTR_print(TR_cross_entropy(z, h).TR_tolist)", "TR_Tensor[TR_Int](h)",
		"TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([0.0, 2.0])\nTR_print(TR_cross_entropy(z, TR_Tensor[TR_Int](h)).TR_tolist)", "1.0986123\n"},
	{messages.H6983, "TR_var x = TR_zeros([2, 4])\nTR_var w = TR_zeros([4, 3])\nTR_var h = TR_Tensor([[1.0, 0.0, 0.0], [0.0, 0.5, 0.5]])\nTR_print(TR_linear_cross_entropy(x, w, h).TR_tolist)", "TR_cross_entropy(x @ w, h)",
		"TR_var x = TR_zeros([2, 4])\nTR_var w = TR_zeros([4, 3])\nTR_var h = TR_Tensor([[1.0, 0.0, 0.0], [0.0, 0.5, 0.5]])\nTR_print(TR_cross_entropy(x @ w, h).TR_tolist)", "1.0986123\n"},
	{messages.H6312, "TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([[0, 1, 0], [1, 0, 0]])\nTR_print(TR_cross_entropy(z, h).TR_tolist)", "TR_argmax(h, TR_dim: -1)",
		"TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([[0, 1, 0], [1, 0, 0]])\nTR_print(TR_cross_entropy(z, TR_argmax(h, TR_dim: -1)).TR_tolist)", "1.0986123\n"},
	{messages.H6312, "TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([[0, 1, 0], [1, 0, 0]])\nTR_print(TR_cross_entropy(z, h).TR_tolist)", "TR_Tensor[TR_Float](h)",
		"TR_var z = TR_zeros([2, 3])\nTR_var h = TR_Tensor([[0, 1, 0], [1, 0, 0]])\nTR_print(TR_cross_entropy(z, TR_Tensor[TR_Float](h)).TR_tolist)", "1.0986123\n"},
	// 15.34: gradyansız adım (H6988), ara sonuca (H6990), görünüme ya da tam sayı tensöre (H6991) türev_izle.
	{messages.H6988, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_true)\nTR_var op = TR_SGD([w], TR_lr: 0.5)\nop.TR_step()\nTR_print(w.TR_tolist)", "TR_backward(...)",
		"TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_true)\nTR_var op = TR_SGD([w], TR_lr: 0.5)\nTR_backward(TR_sum(w * w))\nop.TR_step()\nTR_print(w.TR_tolist)", "[0.0]\n"},
	{messages.H6990, "TR_var x = TR_Tensor([1.0], TR_requires_grad: TR_true)\nTR_var y = x * 2\ny.TR_requires_grad = TR_true\nTR_print(y.TR_tolist)", "TR_Tensor(y)",
		"TR_var x = TR_Tensor([1.0], TR_requires_grad: TR_true)\nTR_var y = x * 2\ny = TR_Tensor(y)\ny.TR_requires_grad = TR_true\nTR_print(y.TR_tolist)", "[2.0]\n"},
	{messages.H6991, "TR_var t = TR_zeros([2, 2])\nTR_var v = t.TR_reshape([4])\nv.TR_requires_grad = TR_true\nTR_print(v.TR_shape)", "TR_Tensor(v)",
		"TR_var t = TR_zeros([2, 2])\nTR_var v = t.TR_reshape([4])\nv = TR_Tensor(v)\nv.TR_requires_grad = TR_true\nTR_print(v.TR_shape)", "[4]\n"},
	{messages.H6991, "TR_var v = TR_Tensor([1, 2])\nv.TR_requires_grad = TR_true\nTR_print(v.TR_shape)", "TR_Tensor[TR_Float](v)",
		"TR_var v = TR_Tensor([1, 2])\nv = TR_Tensor[TR_Float](v)\nv.TR_requires_grad = TR_true\nTR_print(v.TR_shape)", "[2]\n"},
	// 15.34: parametre olmayan yaprak iyileştiricide hatadır; önerisi onu türev izler yapmak (H6995).
	{messages.H6995, "TR_var w1 = TR_Tensor([1.0], TR_requires_grad: TR_true)\nTR_var w2 = TR_Tensor([2.0])\nTR_var op = TR_SGD([w1, w2], TR_lr: 0.5)\nTR_backward(TR_sum(w1 * w1 + w2 * w2))\nop.TR_step()\nTR_print(w1.TR_tolist, w2.TR_tolist)", ".TR_requires_grad = TR_true",
		"TR_var w1 = TR_Tensor([1.0], TR_requires_grad: TR_true)\nTR_var w2 = TR_Tensor([2.0])\nw2.TR_requires_grad = TR_true\nTR_var op = TR_SGD([w1, w2], TR_lr: 0.5)\nTR_backward(TR_sum(w1 * w1 + w2 * w2))\nop.TR_step()\nTR_print(w1.TR_tolist, w2.TR_tolist)", "[0.0] [0.0]\n"},
	// 15.24: dikkatin maskesi TR_Bool (H6985'in iki önerisi).
	{messages.H6985, "TR_var q = TR_zeros([2, 2])\nTR_var m = TR_Tensor([[1.0, 0.0], [1.0, 1.0]])\nTR_print(TR_attention(q, q, q, TR_causal: TR_false, TR_mask: m).TR_shape)", "m != 0",
		"TR_var q = TR_zeros([2, 2])\nTR_var m = TR_Tensor([[1.0, 0.0], [1.0, 1.0]])\nTR_print(TR_attention(q, q, q, TR_causal: TR_false, TR_mask: m != 0).TR_shape)", "[2, 2]\n"},
	{messages.H6985, "TR_var q = TR_zeros([2, 2])\nTR_var m = TR_Tensor([[0.0, -TR_inf], [0.0, 0.0]])\nTR_print(TR_attention(q, q, q, TR_causal: TR_false, TR_mask: m).TR_shape)", "m == 0",
		"TR_var q = TR_zeros([2, 2])\nTR_var m = TR_Tensor([[0.0, -TR_inf], [0.0, 0.0]])\nTR_print(TR_attention(q, q, q, TR_causal: TR_false, TR_mask: m == 0).TR_shape)", "[2, 2]\n"},
	{messages.H6970, "TR_print(TR_zeros[\"int\"]([2]).TR_tolist)", "TR_zeros[TR_Int]",
		"TR_print(TR_zeros[TR_Int]([2]).TR_tolist)", "[0, 0]\n"},
	{messages.H6971, "TR_print(TR_Tensor[TR_String]([1]).TR_tolist)", "TR_Tensor[TR_Int](...)",
		"TR_print(TR_Tensor[TR_Int]([1]).TR_tolist)", "[1]\n"},
	{messages.H6972, "TR_print(TR_randn[TR_Int]([2]).TR_dtype)", "TR_randn[TR_Float]",
		"TR_print(TR_randn[TR_Float]([2]).TR_dtype)", "float32\n"},
	{messages.H6976, "TR_var t = TR_Tensor([1, 2], TR_dtype: \"float64\")\nTR_print(t.TR_tolist, t.TR_dtype)", "TR_Tensor[TR_Float](...)",
		"TR_var t = TR_Tensor[TR_Float]([1, 2], TR_dtype: \"float64\")\nTR_print(t.TR_tolist, t.TR_dtype)", "[1.0, 2.0] float64\n"},
	{messages.H6976, "TR_print(TR_zeros([2], TR_dtype: \"bool\").TR_tolist)", "TR_zeros[TR_Bool](...)",
		"TR_print(TR_zeros[TR_Bool]([2], TR_dtype: \"bool\").TR_tolist)", "[false, false]\n"},
	{messages.H6976, "TR_print(TR_randn([2], TR_dtype: \"int64\").TR_dtype)", "TR_dtype: \"float32\"",
		"TR_print(TR_randn([2], TR_dtype: \"float32\").TR_dtype)", "float32\n"},
	{messages.H6977, "TR_var t = TR_Tensor([1.5, -2.5])\nTR_print(t.TR_to_dtype(\"int32\").TR_tolist)", "TR_Tensor[TR_Int](t, TR_dtype: \"int32\")",
		"TR_var t = TR_Tensor([1.5, -2.5])\nTR_var u = TR_Tensor[TR_Int](t, TR_dtype: \"int32\")\nTR_print(u.TR_tolist, u.TR_dtype)", "[1, -2] int32\n"},
	{messages.H6977, "TR_var t = TR_Tensor([1, 2])\nTR_print(t.TR_to_dtype(\"float64\").TR_tolist)", "TR_Tensor[TR_Float](t, TR_dtype: \"float64\")",
		"TR_var t = TR_Tensor([1, 2])\nTR_var u = TR_Tensor[TR_Float](t, TR_dtype: \"float64\")\nTR_print(u.TR_tolist, u.TR_dtype)", "[1.0, 2.0] float64\n"},
	{messages.H6978, "TR_var m = TR_Tensor([TR_true, TR_false])\nTR_print(TR_Tensor[TR_Float](m).TR_tolist)", "TR_where(m, 1.0, 0.0)",
		"TR_var m = TR_Tensor([TR_true, TR_false])\nTR_print(TR_where(m, 1.0, 0.0).TR_tolist)", "[1.0, 0.0]\n"},
	{messages.H6978, "TR_var t = TR_Tensor([1.5, 0.0])\nTR_print(TR_Tensor[TR_Bool](t).TR_tolist)", "t != 0",
		"TR_var t = TR_Tensor([1.5, 0.0])\nTR_print((t != 0).TR_tolist)", "[true, false]\n"},
	{messages.H6979, "TR_var t = TR_Tensor([2, 3])\nTR_print(TR_all(t))", "TR_all(t != 0)",
		"TR_var t = TR_Tensor([2, 3])\nTR_print(TR_all(t != 0), TR_all(TR_Tensor([2, 0]) != 0))", "true false\n"},
}

// maxUntestedSuggestionCodes izin listesindeki kayıt sayısının üst sınırıdır; liste yalnız küçülür.
const maxUntestedSuggestionCodes = 16

// untestedSuggestionCodes definitions.json'da suggestion/example/fixed yuvası taşıyan ama henüz bu tabloda
// sınanmayan kodlardır. Her yeni sınanan kod buradan silinir; liste büyüyemez.
var untestedSuggestionCodes = map[messages.Code]string{
	messages.H0028: "henüz sınanmadı",
	messages.H0143: "henüz sınanmadı",
	messages.H5529: "henüz sınanmadı",
	messages.H6001: "henüz sınanmadı",
	messages.H6002: "henüz sınanmadı",
	messages.H6530: "henüz sınanmadı",
	messages.H6532: "henüz sınanmadı",
	messages.H7203: "henüz sınanmadı",
	messages.H7209: "henüz sınanmadı",
	messages.H7319: "henüz sınanmadı",
	messages.H7435: "henüz sınanmadı",
	messages.H7453: "henüz sınanmadı",
	messages.H8664: "henüz sınanmadı",
	messages.H8768: "henüz sınanmadı",
	messages.H8820: "henüz sınanmadı",
	messages.H8864: "henüz sınanmadı",
}

func TestMessageSuggestionsRunCleanly(t *testing.T) {
	interp.SetLibrary(lib.Files)
	tested := map[messages.Code]bool{}
	for _, tc := range suggestionCases {
		tested[tc.code] = true
		verifySuggestionCase(t, tc)
	}
	t.Run("sonuçlar", func(t *testing.T) {
		t.Chdir(t.TempDir()) // göreli dosya yazan satırlar için
		for _, tc := range suggestionResultCases {
			tested[tc.code] = true
			verifySuggestionResult(t, tc)
		}
	})
	checkSuggestionCoverage(t, tested)
}

// Kural (2.3a, DENETİM istek 112 Y1): sözlükte noktadan sonra yazılan dosya işlevinin önerisi sözlüğü
// argümanda tutar, çağrı hata verir: aynı adlı dosya taşınmaz, kopyalanmaz, üstüne yazılmaz.
func TestAFileFunctionAfterADotLeavesTheFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("a", []byte("veri"), 0o644); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]struct {
		code messages.Code
		call string
	}{
		"TR_move":       {messages.H7016, "`TR_move(x, y)`"},
		"TR_copy":       {messages.H7016, "`TR_copy(x, y)`"},
		"TR_write_file": {messages.H0301, "`TR_write_file(y, x)`"},
	} {
		wrong := "TR_var d = {\"a\": 1}\nd." + f + "(\"a\", \"b\")"
		matched := firstWithCode(collectProgramDiagnostics(wrong), want.code)
		if matched == nil || !strings.Contains(matched.Error(), want.call) {
			t.Fatalf("%s: %s ve %s bekleniyordu: %v", f, want.code, want.call, matched)
		}
		if diags, _ := runProgram("TR_var d = {\"a\": 1}\n" + f + "(d, \"a\")"); len(diags) == 0 {
			t.Errorf("%s: sözlük argümanı hata vermedi", f)
		}
		if data, err := os.ReadFile("a"); err != nil || string(data) != "veri" {
			t.Errorf("%s: dosya değişti: %q, %v", f, data, err)
		}
		if _, err := os.Stat("b"); err == nil {
			t.Errorf("%s: b dosyası oluştu", f)
		}
	}
}

func verifySuggestionCase(t *testing.T, tc suggestionCase) {
	t.Helper()
	diags := collectProgramDiagnostics(tc.wrong)
	var matched error
	for _, d := range diags {
		if messages.Has(d, tc.code) {
			matched = d
			break
		}
	}
	if matched == nil {
		t.Fatalf("%s (%q): beklenen kod üretilmedi; alınan: %v", tc.code, tc.wrong, diags)
	}
	for _, lang := range []string{"tr", "en"} {
		if text := messages.ErrorIn(matched, lang); !strings.Contains(text, tc.suggestion) {
			t.Errorf("%s (%s): ileti %q önerisini içermiyor: %q", tc.code, lang, tc.suggestion, text)
		}
	}
	fixed := strings.Replace(tc.wrong, tc.replace, tc.suggestion, 1)
	if fixed == tc.wrong {
		t.Fatalf("%s: %q içinde %q bulunamadı", tc.code, tc.wrong, tc.replace)
	}
	if after := collectProgramDiagnostics(fixed); len(after) > 0 {
		t.Errorf("%s: öneri uygulandıktan sonra (%q) hata/uyarı kaldı: %v", tc.code, fixed, after)
	}
}

func verifySuggestionResult(t *testing.T, tc suggestionResultCase) {
	t.Helper()
	matched := firstWithCode(collectProgramDiagnostics(tc.wrong), tc.code)
	if matched == nil {
		t.Fatalf("%s (%q): beklenen kod üretilmedi", tc.code, tc.wrong)
	}
	// Öneri iletinin tam bir kod parçasıdır (iki ters tırnağın arası), parçasının bir kesiti değil.
	for _, lang := range []string{"tr", "en"} {
		if text := messages.ErrorIn(matched, lang); !strings.Contains(text, "`"+tc.suggestion+"`") {
			t.Errorf("%s (%s): ileti %q önerisini tam kod parçası olarak içermiyor: %q", tc.code, lang, tc.suggestion, text)
		}
	}
	if !containsFilled(tc.fixed, tc.suggestion) {
		t.Fatalf("%s: düzeltilmiş program önerinin yazımını (%q) içermiyor", tc.code, tc.suggestion)
	}
	diags, out := runProgram(tc.fixed)
	if len(diags) > 0 {
		t.Errorf("%s: önerilen yazım (%q) hata/uyarı verdi: %v", tc.code, tc.fixed, diags)
	} else if out != tc.out {
		t.Errorf("%s: önerilen yazımın (%q) çıktısı %q, beklenen %q", tc.code, tc.fixed, out, tc.out)
	}
}

// containsFilled program önerinin yazımını içeriyor mu; önerideki "..." programın yazdığı
// argümanların yeridir (`TR_zeros[TR_Bool](...)`): öbür parçalar aynı satırda sırayla bulunur.
func containsFilled(program, suggestion string) bool {
	parts := strings.Split(suggestion, "...")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(strings.Join(parts, `[^\n]+`)).MatchString(program)
}

func firstWithCode(diags []error, code messages.Code) error {
	for _, d := range diags {
		if messages.Has(d, code) {
			return d
		}
	}
	return nil
}

func collectProgramDiagnostics(code string) []error {
	diags, _ := runProgram(code)
	return diags
}

// runProgram programı denetler ve çalıştırır; hataları, uyarıları ve çıktıyı verir.
func runProgram(code string) ([]error, string) {
	r := source.Prepare(code, source.Options{Body: true})
	var out []error
	out = append(out, r.Errors...)
	if r.Stage == source.StageSyntax || r.Program == nil {
		return out, ""
	}
	chk := checker.New()
	chk.SetFile(checker.ProgramFile, code) // trlang check gibi: öneri dosyanın satırını verir
	chk.CheckProgram(r.Program)
	out = append(out, chk.Errs()...)
	out = append(out, chk.Warnings()...)
	vm := interp.New()
	var buf bytes.Buffer
	vm.Out, vm.Err = &buf, &buf
	if _, err := vm.Run(r.Program); err != nil {
		out = append(out, err)
	}
	vm.WaitAsync()
	stdlib.Close() // program bitince olduğu gibi: açık okuyucu ve günlük dosyaları kapanır
	return out, buf.String()
}

func checkSuggestionCoverage(t *testing.T, tested map[messages.Code]bool) {
	t.Helper()
	if len(untestedSuggestionCodes) > maxUntestedSuggestionCodes {
		t.Errorf("untestedSuggestionCodes: %d kayıt, en fazla %d; izin listesi yalnızca küçülür",
			len(untestedSuggestionCodes), maxUntestedSuggestionCodes)
	}
	defined := definitionSuggestionCodes(t)
	for _, c := range defined {
		inTable := tested[c]
		reason, inAllow := untestedSuggestionCodes[c]
		switch {
		case !inTable && !inAllow:
			t.Errorf("%s: suggestion/example/fixed yuvalı ileti ne suggestionCases tablosunda ne de untestedSuggestionCodes izin listesinde", c)
		case inTable && inAllow:
			t.Errorf("%s: hem suggestionCases tablosunda hem untestedSuggestionCodes izin listesinde; izin listesinden çıkarın", c)
		case inAllow && strings.TrimSpace(reason) == "":
			t.Errorf("%s: untestedSuggestionCodes kaydının gerekçesi boş", c)
		}
	}
	for c := range untestedSuggestionCodes {
		if !slices.Contains(defined, c) {
			t.Errorf("%s: untestedSuggestionCodes içinde ama definitions.json'da suggestion/example/fixed yuvası yok", c)
		}
	}
}

func definitionSuggestionCodes(t *testing.T) []messages.Code {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("internal", "lang", "messages", "definitions.json"))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := messages.ParseDefinitions(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []messages.Code
	for _, c := range defs.Codes {
		for _, p := range defs.Params[c] {
			lp := strings.ToLower(p)
			if strings.Contains(lp, "suggestion") || strings.Contains(lp, "example") || strings.Contains(lp, "fixed") {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

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
	_ "github.com/sekerahmet/TCB/internal/stdlib"
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
	{messages.H0026, "TR_pritn(1)", "TR_pritn", "TR_print"},
	{messages.H0032, "TR_var x: TR_Float = .5", ".5", "0.5"},
	{messages.H0027, "TR_var x: TR_List[TR_Int] = [1, 2]\nTR_var y: TR_Int = TR_length", "TR_length", "x.TR_length"},
	{messages.H0029, "TR_var x: TR_Int = 1\nTR_var y: TR_Bool = x.TR_true", "x.TR_true", "TR_true"},
	{messages.H0037, "TR_var x: TR_type_Int = 1", "TR_type_Int", "TR_Int"},
	{messages.H0037, "TR_var x: TR_List[TR_Int] = [1, 2]\nTR_var y: TR_Int = x.TR_mem_length", ".TR_mem_length", ".TR_length"},
	{messages.H0037, "TR_kw_var x: TR_Int = 1", "TR_kw_var", "TR_var"},
	{messages.H0037, "TR_var t: TR_Tensor = TR_zeros([1], TR_arg_requires_grad: TR_true)", "TR_arg_requires_grad:", "TR_requires_grad:"},
	{messages.H1007, "TR_var a: ondalik = 1.5", "ondalik", "TR_Float"},
	{messages.H1012, "TR_var öğrenciSayısı: TR_Int = 1", "öğrenciSayısı", "öğrenci_sayısı"},
	{messages.H1013, "TR_const en_büyük = 10", "en_büyük", "EN_BÜYÜK"},
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
	{messages.H6513, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_false)\nTR_var opt = TR_AdamW([w])", "TR_requires_grad: TR_false", "TR_requires_grad: TR_true"},
	{messages.H6514, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_true) * 1.0\nTR_var opt = TR_AdamW([w])", "TR_requires_grad: TR_true) * 1.0", "TR_requires_grad: TR_true)"},
	{messages.H6520, "TR_var m = {\"w\": TR_Tensor([1.0], TR_requires_grad: TR_false)}\nTR_load_parameters(m, {\"w\": TR_Tensor([2.0])})", "TR_requires_grad: TR_false", "TR_requires_grad: TR_true"},
	{messages.H6700, "TR_var w = TR_Tensor([3.0], TR_requires_grad: TR_true) * 1.0\nTR_var g = w.TR_grad", "TR_requires_grad: TR_true) * 1.0", "TR_requires_grad: TR_true)"},
	{messages.H6811, `TR_var t: TR_Tensor = TR_zeros([1], TR_dtype: "float")`, "float", "float32"},
	{messages.H7229, `TR_var ayar = TR_command_line({"adım": 1}, TR_descriptions: {"adim": "x"})`, "adim", "adım"},
	{messages.H7900, "TR_var s = TR_sum(TR_Tensor([1.0, 2.0]), axis: 0)", "axis", "TR_dim"},
	{messages.H7903, "TR_var t = TR_Tensor([1.0]).tür_", "tür_", "TR_dtype"},
	{messages.H1109, "TR_func f(b = 0) -> TR_Int { # ERROR H1109\n    TR_return b\n}\nTR_print(f())", "f(b = 0)", "f(b: TR_Int = 0)"},
	{messages.H1110, "TR_func f(a) -> TR_Int { # ERROR H1110\n    TR_return a\n}\nTR_print(f(1))", "f(a)", "f(a: TR_Int)"},
	{messages.H1112, "TR_func f(a: TR_Int) { # ERROR H1112\n    TR_return a\n}\nTR_print(f(1))", "f(a: TR_Int) {", "f(a: TR_Int) -> TR_Int {"},
	{messages.H1115, "TR_var l = [3, 1, 2]\nl.TR_sort()\nTR_print(l)", "l.TR_sort()", "l = l.TR_sort()"},
	{messages.H2474, "TR_var l = [\"a\", \"b\"]\nTR_print(\" \".TR_join(l))", "\" \".TR_join(l)", "l.TR_join(\" \")"},
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
	{messages.H1131, "TR_var j = TR_json_parse(`{\"k\": 3}`)\nTR_print(j[\"k\"] == 3)", "j[\"k\"] == 3", "TR_Int(j[\"k\"]) == 3"},
	{messages.H1131, "TR_var j = TR_json_parse(`{\"k\": 3}`)\nTR_print(j[\"k\"] == 3)", "j[\"k\"] == 3", "j[\"k\"] == TR_JSON(3)"},
	{messages.H1131, "TR_var j = TR_json_parse(`[1, 2]`)\nTR_print(TR_List(j) == [1, 2])", "TR_List(j) == [1, 2]", "TR_List(j) == [TR_JSON(1), TR_JSON(2)]"},
	{messages.H1131, "TR_var j = TR_json_parse(`[1, 2]`)\nTR_print(TR_List(j).TR_contains(1))", "TR_List(j).TR_contains(1)", "TR_List(j).TR_contains(TR_JSON(1))"},
	{messages.H1131, "TR_var j = TR_json_parse(`7`)\nTR_print(TR_match j {\n    2 => \"iki\"\n    _ => \"başka\"\n})", "2", "TR_JSON(2)"},
	{messages.H1132, "TR_var j = TR_json_parse(`{\"n\": null}`)\nTR_print(j[\"n\"] == TR_none)", "j[\"n\"] == TR_none", "TR_Type(j[\"n\"]) == TR_NoneType"},
	{messages.H1132, "TR_var j = TR_json_parse(`{\"n\": null}`)\nTR_print(j[\"n\"] == TR_none)", "j[\"n\"] == TR_none", "j.TR_get(\"n\") == TR_none"},
	{messages.H1132, "TR_var j = TR_json_parse(`[1, null]`)\nTR_print(TR_List(j).TR_contains(TR_none))", "TR_List(j).TR_contains(TR_none)", "TR_List(j).TR_contains(TR_JSON(TR_none))"},
	{messages.H1134, "TR_var j = TR_json_parse(`{}`)\nTR_print(TR_Type(j) == TR_JSON)", "TR_Type(j) == TR_JSON", "TR_Type(j) == TR_Dict"},
	{messages.H1135, "TR_var l: TR_List[TR_Int] = []\nTR_var m = [1]\nm[0] = l.TR_first\nTR_print(m)", "l.TR_first", "l.TR_first ?? 0"},
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
	{messages.H6966, "TR_var t = TR_Tensor([3, 4])\nt /= 2\nTR_print(t.TR_tolist)", "//=",
		"TR_var t = TR_Tensor([3, 4])\nt //= 2\nTR_print(t.TR_tolist)", "[1, 2]\n"},
	{messages.H6967, "TR_var m = TR_Tensor([TR_true, TR_false])\nTR_print((m + m).TR_tolist)", "TR_where(m, 1.0, 0.0)",
		"TR_var m = TR_Tensor([TR_true, TR_false])\nTR_print((TR_where(m, 1.0, 0.0) + TR_where(m, 1.0, 0.0)).TR_tolist)", "[2.0, 0.0]\n"},
	{messages.H6967, "TR_var t = TR_Tensor([1.0, -1.0])\nTR_print((~t).TR_tolist)", "t > 0.0",
		"TR_var t = TR_Tensor([1.0, -1.0])\nTR_print((~(t > 0.0)).TR_tolist)", "[false, true]\n"},
	{messages.H6968, "TR_var t = TR_Tensor([1, 2])\nTR_print((t + [1, 2]).TR_tolist)", "TR_Tensor([...])",
		"TR_var t = TR_Tensor([1, 2])\nTR_print((t + TR_Tensor([1, 2])).TR_tolist)", "[2, 4]\n"},
	{messages.H6968, "TR_var t = TR_Tensor([1, 2])\nTR_print((t * 2.5).TR_tolist)", "TR_Tensor[TR_Float](t)",
		"TR_var t = TR_Tensor([1, 2])\nTR_print((TR_Tensor[TR_Float](t) * 2.5).TR_tolist)", "[2.5, 5.0]\n"},
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
	for _, tc := range suggestionResultCases {
		tested[tc.code] = true
		verifySuggestionResult(t, tc)
	}
	checkSuggestionCoverage(t, tested)
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
	chk.SetFile(checker.ProgramFile, code) // tr check gibi: öneri dosyanın satırını verir
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

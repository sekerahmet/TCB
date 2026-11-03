package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (3.6, 3.7; H1136): öğe tipi bilinen kabın indeksine başka tipte değer yazmak denetim hatasıdır;
// bileşik atamada atanan değer işlemin sonucudur. Listede öğe, sözlükte değer. Tam sayı ondalık
// öğeye yazılır (3.13); tipi bilinmeyen değer, öğe tipsiz kap, dilim ve isteğe bağlı öğeli kap geçer.
func TestElementAssignType(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
	}{
		{"liste", "TR_var m = [1, 2]\nm[0] = \"s\"", 2},
		{"bileşik", "TR_var m = [1, 2]\nm[0] += 1.5", 2},
		{"iç içe", "TR_var m = [[1], [2]]\nm[0][0] = \"s\"", 2},
		{"sözlük değeri", "TR_var d = {\"a\": 1}\nd[\"b\"] = \"s\"", 2},
		{"tipli liste", "TR_var m: TR_List[TR_String] = []\nm[0] = 1", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, messages.H1136, tc.line) })
	}
	noErrors(t, "TR_var m = [1.0, 2.0]\nm[0] = 3\nm[1] += 1\nTR_var n = [[1], []]\nn[1] = [2]")
	noErrors(t, "TR_var m = [1, 2]\nm[0..1] = [3]\nTR_var k: TR_List[TR_Int?] = [1]\nk[0] = TR_none")
	noErrors(t, "TR_var u = [[1], \"a\"][0]\nu[0] = \"s\"\nTR_var d = {\"a\": [1]}\nd[\"b\"] = [2]") // u: tipi bilinmeyen değer
}

// Kural (3.6): kabın tip argümanında isteğe bağlı öğe isteğe bağlı olmayan yere verilmez:
// [1, TR_none] bir TR_List[TR_Int?]'dir, TR_List[TR_Int] beklenen yerde H1001.
func TestOptionalElementListIsNotARequiredList(t *testing.T) {
	singleError(t, "TR_var k: TR_List[TR_Int] = [1, TR_none]", messages.H1001, 1)
	singleError(t, "TR_var k: TR_Dict[TR_String, TR_Int] = {\"a\": 1, \"b\": TR_none}", messages.H1001, 1)
	noErrors(t, "TR_var k: TR_List[TR_Int?] = [1, TR_none]\nTR_var m: TR_List[TR_Int?] = [1, 2]")
}

// Kural (4.10, 3.6; H1138): '+' aynı tipten iki listeyi uç uca ekler; öğe tipleri ayrı iki liste
// (TR_List[TR_Int] ile TR_List[TR_Float] de) denetim hatasıdır, '+='da da. Öğe tipsiz ya da tipi
// bilinmeyen liste geçer.
func TestListPlusElementTypes(t *testing.T) {
	singleError(t, "TR_var e = [1] + [\"a\"]", messages.H1138, 1)
	singleError(t, "TR_var e = [1] + [2.5]", messages.H1138, 1)
	singleError(t, "TR_var m = [1, 2]\nm += [\"a\"]", messages.H1138, 2)
	noErrors(t, "TR_var a = [1] + [2]\nTR_var b = [1] + []\nTR_var m = [1.0]\nm += [2.0]\nTR_var u = [[1], \"a\"][0]\nTR_var v = u + [1]")
}

// Kural (4.10, H1139): öğeleri başvuruyla paylaşılan (liste, sözlük, tensör, yapı ya da onları tutan
// demet) liste '*' ile yinelenemez, '*='de de; ileti tek öğeli değişmezde her satırı ayrı kuran
// üreteci önerir. Metin, sayı ve demet öğeli liste ve tipi bilinmeyen öğe geçer.
func TestSharedRowsAreNotRepeated(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
	}{
		{"liste", "TR_var m = [[0] * 3] * 3", 1},
		{"solda sayı", "TR_var m = 3 * [{\"a\": 1}]", 1},
		{"tensör", "TR_var m = [TR_Tensor([1.0])] * 2", 1},
		{"yapı", "TR_struct P { a: TR_Int }\nTR_var m = [P(1)] * 2", 2},
		{"demette liste", "TR_var m = [(1, [2])] * 2", 1},
		{"bileşik", "TR_var l = [[1]]\nl *= 2", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, messages.H1139, tc.line) })
	}
	src := "TR_var n = 2\nTR_var m = [[0] * 3] * (n + 1)"
	c := New()
	c.SetFile(ProgramFile, src) // öneri ifadelerin dosyadaki yazımıyla kurulur
	c.CheckProgram(parser.New(lexer.NewBody(src, true)).ParseProgram())
	if errs := c.Errs(); len(errs) != 1 || !strings.Contains(messages.ErrorIn(errs[0], "en"), "`[[0] * 3 TR_for _ TR_in 0..(n + 1)]`") {
		t.Errorf("üreteç önerisi bekleniyordu: %v", errs)
	}
	noErrors(t, "TR_var a = [0] * 3\nTR_var s = [\"x\"] * 2\nTR_var t = [(1, \"a\")] * 2\nTR_var u = [[1], \"a\"][0]\nTR_var v = [u] * 2")
}

// Kural (15.5): tensörün karşılaştırması öğe öğedir ve bir tensör verir; mantık değeri değildir.
func TestTensorComparisonIsATensor(t *testing.T) {
	noErrors(t, "TR_var t = TR_Tensor([1.0, 2.0])\nTR_var d: TR_Dict[TR_String, TR_Tensor] = {}\nd[\"m\"] = t <= 1.0")
	singleError(t, "TR_var t = TR_Tensor([1.0, 2.0])\nTR_var b: TR_Bool = t > 1.0", messages.H1001, 2)
}

// Kural (3.6, 13.10: TR_List[T].TR_contains(x: T)): öğe tipi bilinen listenin .TR_contains'ine öğe
// tipinden ayrı tipte, tipi bilinen değer denetim hatasıdır, H1142 (motor hatasız TR_false verir).
// Metin sayıya, sayı metne çevrilebiliyorsa ileti tip çağrısını önerir. Sayılar ayrı değildir
// ([1, 2].TR_contains(1.0) doğru); TR_none, tipi bilinmeyen değer, metin ve sözlük alıcı geçer.
func TestListContainsElementType(t *testing.T) {
	cases := []struct {
		name, src, want string
		line            int
	}{
		{"metin", "TR_var l = [1, 2]\nTR_print(l.TR_contains(\"a\"))", "type TR_String", 2},
		{"çevrilebilen metin", "TR_var l = [1, 2]\nTR_print(l.TR_contains(\"2\"))", "`TR_Int(\"2\")`", 2},
		{"sayı metin listesinde", "TR_var l = [\"a\"]\nTR_var n = 1\nTR_print(l.TR_contains(n))", "`TR_String(n)`", 3},
		{"liste", "TR_var l = [[1]]\nTR_print(l.TR_contains(\"a\"))", "TR_List[TR_List[TR_Int]]", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { fileErrorWith(t, tc.src, messages.H1142, tc.line, tc.want) })
	}
	noErrors(t, "TR_print([1, 2].TR_contains(1.0), [1.5].TR_contains(1), [\"a\"].TR_contains(\"b\"), \"abc\".TR_contains(\"a\"))")
	noErrors(t, "TR_var l: TR_List[TR_Int?] = []\nTR_var k: TR_List[TR_Int] = []\nTR_print(l.TR_contains(TR_none), k.TR_contains(TR_none), k.TR_contains("+unknownValue+"), [1].TR_contains([\"a\", 1][0]))")
}

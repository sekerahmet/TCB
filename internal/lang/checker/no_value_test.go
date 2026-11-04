package checker

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// noValueLines programın değer vermeyen çağrılarının satırlarıdır.
func noValueLines(t *testing.T, source string) []int {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.CheckProgram(prog)
	lines := []int{}
	for e := range c.noValue {
		lines = append(lines, e.Line())
	}
	slices.Sort(lines)
	return lines
}

const noValueFn = "TR_func f() { }\n"

// Kural (3.5, 6.1): TR_func'ın çağrısı gövdesinde değerli TR_return yoksa değer vermez; `->`
// ölçüt değildir, iç içe fonksiyonun TR_return'ü sayılmaz, ad bloğunun her yerinden görünür.
func TestAFunctionWithoutAValueReturnGivesNoValue(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         []int
	}{
		{"boş gövde", "TR_func f() { }\nf()", []int{2}},
		{"değersiz dönüş", "TR_func f() { TR_return }\nf()", []int{2}},
		{"`->` yazılmış, TR_return yok", "TR_func f() -> TR_Int { TR_throw \"x\" }\nf()", []int{}},
		{"iç içe fonksiyon sayılmaz", "TR_func f() {\n  TR_var g = (x: TR_Int) => { TR_return x }\n}\nf()", []int{4}},
		{"ileri başvuru", "f()\nTR_func f() { }", []int{1}},
		{"içteki ad dıştakini gizler", noValueFn + "TR_func g(f: () -> TR_Int) -> TR_Int {\n  TR_return f()\n}", []int{}},
		{"bloktaki fonksiyon", "TR_if TR_true {\n  f()\n  TR_func f() { }\n}", []int{2}},
		{"metot", "TR_struct P {\n  TR_func m() { }\n  TR_func n() -> TR_Int { TR_return 1 }\n}\nTR_var p = P()\np.m()\np.n()", []int{6}},
		{"fonksiyon tipli alan", "TR_struct P {\n  m: () -> TR_NoneType\n}\nTR_func q(p: P) { p.m() }", []int{4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := noValueLines(t, tc.source); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("satırlar %v, beklenen %v\nkaynak:\n%s", got, tc.want, tc.source)
			}
		})
	}
}

// Kural (6.6, 6.7): fonksiyon ifadesinin ve sondaki bloğun gövdesi değer vermiyorsa fonksiyon değer
// döndürmez: değersiz çağrı, TR_else'i olmayan TR_if, dallarından biri değer vermeyen TR_if, TR_match ve
// TR_try. TR_throw ya da değerli TR_return ile biten dal sayılmaz. Yeniden atanan değişkenin fonksiyonu bilinmez.
func TestABodyWithoutAValueGivesNoValue(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         []int
	}{
		{"ifade gövdesi", noValueFn + "TR_var g = (x: TR_Int) => f()\ng(1)", []int{2, 3}},
		{"ifade gövdesi değerli", "TR_var g = (x: TR_Int) => x\ng(1)", []int{}},
		{"blok gövdesi", "TR_var g = (x: TR_Int) => { x + 1 }\ng(1)", []int{2}},
		{"blok gövdesi TR_return ile", "TR_var g = (x: TR_Int) => { TR_return x }\ng(1)", []int{}},
		{"hemen çağrılan", noValueFn + "((x: TR_Int) => f())(1)", []int{2, 2}},
		{"dalın biri değersiz", noValueFn + "TR_var g = (x: TR_Bool) => TR_if x { f() } TR_else { 1 }\ng(TR_true)", []int{2, 3}},
		{"iki dal da değerli", "TR_var g = (x: TR_Bool) => TR_if x { 2 } TR_else { 1 }\ng(TR_true)", []int{}},
		{"TR_throw ile biten dal", "TR_var g = (x: TR_Bool) => TR_if x { TR_throw \"h\" } TR_else { 1 }\ng(TR_true)", []int{}},
		{"değersiz TR_return ile biten dal", "TR_var g = (c: TR_Bool) => TR_if c { TR_return } TR_else { 2 }\ng(TR_true)", []int{2}},
		{"değerli TR_return ile biten dal", "TR_var g = (c: TR_Bool) => TR_if c { TR_return 1 } TR_else { 2 }\ng(TR_true)", []int{}},
		{"bütün dalları TR_throw olan iç TR_if", "TR_var g = (c: TR_Bool) => TR_if c {\n  TR_if c { TR_throw \"a\" } TR_else { TR_throw \"b\" }\n} TR_else { 2 }\ng(TR_true)", []int{}},
		{"TR_match'in kolu", noValueFn + "TR_var g = (x: TR_Int) => TR_match x {\n  1 => f()\n  _ => 2\n}\ng(1)", []int{3, 6}},
		{"yeniden atanan", noValueFn + "TR_var g = (x: TR_Int) => f()\ng = (x: TR_Int) => x\ng(1)", []int{2}},
		{"sonra yeniden atanan", noValueFn + "TR_var g = () => f()\nTR_func h() { g() }\ng = () => 1", []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := noValueLines(t, tc.source); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("satırlar %v, beklenen %v\nkaynak:\n%s", got, tc.want, tc.source)
			}
		})
	}
}

// Kural (6.9): tipi `(T) -> TR_NoneType` yazılmış değerin çağrısı değer vermez.
func TestAValueTypedWithoutAResultGivesNoValue(t *testing.T) {
	for source, want := range map[string][]int{
		"TR_func g(h: (TR_Int) -> TR_NoneType) { h(1) }":                {1},
		"TR_func g(h: (TR_Int) -> TR_Int) -> TR_Int { TR_return h(1) }": {},
		"TR_func g(h: (TR_Int) -> TR_NoneType?) { h(1) }":               {},
		"TR_var h: () -> TR_NoneType = () => TR_exit(1)\nh()":           {1, 2},
	} {
		if got := noValueLines(t, source); !reflect.DeepEqual(got, want) {
			t.Errorf("satırlar %v, beklenen %v\nkaynak:\n%s", got, want, source)
		}
	}
}

// Kural (13.10): yerleşik ancak tanım dosyasında dönüşü TR_NoneType ise değer vermez; `returns`
// yazılmamış tanım değer döndürmez demek değildir. Alıcısı bilinmeyen metot ve fonksiyon değeri
// tutan değişken bilinmez (kitaplığın yapısı aynı adla değer döndürebilir: TR_cache.TR_append).
func TestABuiltinGivesNoValueOnlyWhenItsDefinitionSaysSo(t *testing.T) {
	for source, want := range map[string][]int{
		"TR_exit(1)":  {1},
		"TR_sum([1])": {},
		"TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var o = TR_AdamW([w]); o.TR_zero_grad()": {1},
		"TR_var o = " + unknownValue + "; o.TR_zero_grad()":                                                  {},
		"TR_var f = TR_exit\nf(1)": {},
	} {
		if got := noValueLines(t, source); !reflect.DeepEqual(got, want) {
			t.Errorf("satırlar %v, beklenen %v\nkaynak:\n%s", got, want, source)
		}
	}
}

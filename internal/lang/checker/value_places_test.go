package checker

import (
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func hasCode(errs []error, code messages.Code) bool {
	return slices.ContainsFunc(errs, func(e error) bool { return messages.Has(e, code) })
}

// Kural (3.5): muaflar dışında her ifade yeri değer yeridir: değersiz çağrı orada H1143'tür. İleti
// çağrıyı adıyla söyler ve deyim yapmayı önerir.
func TestNoValueCallInAValuePlace(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"ilk değer", "TR_var y = f()"},
		{"atama", "TR_var y = 1\ny = f()"},
		{"argüman", "TR_print(f())"},
		{"adlı argüman", "TR_var t = TR_Tensor([1.0], TR_requires_grad: f())"},
		{"liste öğesi", "TR_var l = [f()]"},
		{"demet öğesi", "TR_var d = (1, f())"},
		{"sözlük değeri", "TR_var d = {\"a\": f()}"},
		{"sözlük anahtarı", "TR_var d = {f(): 1}"},
		{"yerleştirme", "TR_var m = \"\\(f())\""},
		{"işlenen", "TR_var y = f() + 1"},
		{"önek işlenen", "TR_var y = -f()"},
		{"koşul", "TR_if f() { TR_print(1) }"},
		{"TR_if'in dalı", "TR_var y = TR_if TR_true { f() } TR_else { 2 }"},
		{"TR_match'in konusu", "TR_match f() {\n  _ => 1\n}"},
		{"TR_match'in kolu", "TR_var y = TR_match 1 {\n  1 => f()\n  _ => 2\n}"},
		{"TR_try'ın dalı", "TR_var y = TR_try { f() } TR_catch e { 2 }"},
		{"parametre varsayılanı", "TR_func g(a: TR_Int = f()) { }"},
		{"alan varsayılanı", "TR_struct P {\n  x: TR_Int = f()\n}"},
		{"TR_for'un kaynağı", "TR_for x TR_in f() { TR_print(x) }"},
		{"indeks", "TR_var l = [1]\nTR_print(l[f()])"},
		{"aralık ucu", "TR_for i TR_in 0..f() { TR_print(i) }"},
		{"üye alıcısı", "TR_print(f().TR_length)"},
		{"TR_throw", "TR_throw f()"},
		{"değer döndüren fonksiyonun TR_return'ü", "TR_func g() -> TR_Int { TR_return f() }"},
		{"blok gövdesinin TR_return'ü", "TR_var g = () => {\n  TR_return f()\n}"},
		{"liste üreteci", "TR_var l = [f() TR_for x TR_in 0..2]"},
		{"TR_exit dal olarak", "TR_var y = TR_if TR_true { TR_exit(1) } TR_else { 2 }"},
		{"sondaki bloğun içi", "TR_var k = TR_error_code { TR_var y = f() }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := checkErrs(t, noValueFn+tc.source)
			if !hasCode(errs, messages.H1143) {
				t.Errorf("H1143 beklenirdi, gelen %v\nkaynak:\n%s", errs, tc.source)
			}
		})
	}
	singleError(t, noValueFn+"TR_var y = f()", messages.H1143, 2, "`f`", "deyim")
}

// Kural (3.5): deyim, TR_async f(), `=>`'nin ifade gövdesi, sondaki bloğun son ifadesi
// ve değeri kullanılmayan dalın son ifadesi değer yeri değildir; TR_throw ile biten dal sayılmaz.
func TestPlacesWithoutAValue(t *testing.T) {
	for _, source := range []string{
		"f()",
		"TR_async f()",
		"TR_var g = TR_async f()",
		"TR_func h() {\n  TR_defer { f() }\n}",
		"TR_var g = (x: TR_Int) => f()",
		"TR_var k = TR_error_code { f() }",
		"TR_if TR_true { f() } TR_else { f() }",
		"TR_match 1 {\n  1 => f()\n  _ => f()\n}",
		"TR_no_grad { TR_if TR_true { f() } TR_else { f() } }",
		"TR_no_grad {\n  TR_match 1 {\n    1 => f()\n    _ => f()\n  }\n}",
		"TR_var k = TR_error_code {\n  TR_try { f() } TR_catch e { f() }\n}",
		"TR_var g = (x: TR_Bool) => TR_if x { f() } TR_else { 1 }",
		"TR_var y = TR_if TR_true { TR_throw \"h\" } TR_else { 2 }",
		"TR_var y = TR_no_grad { TR_if TR_true { TR_throw \"h\" } TR_else { 2 } }",
	} {
		noErrors(t, noValueFn+source)
	}
}

// Kural (3.5): gövde değer vermiyorsa fonksiyonun çağrısı değer yerinde hatadır: dallardan biri değer
// vermeyen ifade gövdesi, değersiz TR_return ile biten dal.
func TestAFunctionWithoutAValueBodyInAValuePlace(t *testing.T) {
	for _, source := range []string{
		"TR_var g = (x: TR_Bool) => TR_if x { f() } TR_else { 1 }\nTR_var y = g(TR_true)",
		"TR_var g = (c: TR_Bool) => TR_if c { TR_return } TR_else { 2 }\nTR_var y = g(TR_true)",
	} {
		_, errs := checkErrs(t, noValueFn+source)
		if !hasCode(errs, messages.H1143) {
			t.Errorf("H1143 beklenirdi, gelen %v\nkaynak:\n%s", errs, source)
		}
	}
	noErrors(t, "TR_var g = (c: TR_Bool) => TR_if c {\n  TR_if c { TR_throw \"a\" } TR_else { TR_throw \"b\" }\n} TR_else { 2 }\nTR_var y = g(TR_true)")
}

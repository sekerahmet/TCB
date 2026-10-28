package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Denetim her ifadenin ve deyimin içine iner (undefined_names.go'daki gezintiyle aynı
// düğümler): eskiden liste, sözlük, aralık, metin içi ifade, liste üreteci, indeks, üye,
// atama hedefi, yapı alan varsayılanı ve metot gövdesi, eşle deseni ve parametre
// varsayılanı içindeki hatalar sessizce geçerdi.
func TestCheckerDescendsIntoSubexpressions(t *testing.T) {
	const kare = "TR_func kare(x: TR_Int) -> TR_Int { TR_return x * x }\n"
	const want = `1. parametresi "TR_Int" bekliyor ancak "TR_String" verildi`
	cases := []struct{ name, source string }{
		{"liste", `TR_var l = [1, kare("a")]`},
		{"sözlük değeri", `TR_var s = {"k": kare("a")}`},
		{"sözlük anahtarı", `TR_var s = {kare("a"): 1}`},
		{"aralık", `TR_var a = kare("a")..3`},
		{"metin içi ifade", `TR_var m = "x: \(kare("a"))"`},
		{"liste üreteci çıktısı", `TR_var l = [kare("a") TR_for x TR_in 1..3]`},
		{"liste üreteci kaynağı", `TR_var l = [x TR_for x TR_in [kare("a")]]`},
		{"liste üreteci koşulu", `TR_var l = [x TR_for x TR_in 1..3 TR_if kare("a") > 1]`},
		{"indeks", "TR_var l = [1]\n" + `TR_var i = l[kare("a")]`},
		{"indekslenen", `TR_var i = [kare("a")][0]`},
		{"üye", `TR_var u = kare("a").x`},
		{"indeks atama hedefi", "TR_var l = [1]\n" + `l[kare("a")] = 1`},
		{"alan atama hedefi", `[kare("a")].x = 1`},
		{"çoklu atama hedefi", "TR_var l = [1]\nTR_var y = 1\n" + `l[kare("a")], y = 1, 2`},
		{"yapı alan varsayılanı", "TR_struct A {\n  alan = kare(\"a\")\n}"},
		{"yapı metot gövdesi", "TR_struct A {\n  TR_func m() { kare(\"a\") }\n}"},
		{"eşle deseni", `TR_var e = TR_match 1 { kare("a") => 1 }`},
		{"parametre varsayılanı", `TR_func g(p = kare("a")) { }`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			singleError(t, kare+c.source, messages.H1004, 0, want)
		})
	}
	// Metodun dönüş tipi kendi gövdesinde denetlenir.
	singleError(t, "TR_struct A {\n  TR_func m() -> TR_Int { TR_return \"a\" }\n}", messages.H1003, 2)
	// Sabite iç içe bir fonksiyonda atama da yakalanır.
	singleError(t, "TR_const k = 1\nTR_var l = [() => { k = 2 }]", messages.H1002, 2)
	// Metodun adı yapının dışındaki kapsama tanımlanmaz: dıştaki 'm'nin imzası kalır.
	singleError(t, "TR_func m(x: TR_Int) { }\nTR_struct A {\n  TR_func m() { }\n}\nm(\"a\")", messages.H1004, 0, want)
	// Üreteç değişkeni kendi kapsamındadır: dıştaki aynı adlı açıklamalı değişkeni gizler.
	noErrors(t, "TR_func f(x: TR_Int) { }\nTR_var x: TR_String = \"a\"\nTR_var l = [f(x) TR_for x TR_in 1..3]")
}

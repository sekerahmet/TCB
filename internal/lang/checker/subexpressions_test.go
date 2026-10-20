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
	const kare = "fonksiyon kare(x: Tam): Tam { sonuç x * x }\n"
	const want = `1. parametresi "Tam" bekliyor ancak "Metin" verildi`
	cases := []struct{ name, source string }{
		{"liste", `değişken l = [1, kare("a")]`},
		{"sözlük değeri", `değişken s = {"k": kare("a")}`},
		{"sözlük anahtarı", `değişken s = {kare("a"): 1}`},
		{"aralık", `değişken a = kare("a")..3`},
		{"metin içi ifade", `değişken m = "x: {kare("a")}"`},
		{"liste üreteci çıktısı", `değişken l = [kare("a") döngü x içinde 1..3]`},
		{"liste üreteci kaynağı", `değişken l = [x döngü x içinde [kare("a")]]`},
		{"liste üreteci koşulu", `değişken l = [x döngü x içinde 1..3 eğer kare("a") > 1]`},
		{"indeks", "değişken l = [1]\n" + `değişken i = l[kare("a")]`},
		{"indekslenen", `değişken i = [kare("a")][0]`},
		{"üye", `değişken u = kare("a").x`},
		{"indeks atama hedefi", "değişken l = [1]\n" + `l[kare("a")] = 1`},
		{"alan atama hedefi", `[kare("a")].x = 1`},
		{"çoklu atama hedefi", "değişken l = [1]\ndeğişken y = 1\n" + `l[kare("a")], y = 1, 2`},
		{"yapı alan varsayılanı", "yapı A {\n  alan = kare(\"a\")\n}"},
		{"yapı metot gövdesi", "yapı A {\n  fonksiyon m() { kare(\"a\") }\n}"},
		{"eşle deseni", `değişken e = eşle 1 { kare("a") => 1 }`},
		{"parametre varsayılanı", `fonksiyon g(p = kare("a")) { }`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			singleError(t, kare+c.source, messages.H1004, 0, want)
		})
	}
	// Metodun dönüş tipi kendi gövdesinde denetlenir.
	singleError(t, "yapı A {\n  fonksiyon m(): Tam { sonuç \"a\" }\n}", messages.H1003, 2)
	// Sabite iç içe bir fonksiyonda atama da yakalanır.
	singleError(t, "sabit k = 1\ndeğişken l = [fonksiyon() { k = 2 }]", messages.H1002, 2)
	// Metodun adı yapının dışındaki kapsama tanımlanmaz: dıştaki 'm'nin imzası kalır.
	singleError(t, "fonksiyon m(x: Tam) { }\nyapı A {\n  fonksiyon m() { }\n}\nm(\"a\")", messages.H1004, 0, want)
	// Üreteç değişkeni kendi kapsamındadır: dıştaki aynı adlı açıklamalı değişkeni gizler.
	noErrors(t, "fonksiyon f(x: Tam) { }\ndeğişken x: Metin = \"a\"\ndeğişken l = [f(x) döngü x içinde 1..3]")
}

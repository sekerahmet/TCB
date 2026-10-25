package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// ---------------------------------------------------------------------------
// Tip adları ve uyumluluk kuralları
// ---------------------------------------------------------------------------

// normalizeTypeName kabul edilmeyen harf yazımını (tam) da asıl tipe çevirir: hatayı
// tipAdiDenetle verir, aynı satırda ayrıca tip uyuşmazlığı çıkmaz.
func TestTypeNameNormalization(t *testing.T) {
	tests := map[string]string{
		"Tam": "Tam", "tam": "Tam", "TAM": "Tam",
		"Ondalık": "Ondalık", "ONDALIK": "Ondalık",
		"Metin": "Metin", "METİN": "Metin",
		"Mantık": "Mantık", "MANTIK": "Mantık",
		"Liste": "Liste", "Sözlük": "Sözlük", "Tensör": "Tensör", "Yok": "Yok",
		"": "Herhangi",
		// Bilinmeyen adlar (kullanıcı yapıları) olduğu gibi kalır
		"Nokta": "Nokta", "Fonksiyon": "Fonksiyon",
	}
	for input, want := range tests {
		if got := normalizeTypeName(input); got != want {
			t.Errorf("normalizeTypeName(%q) = %q, beklenen %q", input, got, want)
		}
	}
}

// Tip adlarının yalnızca Türkçe yazımı geçerlidir; ASCII yazım hata verir ve asıl adı
// önerir (tek hata: aynı satırda ayrıca tip uyuşmazlığı bildirilmez).
func TestTypeNameASCIISuggestion(t *testing.T) {
	singleError(t, `değişken a: ondalik = 1.5`, messages.H1007, 1, "'ondalik'", "'Ondalık'")
	singleError(t, `değişken a: Mantık = doğru
değişken b: tensor = yok`, messages.H1007, 2, "'tensor'", "'Tensör'")
	singleError(t, `fonksiyon f(x: sozluk) { sonuç x }`, messages.H1007, 1, "'sozluk'", "'Sözlük'")
	singleError(t, `fonksiyon f(x) -> mantik { sonuç doğru }`, messages.H1007, 1, "'mantik'", "'Mantık'")
	// Kaldırılan eş yazımlar da hata verir ve asıl adı önerir.
	for name, canonical := range map[string]string{"int": "Tam", "float": "Ondalık", "str": "Metin", "string": "Metin",
		"bool": "Mantık", "null": "Yok", "list": "Liste", "dict": "Sözlük", "map": "Sözlük", "matrix": "Tensör"} {
		singleError(t, "değişken a: "+name, messages.H1007, 1, "'"+name+"'", "'"+canonical+"'")
	}
}

// Tip adının tek yazımı vardır (3.14): yalnızca büyük/küçük harfi farklı bir yazım da hata
// verir ve asıl yazımı söyler (tek hata: aynı satırda tip uyuşmazlığı bildirilmez).
func TestTypeNameCaseSpelling(t *testing.T) {
	for name, canonical := range map[string]string{"tam": "Tam", "TAM": "Tam", "ondalık": "Ondalık", "ONDALIK": "Ondalık",
		"METİN": "Metin", "mantık": "Mantık", "aralık": "Aralık", "tensör": "Tensör", "YOK": "Yok",
		"yerleşikFonksiyon": "YerleşikFonksiyon", "tip": "Tip"} {
		singleError(t, "değişken a: "+name, messages.H1006, 1, "'"+name+"'", "'"+canonical+"'")
	}
	singleError(t, `fonksiyon f(x: liste) { sonuç x }`, messages.H1006, 1, "'liste'", "'Liste'")
	singleError(t, `fonksiyon f(x) -> sözlük { sonuç {} }`, messages.H1006, 1, "'sözlük'", "'Sözlük'")
	singleError(t, "değişken a: tam = 1\ndeğişken b: Tam = a", messages.H1006, 1, "'tam'")
}

func TestTypeCompatibility(t *testing.T) {
	tests := []struct {
		expected, actual string
		compatible       bool
	}{
		{"Tam", "Tam", true},
		{"Ondalık", "Tam", true}, // Tam, Ondalık beklenen yere atanabilir
		{"Tam", "Ondalık", false},
		{"Metin", "Tam", false},
		{"Herhangi", "Metin", true},
		{"Metin", "Herhangi", true},
		{"Liste", "Yok", true}, // 'yok' her tipe atanabilir
		{"Yok", "Tam", false},
		{"tam", "Tam", true}, // kabul edilmeyen harf yazımı hatayı tipAdiDenetle verir (3.14); uyuşmazlık eklenmez
		{"Nokta", "Nokta", true},
		{"Nokta", "Tam", false},
		{"Liste", "Sözlük", false},
	}
	for _, tt := range tests {
		if got := typesCompatible(tt.expected, tt.actual); got != tt.compatible {
			t.Errorf("typesCompatible(%q, %q) = %v, beklenen %v", tt.expected, tt.actual, got, tt.compatible)
		}
	}
}

// ---------------------------------------------------------------------------
// Tip belirtimli değişkenler
// ---------------------------------------------------------------------------

func TestTypedVarCompatible(t *testing.T) {
	for _, source := range []string{
		`değişken a: Tam = 5`,
		`değişken a: Ondalık = 5`,
		`değişken a: Ondalık = 2.5`,
		`değişken a: Ondalık = 1 + 2.5`,
		`değişken a: Metin = "merhaba"`,
		`değişken x = 1
		 değişken a: Metin = "x = {x}"`,
		`değişken a: Metin = "a" + 1`,
		`değişken a: Mantık = doğru`,
		`değişken a: Mantık = 1 < 2`,
		`değişken a: Mantık = !5`,
		`değişken a: Mantık = doğru ve yanlış`,
		`değişken a: Liste = [1, 2]`,
		`değişken a: Aralık = 1..=5`,
		`değişken a: Ondalık = 7 / 2`,
		`değişken a: Ondalık = 2 ** -1`,
		`değişken a: Tam = 7 % 2`,
		`değişken a: Tam = (2 ** 3) * 4`,
		`değişken a = 7 / 2`,
		`değişken a: Liste = [x * 2 döngü x içinde 1..3]`,
		`değişken a: Sözlük = {"a": 1}`,
		"değişken m1 = birim_matris(2)\ndeğişken m2 = m1\ndeğişken a: Tensör = m1 @ m2",
		`değişken a: Tam = yok`,
		`değişken a: Tam`, // değer verilmezse 'yok' atanır
		"değişken bilinmeyen_tip = girdi()\ndeğişken a: Tam = bilinmeyen_tip",
		`değişken a: Tam = yazdır("x")`,
		`değişken a: Tam = 5 |> tam`,
		`değişken a: Tam = (5 ?? 6)`,
		`değişken a: Metin = yok ?? "varsayılan"`,
		`değişken a: Tam = -5`,
		`değişken a: Tam = 2 ** 3`,
		`değişken a: Fonksiyon = fonksiyon(x) { sonuç x }`,
		`sabit a: Tam = 1`,
	} {
		noErrors(t, source)
	}
}

func TestTypedVarIncompatible(t *testing.T) {
	tests := []struct {
		source, expected, actual string
	}{
		{`değişken yaş: Tam = "yirmi"`, "Tam", "Metin"},
		{`değişken a: Tam = 2.5`, "Tam", "Ondalık"},
		{`değişken a: Tam = 1 + 2.5`, "Tam", "Ondalık"},
		{`değişken a: Metin = 5`, "Metin", "Tam"},
		{`değişken a: Mantık = 1`, "Mantık", "Tam"},
		{`değişken a: Tam = 1 == 1`, "Tam", "Mantık"},
		{`değişken a: Tam = !doğru`, "Tam", "Mantık"},
		{`değişken a: Liste = {"a": 1}`, "Liste", "Sözlük"},
		{`değişken a: Sözlük = [1]`, "Sözlük", "Liste"},
		{`değişken a: Tam = 1..5`, "Tam", "Aralık"},
		{`değişken a: Liste = 1..5`, "Liste", "Aralık"},
		{`değişken a: Tam = 7 / 2`, "Tam", "Tam ya da Ondalık"},
		{`değişken a: Tam = 6 / 3`, "Tam", "Tam ya da Ondalık"},
		{`değişken a: Tam = 2 ** -1`, "Tam", "Tam ya da Ondalık"},
		{`değişken a: Metin = 7 / 2`, "Metin", "Tam ya da Ondalık"},
		{`değişken a: Metin = fonksiyon() { }`, "Metin", "Fonksiyon"},
		{`değişken a: Nokta = 5`, "Nokta", "Tam"},
		{`sabit a: Metin = doğru`, "Metin", "Mantık"},
		{`değişken a: Metin = 5 ?? "x"`, "Metin", "Tam"},
	}
	for _, tt := range tests {
		singleError(t, tt.source, messages.H1001, 1, `"`+tt.expected+`"`, `"`+tt.actual+`"`)
	}
}

func TestTypeErrorLineAndVarName(t *testing.T) {
	singleError(t, "değişken a = 1\n\ndeğişken öğrenci_sayısı: Tam = \"çok\"",
		messages.H1001, 3, `"öğrenci_sayısı" değişkeni`)
}

// ---------------------------------------------------------------------------
// Atamalar
// ---------------------------------------------------------------------------

func TestAssignmentTypeCheck(t *testing.T) {
	// Belirtilen tip korunur
	singleError(t, "değişken a: Tam = 1\na = \"x\"", messages.H1005, 2, `"a" değişkeni (Tam)`, `"Metin"`)
	noErrors(t, "değişken a: Ondalık = 1\na = 2.5\na = 3")

	// Dil dinamiktir: tip açıklaması olmayan değişkenin ilk değerden çıkarılan tipi kalıcı
	// bir bildirim değildir; başka tipte değer atanabilir. Sabitlerin tipi kalır.
	noErrors(t, "değişken a = 5\na = \"beş\"")
	noErrors(t, "x := doğru\nx = 1")
	noErrors(t, "değişken kayıp = 0\ndöngü i içinde 1..3 { kayıp += 0.5 }")
	singleError(t, "sabit S = 5\ndeğişken m: Metin = S", messages.H1001, 2, `"Metin"`, `"Tam"`)

	// 'yok' veya bilinmeyen tipte başlayan değişkenler serbestçe atanabilir
	noErrors(t, "değişken a = yok\na = 5\na = \"x\"")
	noErrors(t, "değişken a = girdi()\na = 5\na = \"x\"")
	noErrors(t, "değişken a\na = [1]")

	// Tanımlanmamış bir hedefe atama tanımsız ad hatasıdır (5.4, 5.14)
	singleError(t, "tanımsız = 5", messages.H1010, 1, "'tanımsız'")
}

func TestCompoundAssignment(t *testing.T) {
	// 's += 1' ile 's = s + 1' aynı şekilde değerlendirilmeli (Metin + Tam = Metin)
	noErrors(t, "değişken s = \"a\"\ns += 1")
	noErrors(t, "değişken s = \"a\"\ns = s + 1")
	noErrors(t, "değişken x = 10\nx += 1\nx -= 2\nx *= 3")
	noErrors(t, "değişken x = 1.5\nx += 1")
	noErrors(t, "değişken x: Ondalık = 1\nx *= 2.5")

	// 'x = x + 0.5' gibi, Tam açıklamalı bir değişkene Ondalık sonuç atanamaz; açıklamasız
	// değişken (biriktirici) serbesttir.
	singleError(t, "değişken x: Tam = 1\nx += 0.5", messages.H1005, 2, `"x" değişkeni (Tam)`, `"Ondalık"`)
	singleError(t, "değişken x: Tam = 1\nx = x + 0.5", messages.H1005, 2, `"x" değişkeni (Tam)`, `"Ondalık"`)
	noErrors(t, "değişken x = 1\nx += 0.5")

	// '/' Tam/Tam'da tam bölünmezse Ondalık verir (10 / 4 = 2.5): sonuç "Tam ya da Ondalık"tır.
	noErrors(t, "değişken x = 10\nx /= 4")
	singleError(t, "değişken y: Tam = 10 / 4", messages.H1001, 1, `"Tam ya da Ondalık"`, "tam(...)")
	singleError(t, "değişken y: Tam = 10\ny /= 4", messages.H1005, 2, `"Tam ya da Ondalık"`)
}

func TestConstCannotChange(t *testing.T) {
	singleError(t, "sabit PI = 3.14\nPI = 3.0", messages.H1002, 2, `"PI"`)
	singleError(t, "sabit SAYAÇ = 1\nSAYAÇ += 1", messages.H1002, 2, `"SAYAÇ"`)
	singleError(t, "sabit a: Metin = \"x\"\na = \"y\"", messages.H1002, 2)

	// Çoklu tanım ve çoklu atama
	singleError(t, "sabit a, b = 1, 2\nb = 3", messages.H1002, 2, `"b"`)
	singleError(t, "sabit a = 1\ndeğişken b = 2\nb, a = a, b", messages.H1002, 3, `"a"`)

	// İç kapsamlardan dış sabite atama da yakalanır
	singleError(t, "sabit a = 1\neğer doğru {\n  a = 2\n}", messages.H1002, 3)
	singleError(t, "sabit a = 1\nfonksiyon f() {\n  a = 2\n}", messages.H1002, 3)
	singleError(t, "sabit a = 1\ndöngü i içinde 1..3 {\n  a = i\n}", messages.H1002, 3)

	// Hem sabit hem tip hatası aynı anda bildirilir
	errs, coded := checkErrs(t, "sabit a = 1\na = \"x\"")
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1002) || !messages.Has(coded[1], messages.H1005) {
		t.Errorf("önce sabit, sonra tip hatası bekleniyordu, gelen: %v", errs)
	}
}

// İç kapsamda aynı adla tanımlanan değişken/parametre dış sabiti gölgeler.
func TestConstShadowing(t *testing.T) {
	noErrors(t, "sabit x = 1\neğer doğru {\n  değişken x = 2\n  x = 3\n}")
	noErrors(t, "sabit x = 1\nfonksiyon f(x) {\n  x = 5\n}")
	noErrors(t, "sabit x = 1\ndöngü x içinde [1, 2] {\n  x = 5\n}")
	noErrors(t, "sabit i = 1\ndöngü i, x içinde [1, 2] {\n  i = 5\n}")
	noErrors(t, "sabit e = 1\ndene {\n} yakala e {\n  e = \"x\"\n}")
	// Blok bitince dış sabit yeniden geçerlidir
	singleError(t, "sabit x = 1\neğer doğru {\n  değişken x = 2\n}\nx = 3", messages.H1002, 5)
}

// ---------------------------------------------------------------------------
// Fonksiyonlar
// ---------------------------------------------------------------------------

func TestFunctionParamTypes(t *testing.T) {
	def := "fonksiyon kare(x: Tam) { sonuç x * x }\n"
	singleError(t, def+`kare("üç")`, messages.H1004, 2, `"kare"`, "1. parametresi", `"Tam"`, `"Metin"`)
	noErrors(t, def+"kare(3)")
	noErrors(t, def+"kare(girdi())")
	noErrors(t, def+"kare(yok)")

	two := "fonksiyon böl(a: Ondalık, b: Ondalık) -> Ondalık { sonuç a / b }\n"
	noErrors(t, two+"böl(1, 2)")
	singleError(t, two+`böl(1, "iki")`, messages.H1004, 0, "2. parametresi", `"Ondalık"`, `"Metin"`)

	// Tipsiz parametreler her değeri kabul eder
	noErrors(t, "fonksiyon f(a, b: Tam) { }\nf(\"x\", 2)")
	singleError(t, "fonksiyon f(a, b: Tam) { }\nf(\"x\", \"y\")", messages.H1004, 0, "2. parametresi")

	// Parametreler fonksiyon gövdesinde bildirilen tipleriyle bilinir
	singleError(t, "fonksiyon f(a: Metin) {\n  değişken b: Tam = a\n}", messages.H1001, 2, `"b"`, `"Metin"`)

	// Özyinelemeli çağrılar da denetlenir (ad, gövdeden önce tanımlanır)
	singleError(t, "fonksiyon f(n: Tam) {\n  sonuç f(\"x\")\n}", messages.H1004, 2)

	// İç içe çağrılar denetlenir
	singleError(t, def+"yazdır(kare(\"a\"))", messages.H1004, 2)
	singleError(t, def+"kare(kare(\"a\"))", messages.H1004, 2)
	// Fazladan argümanlar ve isimli argümanlar içindeki çağrılar da denetlenir
	singleError(t, def+"kare(1, kare(\"a\"))", messages.H1004, 2)
	singleError(t, def+"yazdır(1, ayraç: kare(\"a\"))", messages.H1004, 2)
}

func TestFunctionReturnTypeInference(t *testing.T) {
	def := "fonksiyon topla(a: Tam, b: Tam) -> Tam { sonuç a + b }\n"
	singleError(t, def+"değişken s: Metin = topla(1, 2)", messages.H1001, 2, `"Metin"`, `"Tam"`)
	noErrors(t, def+"değişken s: Ondalık = topla(1, 2)")

	// Parametresiz fonksiyonların dönüş tipi de kullanılır
	singleError(t, "fonksiyon ad(): Metin { sonuç \"Ali\" }\ndeğişken n: Tam = ad()", messages.H1001, 2, `"Tam"`, `"Metin"`)

	// Dönüş tipi belirtilmemişse sonuç bilinmez (Herhangi)
	noErrors(t, "fonksiyon f(a: Tam) { sonuç a }\ndeğişken s: Metin = f(1)")
	noErrors(t, "fonksiyon f() { sonuç 1 }\ndeğişken s: Metin = f()")
}

func TestReturnTypeCheck(t *testing.T) {
	singleError(t, "fonksiyon f(): Tam {\n  sonuç \"x\"\n}", messages.H1003, 2, `"Tam"`, `"Metin"`)
	singleError(t, "fonksiyon f() -> Metin {\n  sonuç 1\n}", messages.H1003, 2)
	singleError(t, "fonksiyon f(): Tam { sonuç 2.5 }", messages.H1003, 0, `"Ondalık"`)
	// Tam / Tam Tam olmayabilir (7 / 2 = 3.5): ': Tam' bildiren fonksiyon bunu döndüremez
	singleError(t, "fonksiyon ort(a: Tam, b: Tam): Tam { sonuç a / b }", messages.H1003, 0, `"Tam ya da Ondalık"`)
	noErrors(t, "fonksiyon ort(a: Tam, b: Tam): Ondalık { sonuç a / b }")

	// İç bloklardaki dönüşler de denetlenir
	singleError(t, "fonksiyon f(x): Tam {\n  eğer x {\n    sonuç \"a\"\n  }\n  sonuç 1\n}", messages.H1003, 3)
	singleError(t, "fonksiyon f(x): Tam {\n  döngü i içinde x {\n    sonuç doğru\n  }\n}", messages.H1003, 3)

	for _, source := range []string{
		"fonksiyon f(): Ondalık { sonuç 1 }",       // Tam -> Ondalık
		"fonksiyon f(): Liste { sonuç yok }",       // 'yok' her tipe uyar
		"fonksiyon f(): Tam { sonuç }",             // boş dönüş 'yok' sayılır
		"fonksiyon f(x): Tam { sonuç x }",          // bilinmeyen tip
		"fonksiyon f(x: Tam): Tam { sonuç x * 2 }", // çıkarılan tip uyumlu
		"fonksiyon f(): Mantık { sonuç 1 < 2 }",    // karşılaştırma
		"fonksiyon f(): Fonksiyon { sonuç (x) => x }",
		"fonksiyon f() { sonuç \"x\" }", // dönüş tipi yoksa denetim yok
		"sonuç \"x\"",                   // fonksiyon dışındaki dönüş denetlenmez
	} {
		noErrors(t, source)
	}

	// İç fonksiyonlar dış fonksiyonun dönüş tipini devralmaz
	noErrors(t, "fonksiyon f(): Tam {\n  değişken g = (x) => \"metin\"\n  sonuç 1\n}")
	noErrors(t, "fonksiyon f(): Tam {\n  fonksiyon g() { sonuç \"metin\" }\n  sonuç 1\n}")
	// İç fonksiyonun kendi dönüş tipi denetlenir; dış fonksiyonunki geri yüklenir
	errs, coded := checkErrs(t, "fonksiyon f(): Tam {\n  fonksiyon g(): Metin { sonuç 1 }\n  sonuç \"x\"\n}")
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1003) || !messages.Has(coded[1], messages.H1003) ||
		!strings.Contains(errs[0], `"Metin"`) || !strings.Contains(errs[1], `"Tam"`) {
		t.Errorf("iç ve dış fonksiyon için birer dönüş hatası bekleniyordu, gelen: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Denetimin tüm deyim türlerine ulaşması
// ---------------------------------------------------------------------------

func TestNestedStatementsAreChecked(t *testing.T) {
	def := "fonksiyon kare(x: Tam) { sonuç x * x }\n"
	invalid := `kare("a")`
	tests := map[string]string{
		"eğer":             "eğer doğru { " + invalid + " }",
		"değilse eğer":     "eğer yanlış { } değilse eğer doğru { " + invalid + " }",
		"değilse":          "eğer yanlış { } değilse { " + invalid + " }",
		"eğer koşulu":      "eğer " + invalid + " > 1 { }",
		"iken":             "iken yanlış { " + invalid + " }",
		"iken koşulu":      "iken " + invalid + " > 1 { }",
		"döngü":            "döngü i içinde 1..3 { " + invalid + " }",
		"döngü kaynağı":    "döngü i içinde " + invalid + " { }",
		"dene":             "dene { " + invalid + " } yakala h { }",
		"yakala":           "dene { } yakala h { " + invalid + " }",
		"eşle gövde":       "eşle 1 { 1 => { " + invalid + " } }",
		"eşle ifade":       "eşle 1 { 1 => " + invalid + " }",
		"eşle konusu":      "eşle " + invalid + " { varsayılan => 1 }",
		"ertele blok":      "ertele { " + invalid + " }",
		"ertele çağrı":     "ertele " + invalid,
		"sonuç":            "fonksiyon g() { sonuç " + invalid + " }",
		"fırlat":           "fırlat " + invalid,
		"eşzamanlı blok":   "eşzamanlı { " + invalid + " }",
		"eşzamanlı çağrı":  "eşzamanlı " + invalid,
		"kesme noktası":    "kesme_noktası eğer " + invalid + " > 1",
		"lambda":           "değişken g = (y) => " + invalid,
		"çoklu tanım":      "değişken a, b = 1, " + invalid,
		"çoklu atama":      "değişken a = 1\ndeğişken b = 2\na, b = " + invalid + ", 1",
		"atama değeri":     "değişken a = 1\na = " + invalid,
		"liste dışı ifade": "yazdır(" + invalid + ")",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			singleError(t, def+body, messages.H1004, 0)
		})
	}
}

func TestMultipleErrorsInOrder(t *testing.T) {
	errs, coded := checkErrs(t, `değişken a: Tam = "x"
sabit b = 1
b = 2
fonksiyon f(x: Metin): Tam { sonuç x }
f(5)`)
	expected := []struct {
		line int
		code messages.Code
	}{{1, messages.H1001}, {3, messages.H1002}, {4, messages.H1003}, {5, messages.H1004}}
	if len(errs) != len(expected) {
		t.Fatalf("%d hata bekleniyordu, gelen: %v", len(expected), errs)
	}
	for i, b := range expected {
		expectCode(t, coded[i], b.code, b.line)
		// Metin biçimi: "Satır N: ..." ile başlar.
		if prefix := fmt.Sprintf("Satır %d: ", b.line); !strings.HasPrefix(errs[i], prefix) {
			t.Errorf("hata[%d] %q ile başlamalı, gelen: %q", i, prefix, errs[i])
		}
	}
}

// Yerleşik bir adı kullanıcı adı olarak tanımlamak hata değil uyarıdır: program çalışır,
// ama o kapsamda yerleşik artık kullanılamaz.
func TestBuiltinShadowingWarning(t *testing.T) {
	source := `değişken uzunluk = 5
fonksiyon f(toplam, x) {
    sonuç toplam + x
}
fonksiyon girdi() { sonuç 1 }
döngü i, metin içinde ["a"] { yazdır(metin) }
dene { fırlat "x" } yakala ekle { yazdır(ekle) }
değişken a, sırala = 1, 2
yapı Nokta {
    x: Tam
    fonksiyon uzunluk() { sonuç 1 }
}
değişken sayaç = uzunluk + sırala`
	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	c := New()
	if errs := c.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("gölgeleme hata değildir: %v", errs)
	}
	// uzunluk, ekle ve sırala artık yerleşik değil (metotlar): gölgeleme sayılmaz.
	// Uyarı konumuyla (yapı) ve koduyla (H1009) sınanır; metne bakılmaz.
	expected := map[int]string{2: "toplam", 5: "girdi", 6: "metin"}
	got := c.Warnings()
	if len(got) != len(expected) {
		t.Fatalf("%d uyarı bekleniyordu: %v", len(expected), got)
	}
	for _, w := range got {
		line, col, ok := messages.Pos(w)
		if !ok || col != 0 || !messages.Has(w, messages.H1009) || !strings.Contains(w.Error(), "'"+expected[line]+"'") {
			t.Errorf("uyarı H1009 olmalı, satır %d (%v) için '%s' adını taşımalı: %v", line, ok, expected[line], w)
		}
	}
	// Yapı metotları ve alanları yerleşikleri gölgelemez (x.uzunluk() ile erişilir).
	if u := New().Warnings(); len(u) != 0 {
		t.Errorf("yeni denetleyicide uyarı olmamalı: %v", u)
	}
}

// Değeri kullanılmayan '-x' deyimi uyarı verir (2.8): satır başındaki '-' önceki satırı
// sürdürmez, bu yüzden 'toplam = a' ardından gelen '- b' satırı sessizce etkisiz kalırdı (Kotlin
// de bu durumda uyarır). Kabukta (etkileşimli) ifadenin değeri gösterildiği için uyarı yoktur.
func TestUnusedNegationWarns(t *testing.T) {
	check := func(code string, interactive bool) []error {
		p := parser.New(lexer.New(code))
		prog := p.ParseProgram()
		c := New()
		if interactive {
			c.SetInteractive()
		}
		c.CheckProgram(prog)
		return c.Warnings()
	}
	w := check("değişken b = 2\ndeğişken a = 1\n    - b", false)
	if len(w) != 1 || !messages.Has(w[0], messages.H1011) {
		t.Fatalf("H1011 uyarısı bekleniyordu: %v", w)
	}
	if line, _, _ := messages.Pos(w[0]); line != 3 {
		t.Errorf("uyarı 3. satırda olmalı: %v", w[0])
	}
	if w := check("değişken b = 2\ndeğişken a = 1 -\n    b\ndeğişken c = -b", false); len(w) != 0 {
		t.Errorf("kullanılan değerde uyarı olmamalı: %v", w)
	}
	if w := check("değişken b = 2\n-b", true); len(w) != 0 {
		t.Errorf("kabukta uyarı olmamalı: %v", w)
	}
}

func TestEmptyProgram(t *testing.T) {
	noErrors(t, "")
	noErrors(t, "// yalnızca yorum")
	if errs := New().Errors(); errs == nil || len(errs) != 0 {
		t.Errorf("yeni denetleyicinin hata listesi boş (nil olmayan) olmalı, gelen: %#v", errs)
	}
}

// ---------------------------------------------------------------------------
// Bilinen sınırlamalar: denetleyici şu anda bunları YAKALAMAZ.
// Bu testler mevcut davranışı belgeler; bir sınırlama giderildiğinde ilgili
// satır hata bekleyecek şekilde güncellenmelidir.
// ---------------------------------------------------------------------------

func TestKnownLimitations(t *testing.T) {
	tests := []struct {
		name, source string
	}{
		{"tanımdan önce çağrı (öne alma yok)", "kare(\"a\")\nfonksiyon kare(x: Tam) { }"},
		{"değişkene atanan fonksiyonun parametre tipleri unutulur", "değişken kare = fonksiyon(x: Tam) { sonuç x }\nkare(\"a\")"},
		{"lambda parametre tipleri", "değişken f = (x: Tam) => x\nf(\"a\")"},
		{"eksik argüman", "fonksiyon f(a: Tam, b: Tam) { }\nf(1)"},
		{"fazla argüman", "fonksiyon f(a: Tam) { }\nf(1, 2, 3)"},
		{"isimli argüman tipi", "fonksiyon f(a: Tam) { }\nf(a: \"x\")"},
		{"dönüş tipi olan fonksiyonda eksik 'sonuç'", "fonksiyon f(): Tam { }"},
		{"sabitin yeniden tanımlanması", "sabit x = 1\ndeğişken x = 2\nx = 3"},
		{"sabitin aynı adlı fonksiyonla ezilmesi", "sabit f = 1\nfonksiyon f() { }\nf = fonksiyon() { }"},
		{"sabit listenin elemanını değiştirme", "sabit l = [1, 2]\nl[0] = 5"},
		{"sabit sözlüğün alanını değiştirme", "sabit p = {\"x\": 1}\np.x = 5"},
		{"çoklu atamada tip denetimi yok", "değişken x: Tam = 1\ndeğişken y = 2\nx, y = \"a\", 2"},
		{"çoklu tanımda değişkenler tipsizdir", "değişken a, b = 1, 2\na = \"x\""},
		{"yapı alan tipleri", "yapı Nokta {\n  x: Tam\n}\ndeğişken p = Nokta(\"a\")"},
		{"döngü değişkenleri tipsizdir", "döngü x içinde [1, 2] { değişken s: Metin = x }"},
		{"uyumsuz işlenenler", "değişken a = \"x\" - 1\ndeğişken b = doğru + 1"},
		{"koşul tipi", "eğer 5 { }\niken \"x\" { }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noErrors(t, tt.source)
		})
	}

	// Yakalanan hata değişkeni açıklamasızdır: başka tipte değer alabilir.
	noErrors(t, "dene { } yakala h {\n  h = 5\n}")
}

// ---------------------------------------------------------------------------
// Örnek programlar yanlış alarm üretmemeli
// ---------------------------------------------------------------------------

func TestExamplesCheckWithoutErrors(t *testing.T) {
	files, _ := reporoot.Glob(reporoot.Path("examples/tr/*.tr"))
	if len(files) == 0 {
		t.Fatal("examples/tr/*.tr bulunamadı")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			noErrors(t, string(data))
		})
	}
	// Depodaki programlar (örnekler, dil tanımı testleri ve kıyas programları)
	// uyarısız denetlenmeli: yerleşik adları gölgeleyen ya da yazılış kuralına (2.3b) uymayan
	// kullanıcı adı olmamalı.
	for _, file := range testkit.GlobFiles(t, "examples/tr/*.tr", "examples/tr/helpers/*.tr", "tests/*.tr", "tools/bench/tr/*.tr") {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(strings.TrimPrefix(string(data), "\ufeff")))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Errorf("%s: %v", file, p.Errors())
			continue
		}
		c := New()
		c.CheckProgram(prog)
		if u := c.Warnings(); len(u) > 0 {
			t.Errorf("%s uyarısız denetlenmeli: %v", file, u)
		}
	}
}

// Tip açıklamasında kabul edilen yazımlar sözlükten gelir (A2xx); her tipin tek yazımı
// vardır, büyük/küçük harfi farklı biçim kabul edilen bir yazım değildir (3.14).
func TestTypeSpellingsUnique(t *testing.T) {
	expected := []string{"Tam", "Ondalık", "Metin", "Mantık", "Liste", "Sözlük", "Tensör", "Yok",
		"Aralık", "Fonksiyon", "YerleşikFonksiyon", "Tip"}
	if len(typeSpellings) != len(expected) {
		t.Errorf("%d yazım bekleniyordu, %d var: %v", len(expected), len(typeSpellings), typeSpellings)
	}
	for _, typ := range expected {
		if got, ok := typeSpellings[typ]; !ok || got != typ {
			t.Errorf("tipYazimlari[%q] = %q, %v; kendisi bekleniyordu", typ, got, ok)
		}
		if _, ok := typeSpellings[turkish.Lower(typ)]; ok {
			t.Errorf("%q kabul edilen bir yazım olmamalı (3.14)", turkish.Lower(typ))
		}
	}
	for _, c := range []struct{ in, want string }{{"", "Herhangi"}, {"TAM", "Tam"}, {"Herhangi", "Herhangi"}, {"herhangi", "herhangi"}, {"Nokta", "Nokta"}} {
		if got := normalizeTypeName(c.in); got != c.want {
			t.Errorf("normalizeTypeName(%q) = %q, %q bekleniyordu", c.in, got, c.want)
		}
	}
}

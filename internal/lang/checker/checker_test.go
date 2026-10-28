package checker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// ---------------------------------------------------------------------------
// Tip adları ve uyumluluk kuralları
// ---------------------------------------------------------------------------

// normalizeTypeName kabul edilmeyen harf yazımını (tam, int) da asıl tipe çevirir: hatayı
// checkedTypeName verir, aynı satırda ayrıca tip uyuşmazlığı çıkmaz. Program içeride kimliklerle
// çalışır: asıl tip adı tipin kimliğidir; dili bilinmeyen adın ipucu her yazım dilinden gelir.
func TestTypeNameNormalization(t *testing.T) {
	tests := map[string]codes.Code{
		"TR_type_Int": codes.TypeInt, "tam": codes.TypeInt, "TAM": codes.TypeInt, "int": codes.TypeInt,
		"TR_type_Float": codes.TypeFloat, "ONDALIK": codes.TypeFloat, "float": codes.TypeFloat,
		"TR_type_String": codes.TypeString, "METİN": codes.TypeString,
		"TR_type_Bool": codes.TypeBool, "MANTIK": codes.TypeBool,
		"TR_type_List": codes.TypeList, "TR_type_Dict": codes.TypeDict, "TR_type_Tensor": codes.TypeTensor, "TR_type_None": codes.TypeNone,
		"Fonksiyon": codes.TypeFunction, "": codes.TypeAny,
	}
	for input, want := range tests {
		if got := normalizeTypeName(input); got != want.String() {
			t.Errorf("normalizeTypeName(%q) = %q, beklenen %q", input, got, want)
		}
	}
	// Bilinmeyen adlar (kullanıcı yapıları) olduğu gibi kalır.
	for _, name := range []string{"Nokta", "Nesne"} {
		if got := normalizeTypeName(name); got != name {
			t.Errorf("normalizeTypeName(%q) = %q, olduğu gibi kalmalı", name, got)
		}
	}
}

// Kural (tek hata, tek mesaj): kabul edilmeyen bir tip yazımı tek hata verir ve dosyanın dilindeki
// asıl adı önerir; aynı satırda ayrıca tip uyuşmazlığı bildirilmez. Türkçe dosyada Türkçe
// karaktersiz yazım (ondalik) ve başka dillerden alışılmış adlar (int, dict) Türkçe adı, İngilizce
// dosyada başka bir yazım dilinin adı (Ondalık) İngilizce adı önerir.
func TestTypeNameASCIISuggestion(t *testing.T) {
	singleErrorIn(t, "tr", "TR_var a: ondalik = 1.5", messages.H1007, 1, "'ondalik'", "'Ondalık'")
	singleErrorIn(t, "tr", "TR_var a: TR_Bool = TR_true\nTR_var b: tensor = TR_none", messages.H1007, 2, "'tensor'", "'Tensör'")
	singleErrorIn(t, "tr", "TR_func f(x: sozluk) { TR_return x }", messages.H1007, 1, "'sozluk'", "'Sözlük'")
	singleErrorIn(t, "tr", "TR_func f(x) -> mantik { TR_return TR_true }", messages.H1007, 1, "'mantik'", "'Mantık'")
	for name, canonical := range map[string]string{"int": "Tam", "float": "Ondalık", "str": "Metin", "string": "Metin",
		"bool": "Mantık", "null": "Yok", "list": "Liste", "dict": "Sözlük", "map": "Sözlük", "matrix": "Tensör"} {
		singleErrorIn(t, "tr", "TR_var a: "+name, messages.H1007, 1, "'"+name+"'", "'"+canonical+"'")
	}
	for name, canonical := range map[string]string{"Ondalık": "Float", "Tam": "Int", "Sözlük": "Dict"} {
		singleErrorIn(t, "en", "TR_var a: "+name, messages.H1007, 1, "'"+name+"'", "'"+canonical+"'")
	}
}

// Kural (3.14, tek hata, tek mesaj): tip adının her yazım dilinde tek yazımı vardır; yalnızca
// büyük/küçük harfi farklı bir yazım da tek hata verir ve dosyanın dilindeki asıl yazımı söyler.
func TestTypeNameCaseSpelling(t *testing.T) {
	for lang, names := range map[string]map[string]string{
		"tr": {"tam": "Tam", "TAM": "Tam", "ondalık": "Ondalık", "ONDALIK": "Ondalık", "METİN": "Metin", "mantık": "Mantık",
			"aralık": "Aralık", "tensör": "Tensör", "YOK": "Yok", "yerleşikFonksiyon": "YerleşikFonksiyon", "tip": "Tip"},
		"en": {"int": "Int", "FLOAT": "Float", "string": "String", "bool": "Bool", "list": "List"},
	} {
		for name, canonical := range names {
			singleErrorIn(t, lang, "TR_var a: "+name, messages.H1006, 1, "'"+name+"'", "'"+canonical+"'")
		}
	}
	singleErrorIn(t, "tr", "TR_func f(x: liste) { TR_return x }", messages.H1006, 1, "'liste'", "'Liste'")
	singleErrorIn(t, "tr", "TR_func f(x) -> sözlük { TR_return {} }", messages.H1006, 1, "'sözlük'", "'Sözlük'")
	singleErrorIn(t, "tr", "TR_var a: tam = 1\nTR_var b: TR_Int = a", messages.H1006, 1, "'tam'")
}

// Tip uyumu kimlikler üzerindedir: Tam Ondalık beklenen yere, yok her tipe atanabilir; Herhangi
// her şeyle uyumludur; kabul edilmeyen harf yazımı ayrıca uyuşmazlık sayılmaz (hatayı
// checkedTypeName verir).
func TestTypeCompatibility(t *testing.T) {
	i, f, s, l := codes.TypeInt.String(), codes.TypeFloat.String(), codes.TypeString.String(), codes.TypeList.String()
	tests := []struct {
		expected, actual string
		compatible       bool
	}{
		{i, i, true},
		{f, i, true},
		{i, f, false},
		{s, i, false},
		{typeAny, s, true},
		{s, typeAny, true},
		{l, typeNull, true},
		{typeNull, i, false},
		{"tam", i, true},
		{"Nokta", "Nokta", true},
		{"Nokta", i, false},
		{l, codes.TypeDict.String(), false},
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
		"TR_var a: TR_Int = 5",
		"TR_var a: TR_Float = 5",
		"TR_var a: TR_Float = 2.5",
		"TR_var a: TR_Float = 1 + 2.5",
		`TR_var a: TR_String = "merhaba"`,
		`TR_var x = 1
		 TR_var a: TR_String = "x = \(x)"`,
		`TR_var a: TR_String = "a" + 1`,
		"TR_var a: TR_Bool = TR_true",
		"TR_var a: TR_Bool = 1 < 2",
		"TR_var a: TR_Bool = !5",
		"TR_var a: TR_Bool = TR_true TR_and TR_false",
		"TR_var a: TR_List = [1, 2]",
		"TR_var a: TR_Range = 1..=5",
		"TR_var a: TR_Float = 7 / 2",
		"TR_var a: TR_Float = 2 ** -1",
		"TR_var a: TR_Int = 7 % 2",
		"TR_var a: TR_Int = (2 ** 3) * 4",
		"TR_var a = 7 / 2",
		"TR_var a: TR_List = [x * 2 TR_for x TR_in 1..3]",
		`TR_var a: TR_Dict = {"a": 1}`,
		"TR_var m1 = TR_eye(2)\nTR_var m2 = m1\nTR_var a: TR_Tensor = m1 @ m2",
		"TR_var a: TR_Int = TR_none",
		"TR_var a: TR_Int", // değer verilmezse 'yok' atanır
		"TR_var bilinmeyen_tip = TR_input()\nTR_var a: TR_Int = bilinmeyen_tip",
		`TR_var a: TR_Int = TR_print("x")`,
		"TR_var a: TR_Int = 5 |> TR_int",
		"TR_var a: TR_Int = (5 ?? 6)",
		`TR_var a: TR_String = TR_none ?? "varsayılan"`,
		"TR_var a: TR_Int = -5",
		"TR_var a: TR_Int = 2 ** 3",
		"TR_var a: TR_Function = TR_func(x) { TR_return x }",
		"TR_const a: TR_Int = 1",
	} {
		noErrors(t, source)
	}
}

func TestTypedVarIncompatible(t *testing.T) {
	tests := []struct {
		source, expected, actual string
	}{
		{`TR_var yaş: TR_Int = "yirmi"`, "TR_Int", "TR_String"},
		{"TR_var a: TR_Int = 2.5", "TR_Int", "TR_Float"},
		{"TR_var a: TR_Int = 1 + 2.5", "TR_Int", "TR_Float"},
		{"TR_var a: TR_String = 5", "TR_String", "TR_Int"},
		{"TR_var a: TR_Bool = 1", "TR_Bool", "TR_Int"},
		{"TR_var a: TR_Int = 1 == 1", "TR_Int", "TR_Bool"},
		{"TR_var a: TR_Int = !TR_true", "TR_Int", "TR_Bool"},
		{`TR_var a: TR_List = {"a": 1}`, "TR_List", "TR_Dict"},
		{"TR_var a: TR_Dict = [1]", "TR_Dict", "TR_List"},
		{"TR_var a: TR_Int = 1..5", "TR_Int", "TR_Range"},
		{"TR_var a: TR_List = 1..5", "TR_List", "TR_Range"},
		{"TR_var a: TR_Int = 7 / 2", "TR_Int", "TR_Int ya da TR_Float"},
		{"TR_var a: TR_Int = 6 / 3", "TR_Int", "TR_Int ya da TR_Float"},
		{"TR_var a: TR_Int = 2 ** -1", "TR_Int", "TR_Int ya da TR_Float"},
		{"TR_var a: TR_String = 7 / 2", "TR_String", "TR_Int ya da TR_Float"},
		{"TR_var a: TR_String = TR_func() { }", "TR_String", "TR_Function"},
		{"TR_var a: Nokta = 5", "Nokta", "TR_Int"},
		{"TR_const a: TR_String = TR_true", "TR_String", "TR_Bool"},
		{`TR_var a: TR_String = 5 ?? "x"`, "TR_String", "TR_Int"},
	}
	for _, tt := range tests {
		singleError(t, tt.source, messages.H1001, 1, `"`+tt.expected+`"`, `"`+tt.actual+`"`)
	}
}

func TestTypeErrorLineAndVarName(t *testing.T) {
	singleError(t, "TR_var a = 1\n\nTR_var öğrenci_sayısı: TR_Int = \"çok\"",
		messages.H1001, 3, `"öğrenci_sayısı" değişkeni`)
}

// ---------------------------------------------------------------------------
// Atamalar
// ---------------------------------------------------------------------------

func TestAssignmentTypeCheck(t *testing.T) {
	// Belirtilen tip korunur
	singleError(t, "TR_var a: TR_Int = 1\na = \"x\"", messages.H1005, 2, `"a" değişkeni (TR_Int)`, `"TR_String"`)
	noErrors(t, "TR_var a: TR_Float = 1\na = 2.5\na = 3")

	// Dil dinamiktir: tip açıklaması olmayan değişkenin ilk değerden çıkarılan tipi kalıcı
	// bir bildirim değildir; başka tipte değer atanabilir. Sabitlerin tipi kalır.
	noErrors(t, "TR_var a = 5\na = \"beş\"")
	noErrors(t, "x := TR_true\nx = 1")
	noErrors(t, "TR_var kayıp = 0\nTR_for i TR_in 1..3 { kayıp += 0.5 }")
	singleError(t, "TR_const S = 5\nTR_var m: TR_String = S", messages.H1001, 2, `"TR_String"`, `"TR_Int"`)

	// 'yok' veya bilinmeyen tipte başlayan değişkenler serbestçe atanabilir
	noErrors(t, "TR_var a = TR_none\na = 5\na = \"x\"")
	noErrors(t, "TR_var a = TR_input()\na = 5\na = \"x\"")
	noErrors(t, "TR_var a\na = [1]")

	// Tanımlanmamış bir hedefe atama tanımsız ad hatasıdır (5.4, 5.14)
	singleError(t, "tanımsız = 5", messages.H1010, 1, "'tanımsız'")
}

func TestCompoundAssignment(t *testing.T) {
	// 's += 1' ile 's = s + 1' aynı şekilde değerlendirilmeli (Metin + Tam = Metin)
	noErrors(t, "TR_var s = \"a\"\ns += 1")
	noErrors(t, "TR_var s = \"a\"\ns = s + 1")
	noErrors(t, "TR_var x = 10\nx += 1\nx -= 2\nx *= 3")
	noErrors(t, "TR_var x = 1.5\nx += 1")
	noErrors(t, "TR_var x: TR_Float = 1\nx *= 2.5")

	// 'x = x + 0.5' gibi, Tam açıklamalı bir değişkene Ondalık sonuç atanamaz; açıklamasız
	// değişken (biriktirici) serbesttir.
	singleError(t, "TR_var x: TR_Int = 1\nx += 0.5", messages.H1005, 2, `"x" değişkeni (TR_Int)`, `"TR_Float"`)
	singleError(t, "TR_var x: TR_Int = 1\nx = x + 0.5", messages.H1005, 2, `"x" değişkeni (TR_Int)`, `"TR_Float"`)
	noErrors(t, "TR_var x = 1\nx += 0.5")

	// '/' Tam/Tam'da tam bölünmezse Ondalık verir (10 / 4 = 2.5): sonuç "TR_Int ya da TR_Float"tır.
	noErrors(t, "TR_var x = 10\nx /= 4")
	singleError(t, "TR_var y: TR_Int = 10 / 4", messages.H1001, 1, `"TR_Int ya da TR_Float"`, "TR_int(...)")
	singleError(t, "TR_var y: TR_Int = 10\ny /= 4", messages.H1005, 2, `"TR_Int ya da TR_Float"`)
}

func TestConstCannotChange(t *testing.T) {
	singleError(t, "TR_const PI = 3.14\nPI = 3.0", messages.H1002, 2, `"PI"`)
	singleError(t, "TR_const SAYAÇ = 1\nSAYAÇ += 1", messages.H1002, 2, `"SAYAÇ"`)
	singleError(t, "TR_const a: TR_String = \"x\"\na = \"y\"", messages.H1002, 2)

	// Çoklu tanım ve çoklu atama
	singleError(t, "TR_const a, b = 1, 2\nb = 3", messages.H1002, 2, `"b"`)
	singleError(t, "TR_const a = 1\nTR_var b = 2\nb, a = a, b", messages.H1002, 3, `"a"`)

	// İç kapsamlardan dış sabite atama da yakalanır
	singleError(t, "TR_const a = 1\nTR_if TR_true {\n  a = 2\n}", messages.H1002, 3)
	singleError(t, "TR_const a = 1\nTR_func f() {\n  a = 2\n}", messages.H1002, 3)
	singleError(t, "TR_const a = 1\nTR_for i TR_in 1..3 {\n  a = i\n}", messages.H1002, 3)

	// Hem sabit hem tip hatası aynı anda bildirilir
	errs, coded := checkErrs(t, "TR_const a = 1\na = \"x\"")
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1002) || !messages.Has(coded[1], messages.H1005) {
		t.Errorf("önce sabit, sonra tip hatası bekleniyordu, gelen: %v", errs)
	}
}

// İç kapsamda aynı adla tanımlanan değişken/parametre dış sabiti gölgeler.
func TestConstShadowing(t *testing.T) {
	noErrors(t, "TR_const x = 1\nTR_if TR_true {\n  TR_var x = 2\n  x = 3\n}")
	noErrors(t, "TR_const x = 1\nTR_func f(x) {\n  x = 5\n}")
	noErrors(t, "TR_const x = 1\nTR_for x TR_in [1, 2] {\n  x = 5\n}")
	noErrors(t, "TR_const i = 1\nTR_for i, x TR_in [1, 2] {\n  i = 5\n}")
	noErrors(t, "TR_const e = 1\nTR_try {\n} TR_catch e {\n  e = \"x\"\n}")
	// Blok bitince dış sabit yeniden geçerlidir
	singleError(t, "TR_const x = 1\nTR_if TR_true {\n  TR_var x = 2\n}\nx = 3", messages.H1002, 5)
}

// ---------------------------------------------------------------------------
// Fonksiyonlar
// ---------------------------------------------------------------------------

func TestFunctionParamTypes(t *testing.T) {
	def := "TR_func kare(x: TR_Int) { TR_return x * x }\n"
	singleError(t, def+`kare("üç")`, messages.H1004, 2, `"kare"`, "1. parametresi", `"TR_Int"`, `"TR_String"`)
	noErrors(t, def+"kare(3)")
	noErrors(t, def+"kare(TR_input())")
	noErrors(t, def+"kare(TR_none)")

	two := "TR_func böl(a: TR_Float, b: TR_Float) -> TR_Float { TR_return a / b }\n"
	noErrors(t, two+"böl(1, 2)")
	singleError(t, two+`böl(1, "iki")`, messages.H1004, 0, "2. parametresi", `"TR_Float"`, `"TR_String"`)

	// Tipsiz parametreler her değeri kabul eder
	noErrors(t, "TR_func f(a, b: TR_Int) { }\nf(\"x\", 2)")
	singleError(t, "TR_func f(a, b: TR_Int) { }\nf(\"x\", \"y\")", messages.H1004, 0, "2. parametresi")

	// Parametreler fonksiyon gövdesinde bildirilen tipleriyle bilinir
	singleError(t, "TR_func f(a: TR_String) {\n  TR_var b: TR_Int = a\n}", messages.H1001, 2, `"b"`, `"TR_String"`)

	// Özyinelemeli çağrılar da denetlenir (ad, gövdeden önce tanımlanır)
	singleError(t, "TR_func f(n: TR_Int) {\n  TR_return f(\"x\")\n}", messages.H1004, 2)

	// İç içe çağrılar denetlenir
	singleError(t, def+`TR_print(kare("a"))`, messages.H1004, 2)
	singleError(t, def+"kare(kare(\"a\"))", messages.H1004, 2)
	// Fazladan argümanlar ve isimli argümanlar içindeki çağrılar da denetlenir
	singleError(t, def+"kare(1, kare(\"a\"))", messages.H1004, 2)
	singleError(t, def+`TR_print(1, ayraç: kare("a"))`, messages.H1004, 2)
}

func TestFunctionReturnTypeInference(t *testing.T) {
	def := "TR_func topla(a: TR_Int, b: TR_Int) -> TR_Int { TR_return a + b }\n"
	singleError(t, def+"TR_var s: TR_String = topla(1, 2)", messages.H1001, 2, `"TR_String"`, `"TR_Int"`)
	noErrors(t, def+"TR_var s: TR_Float = topla(1, 2)")

	// Parametresiz fonksiyonların dönüş tipi de kullanılır
	singleError(t, "TR_func ad(): TR_String { TR_return \"Ali\" }\nTR_var n: TR_Int = ad()", messages.H1001, 2, `"TR_Int"`, `"TR_String"`)

	// Dönüş tipi belirtilmemişse sonuç bilinmez (Herhangi)
	noErrors(t, "TR_func f(a: TR_Int) { TR_return a }\nTR_var s: TR_String = f(1)")
	noErrors(t, "TR_func f() { TR_return 1 }\nTR_var s: TR_String = f()")
}

func TestReturnTypeCheck(t *testing.T) {
	singleError(t, "TR_func f(): TR_Int {\n  TR_return \"x\"\n}", messages.H1003, 2, `"TR_Int"`, `"TR_String"`)
	singleError(t, "TR_func f() -> TR_String {\n  TR_return 1\n}", messages.H1003, 2)
	singleError(t, "TR_func f(): TR_Int { TR_return 2.5 }", messages.H1003, 0, `"TR_Float"`)
	// Tam / Tam Tam olmayabilir (7 / 2 = 3.5): ': Tam' bildiren fonksiyon bunu döndüremez
	singleError(t, "TR_func ort(a: TR_Int, b: TR_Int): TR_Int { TR_return a / b }", messages.H1003, 0, `"TR_Int ya da TR_Float"`)
	noErrors(t, "TR_func ort(a: TR_Int, b: TR_Int): TR_Float { TR_return a / b }")

	// İç bloklardaki dönüşler de denetlenir
	singleError(t, "TR_func f(x): TR_Int {\n  TR_if x {\n    TR_return \"a\"\n  }\n  TR_return 1\n}", messages.H1003, 3)
	singleError(t, "TR_func f(x): TR_Int {\n  TR_for i TR_in x {\n    TR_return TR_true\n  }\n}", messages.H1003, 3)

	for _, source := range []string{
		"TR_func f(): TR_Float { TR_return 1 }",            // Tam -> Ondalık
		"TR_func f(): TR_List { TR_return TR_none }",       // 'yok' her tipe uyar
		"TR_func f(): TR_Int { TR_return }",                // boş dönüş 'yok' sayılır
		"TR_func f(x): TR_Int { TR_return x }",             // bilinmeyen tip
		"TR_func f(x: TR_Int): TR_Int { TR_return x * 2 }", // çıkarılan tip uyumlu
		"TR_func f(): TR_Bool { TR_return 1 < 2 }",         // karşılaştırma
		"TR_func f(): TR_Function { TR_return (x) => x }",
		`TR_func f() { TR_return "x" }`, // dönüş tipi yoksa denetim yok
	} {
		noErrors(t, source)
	}

	// İç fonksiyonlar dış fonksiyonun dönüş tipini devralmaz
	noErrors(t, "TR_func f(): TR_Int {\n  TR_var g = (x) => \"metin\"\n  TR_return 1\n}")
	noErrors(t, "TR_func f(): TR_Int {\n  TR_func g() { TR_return \"metin\" }\n  TR_return 1\n}")
	// İç fonksiyonun kendi dönüş tipi denetlenir; dış fonksiyonunki geri yüklenir
	errs, coded := checkErrs(t, "TR_func f(): TR_Int {\n  TR_func g(): TR_String { TR_return 1 }\n  TR_return \"x\"\n}")
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1003) || !messages.Has(coded[1], messages.H1003) ||
		!strings.Contains(errs[0], `"TR_String"`) || !strings.Contains(errs[1], `"TR_Int"`) {
		t.Errorf("iç ve dış fonksiyon için birer dönüş hatası bekleniyordu, gelen: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Denetimin tüm deyim türlerine ulaşması
// ---------------------------------------------------------------------------

func TestNestedStatementsAreChecked(t *testing.T) {
	def := "TR_func kare(x: TR_Int) { TR_return x * x }\n"
	invalid := `kare("a")`
	tests := map[string]string{
		"eğer":             "TR_if TR_true { " + invalid + " }",
		"değilse eğer":     "TR_if TR_false { } TR_else TR_if TR_true { " + invalid + " }",
		"değilse":          "TR_if TR_false { } TR_else { " + invalid + " }",
		"eğer koşulu":      "TR_if " + invalid + " > 1 { }",
		"iken":             "TR_for TR_false { " + invalid + " }",
		"iken koşulu":      "TR_for " + invalid + " > 1 { }",
		"döngü":            "TR_for i TR_in 1..3 { " + invalid + " }",
		"döngü kaynağı":    "TR_for i TR_in " + invalid + " { }",
		"dene":             "TR_try { " + invalid + " } TR_catch h { }",
		"yakala":           "TR_try { } TR_catch h { " + invalid + " }",
		"eşle gövde":       "TR_match 1 { 1 => { " + invalid + " } }",
		"eşle ifade":       "TR_match 1 { 1 => " + invalid + " }",
		"eşle konusu":      "TR_match " + invalid + " { _ => 1 }",
		"ertele blok":      "TR_defer { " + invalid + " }",
		"ertele çağrı":     "TR_defer " + invalid,
		"sonuç":            "TR_func g() { TR_return " + invalid + " }",
		"fırlat":           "TR_throw " + invalid,
		"eşzamanlı blok":   "TR_async { " + invalid + " }",
		"eşzamanlı çağrı":  "TR_async " + invalid,
		"kesme noktası":    "TR_breakpoint TR_if " + invalid + " > 1",
		"lambda":           "TR_var g = (y) => " + invalid,
		"çoklu tanım":      "TR_var a, b = 1, " + invalid,
		"çoklu atama":      "TR_var a = 1\nTR_var b = 2\na, b = " + invalid + ", 1",
		"atama değeri":     "TR_var a = 1\na = " + invalid,
		"liste dışı ifade": "TR_print(" + invalid + ")",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			singleError(t, def+body, messages.H1004, 0)
		})
	}
}

func TestMultipleErrorsInOrder(t *testing.T) {
	errs, coded := checkErrs(t, `TR_var a: TR_Int = "x"
TR_const b = 1
b = 2
TR_func f(x: TR_String): TR_Int { TR_return x }
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

// Kural (hata ile uyarı ayrıdır): yerleşik bir adı kullanıcı adı olarak tanımlamak hata değil
// uyarıdır: program çalışır, ama o kapsamda yerleşik artık kullanılamaz. Ad dosyanın dilinde bir
// yerleşiğin adıysa uyarılır: Türkçe dosyada girdi, İngilizce dosyada input.
func TestBuiltinShadowingWarning(t *testing.T) {
	for lang, c := range map[string]struct {
		input, str string
		other      string // öbür dilin yerleşik adı: bu dosyada sıradan kullanıcı adıdır
	}{
		"tr": {"girdi", "metin", "input"},
		"en": {"input", "str", "girdi"},
	} {
		source := `TR_var uzunluk = 5
TR_func f(toplam, x) {
    TR_return toplam + x
}
TR_func ` + c.input + `() { TR_return 1 }
TR_for i, ` + c.str + ` TR_in ["a"] { TR_print(` + c.str + `) }
TR_try { TR_throw "x" } TR_catch ekle { TR_print(ekle) }
TR_var a, sırala = 1, 2
TR_struct Nokta {
    x: TR_Int
    TR_func TR_length() { TR_return 1 }
}
TR_var sayaç = uzunluk + sırala
TR_var ` + c.other + ` = 1`
		p := parser.New(lexer.NewBody(source, true))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		chk := New()
		chk.SetFileLanguage(lang)
		if errs := chk.CheckProgram(prog); len(errs) > 0 {
			t.Fatalf("%s: gölgeleme hata değildir: %v", lang, errs)
		}
		// uzunluk, ekle ve sırala yerleşik değil (metotlar): gölgeleme sayılmaz. Uyarı konumuyla
		// (yapı) ve koduyla (H1009) sınanır.
		expected := map[int]string{5: c.input, 6: c.str}
		if lang == "tr" {
			expected[2] = "toplam" // İngilizce dosyada toplam sıradan bir addır (sum)
		}
		got := chk.Warnings()
		if len(got) != len(expected) {
			t.Fatalf("%s: %d uyarı bekleniyordu: %v", lang, len(expected), got)
		}
		for _, w := range got {
			line, col, ok := messages.Pos(w)
			if !ok || col != 0 || !messages.Has(w, messages.H1009) || !strings.Contains(messages.ErrorIn(w, lang), "'"+expected[line]+"'") {
				t.Errorf("%s: uyarı H1009 olmalı, satır %d (%v) için '%s' adını taşımalı: %v", lang, line, ok, expected[line], w)
			}
		}
	}
	// Yapı metotları ve alanları yerleşikleri gölgelemez (x.uzunluk() ile erişilir).
	if u := New().Warnings(); len(u) != 0 {
		t.Errorf("yeni denetleyicide uyarı olmamalı: %v", u)
	}
	// Dosyanın dili yoktur (2.1): yerleşik TR_ ile yazılır, 'toplam' ya da 'input' adlı kullanıcı adı
	// hiçbir yerleşiği gölgelemez; uyarı yalnız kişinin dilinde yazılan satırdadır (SetFileLanguage).
	p := parser.New(lexer.NewBody("TR_var toplam, input, girdi = 1, 2, 3\nTR_print(toplam + input + girdi)", true))
	chk := New()
	chk.CheckProgram(p.ParseProgram())
	if u := chk.Warnings(); len(u) != 0 {
		t.Errorf("dosyada kullanıcı adı yerleşiği gölgelemez: %v", u)
	}
}

func TestEmptyProgram(t *testing.T) {
	noErrors(t, "")
	noErrors(t, "# yalnızca yorum")
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
		{"tanımdan önce çağrı (öne alma yok)", "kare(\"a\")\nTR_func kare(x: TR_Int) { }"},
		{"değişkene atanan fonksiyonun parametre tipleri unutulur", "TR_var kare = TR_func(x: TR_Int) { TR_return x }\nkare(\"a\")"},
		{"lambda parametre tipleri", "TR_var f = (x: TR_Int) => x\nf(\"a\")"},
		{"eksik argüman", "TR_func f(a: TR_Int, b: TR_Int) { }\nf(1)"},
		{"fazla argüman", "TR_func f(a: TR_Int) { }\nf(1, 2, 3)"},
		{"isimli argüman tipi", "TR_func f(a: TR_Int) { }\nf(a: \"x\")"},
		{"dönüş tipi olan fonksiyonda eksik 'sonuç'", "TR_func f(): TR_Int { }"},
		{"sabitin yeniden tanımlanması", "TR_const x = 1\nTR_var x = 2\nx = 3"},
		{"sabitin aynı adlı fonksiyonla ezilmesi", "TR_const f = 1\nTR_func f() { }\nf = TR_func() { }"},
		{"TR_const listenin elemanını değiştirme", "TR_const l = [1, 2]\nl[0] = 5"},
		{"TR_const sözlüğün alanını değiştirme", "TR_const p = {\"x\": 1}\np.x = 5"},
		{"çoklu atamada tip denetimi yok", "TR_var x: TR_Int = 1\nTR_var y = 2\nx, y = \"a\", 2"},
		{"çoklu tanımda değişkenler tipsizdir", "TR_var a, b = 1, 2\na = \"x\""},
		{"yapı alan tipleri", "TR_struct Nokta {\n  x: TR_Int\n}\nTR_var p = Nokta(\"a\")"},
		{"döngü değişkenleri tipsizdir", "TR_for x TR_in [1, 2] { TR_var s: TR_String = x }"},
		{"uyumsuz işlenenler", "TR_var a = \"x\" - 1\nTR_var b = TR_true + 1"},
		{"koşul tipi", "TR_if 5 { }\nTR_for \"x\" { }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noErrors(t, tt.source)
		})
	}

	// Yakalanan hata değişkeni açıklamasızdır: başka tipte değer alabilir.
	noErrors(t, "TR_try { } TR_catch h {\n  h = 5\n}")
}

// Kural (3.14): tip açıklamasında kabul edilen yazımlar sözlükten gelir (type); program içeride
// kimliklerle çalıştığı için her tipin tek kabul edilen yazımı kimliğidir; büyük/küçük harfi
// farklı biçim kabul edilen bir yazım değildir.
func TestTypeSpellingsUnique(t *testing.T) {
	if len(typeSpellings) != len(annotationTypes) {
		t.Errorf("%d yazım bekleniyordu, %d var: %v", len(annotationTypes), len(typeSpellings), typeSpellings)
	}
	for _, k := range annotationTypes {
		id := k.String()
		if got, ok := typeSpellings[id]; !ok || got != id {
			t.Errorf("typeSpellings[%q] = %q, %v; kendisi bekleniyordu", id, got, ok)
		}
		if _, ok := typeSpellings[strings.ToLower(id)]; ok {
			t.Errorf("%q kabul edilen bir yazım olmamalı (3.14)", strings.ToLower(id))
		}
	}
	for _, c := range []struct{ in, want string }{{"", typeAny}, {"TAM", typeInt}, {typeAny, typeAny}, {"herhangi", "herhangi"}, {"Nokta", "Nokta"}} {
		if got := normalizeTypeName(c.in); got != c.want {
			t.Errorf("normalizeTypeName(%q) = %q, %q bekleniyordu", c.in, got, c.want)
		}
	}
}

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
	singleErrorIn(t, "tr", "TR_kw_var a: ondalik = 1.5", messages.H1007, 1, "'ondalik'", "'Ondalık'")
	singleErrorIn(t, "tr", "TR_kw_var a: TR_type_Bool = TR_kw_true\nTR_kw_var b: tensor = TR_kw_none", messages.H1007, 2, "'tensor'", "'Tensör'")
	singleErrorIn(t, "tr", "TR_kw_func f(x: sozluk) { TR_kw_return x }", messages.H1007, 1, "'sozluk'", "'Sözlük'")
	singleErrorIn(t, "tr", "TR_kw_func f(x) -> mantik { TR_kw_return TR_kw_true }", messages.H1007, 1, "'mantik'", "'Mantık'")
	for name, canonical := range map[string]string{"int": "Tam", "float": "Ondalık", "str": "Metin", "string": "Metin",
		"bool": "Mantık", "null": "Yok", "list": "Liste", "dict": "Sözlük", "map": "Sözlük", "matrix": "Tensör"} {
		singleErrorIn(t, "tr", "TR_kw_var a: "+name, messages.H1007, 1, "'"+name+"'", "'"+canonical+"'")
	}
	for name, canonical := range map[string]string{"Ondalık": "Float", "Tam": "Int", "Sözlük": "Dict"} {
		singleErrorIn(t, "en", "TR_kw_var a: "+name, messages.H1007, 1, "'"+name+"'", "'"+canonical+"'")
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
			singleErrorIn(t, lang, "TR_kw_var a: "+name, messages.H1006, 1, "'"+name+"'", "'"+canonical+"'")
		}
	}
	singleErrorIn(t, "tr", "TR_kw_func f(x: liste) { TR_kw_return x }", messages.H1006, 1, "'liste'", "'Liste'")
	singleErrorIn(t, "tr", "TR_kw_func f(x) -> sözlük { TR_kw_return {} }", messages.H1006, 1, "'sözlük'", "'Sözlük'")
	singleErrorIn(t, "tr", "TR_kw_var a: tam = 1\nTR_kw_var b: TR_type_Int = a", messages.H1006, 1, "'tam'")
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
		"TR_kw_var a: TR_type_Int = 5",
		"TR_kw_var a: TR_type_Float = 5",
		"TR_kw_var a: TR_type_Float = 2.5",
		"TR_kw_var a: TR_type_Float = 1 + 2.5",
		`TR_kw_var a: TR_type_String = "merhaba"`,
		`TR_kw_var x = 1
		 TR_kw_var a: TR_type_String = "x = {x}"`,
		`TR_kw_var a: TR_type_String = "a" + 1`,
		"TR_kw_var a: TR_type_Bool = TR_kw_true",
		"TR_kw_var a: TR_type_Bool = 1 < 2",
		"TR_kw_var a: TR_type_Bool = !5",
		"TR_kw_var a: TR_type_Bool = TR_kw_true TR_kw_and TR_kw_false",
		"TR_kw_var a: TR_type_List = [1, 2]",
		"TR_kw_var a: TR_type_Range = 1..=5",
		"TR_kw_var a: TR_type_Float = 7 / 2",
		"TR_kw_var a: TR_type_Float = 2 ** -1",
		"TR_kw_var a: TR_type_Int = 7 % 2",
		"TR_kw_var a: TR_type_Int = (2 ** 3) * 4",
		"TR_kw_var a = 7 / 2",
		"TR_kw_var a: TR_type_List = [x * 2 TR_kw_for x TR_kw_in 1..3]",
		`TR_kw_var a: TR_type_Dict = {"a": 1}`,
		"TR_kw_var m1 = TR_fn_eye(2)\nTR_kw_var m2 = m1\nTR_kw_var a: TR_type_Tensor = m1 @ m2",
		"TR_kw_var a: TR_type_Int = TR_kw_none",
		"TR_kw_var a: TR_type_Int", // değer verilmezse 'yok' atanır
		"TR_kw_var bilinmeyen_tip = TR_fn_input()\nTR_kw_var a: TR_type_Int = bilinmeyen_tip",
		`TR_kw_var a: TR_type_Int = TR_fn_print("x")`,
		"TR_kw_var a: TR_type_Int = 5 |> TR_fn_int",
		"TR_kw_var a: TR_type_Int = (5 ?? 6)",
		`TR_kw_var a: TR_type_String = TR_kw_none ?? "varsayılan"`,
		"TR_kw_var a: TR_type_Int = -5",
		"TR_kw_var a: TR_type_Int = 2 ** 3",
		"TR_kw_var a: TR_type_Function = TR_kw_func(x) { TR_kw_return x }",
		"TR_kw_const a: TR_type_Int = 1",
	} {
		noErrors(t, source)
	}
}

func TestTypedVarIncompatible(t *testing.T) {
	tests := []struct {
		source, expected, actual string
	}{
		{`TR_kw_var yaş: TR_type_Int = "yirmi"`, "Tam", "Metin"},
		{"TR_kw_var a: TR_type_Int = 2.5", "Tam", "Ondalık"},
		{"TR_kw_var a: TR_type_Int = 1 + 2.5", "Tam", "Ondalık"},
		{"TR_kw_var a: TR_type_String = 5", "Metin", "Tam"},
		{"TR_kw_var a: TR_type_Bool = 1", "Mantık", "Tam"},
		{"TR_kw_var a: TR_type_Int = 1 == 1", "Tam", "Mantık"},
		{"TR_kw_var a: TR_type_Int = !TR_kw_true", "Tam", "Mantık"},
		{`TR_kw_var a: TR_type_List = {"a": 1}`, "Liste", "Sözlük"},
		{"TR_kw_var a: TR_type_Dict = [1]", "Sözlük", "Liste"},
		{"TR_kw_var a: TR_type_Int = 1..5", "Tam", "Aralık"},
		{"TR_kw_var a: TR_type_List = 1..5", "Liste", "Aralık"},
		{"TR_kw_var a: TR_type_Int = 7 / 2", "Tam", "Tam ya da Ondalık"},
		{"TR_kw_var a: TR_type_Int = 6 / 3", "Tam", "Tam ya da Ondalık"},
		{"TR_kw_var a: TR_type_Int = 2 ** -1", "Tam", "Tam ya da Ondalık"},
		{"TR_kw_var a: TR_type_String = 7 / 2", "Metin", "Tam ya da Ondalık"},
		{"TR_kw_var a: TR_type_String = TR_kw_func() { }", "Metin", "Fonksiyon"},
		{"TR_kw_var a: Nokta = 5", "Nokta", "Tam"},
		{"TR_kw_const a: TR_type_String = TR_kw_true", "Metin", "Mantık"},
		{`TR_kw_var a: TR_type_String = 5 ?? "x"`, "Metin", "Tam"},
	}
	for _, tt := range tests {
		singleError(t, tt.source, messages.H1001, 1, `"`+tt.expected+`"`, `"`+tt.actual+`"`)
	}
}

func TestTypeErrorLineAndVarName(t *testing.T) {
	singleError(t, "TR_kw_var a = 1\n\nTR_kw_var öğrenci_sayısı: TR_type_Int = \"çok\"",
		messages.H1001, 3, `"öğrenci_sayısı" değişkeni`)
}

// ---------------------------------------------------------------------------
// Atamalar
// ---------------------------------------------------------------------------

func TestAssignmentTypeCheck(t *testing.T) {
	// Belirtilen tip korunur
	singleError(t, "TR_kw_var a: TR_type_Int = 1\na = \"x\"", messages.H1005, 2, `"a" değişkeni (Tam)`, `"Metin"`)
	noErrors(t, "TR_kw_var a: TR_type_Float = 1\na = 2.5\na = 3")

	// Dil dinamiktir: tip açıklaması olmayan değişkenin ilk değerden çıkarılan tipi kalıcı
	// bir bildirim değildir; başka tipte değer atanabilir. Sabitlerin tipi kalır.
	noErrors(t, "TR_kw_var a = 5\na = \"beş\"")
	noErrors(t, "x := TR_kw_true\nx = 1")
	noErrors(t, "TR_kw_var kayıp = 0\nTR_kw_for i TR_kw_in 1..3 { kayıp += 0.5 }")
	singleError(t, "TR_kw_const S = 5\nTR_kw_var m: TR_type_String = S", messages.H1001, 2, `"Metin"`, `"Tam"`)

	// 'yok' veya bilinmeyen tipte başlayan değişkenler serbestçe atanabilir
	noErrors(t, "TR_kw_var a = TR_kw_none\na = 5\na = \"x\"")
	noErrors(t, "TR_kw_var a = TR_fn_input()\na = 5\na = \"x\"")
	noErrors(t, "TR_kw_var a\na = [1]")

	// Tanımlanmamış bir hedefe atama tanımsız ad hatasıdır (5.4, 5.14)
	singleError(t, "tanımsız = 5", messages.H1010, 1, "'tanımsız'")
}

func TestCompoundAssignment(t *testing.T) {
	// 's += 1' ile 's = s + 1' aynı şekilde değerlendirilmeli (Metin + Tam = Metin)
	noErrors(t, "TR_kw_var s = \"a\"\ns += 1")
	noErrors(t, "TR_kw_var s = \"a\"\ns = s + 1")
	noErrors(t, "TR_kw_var x = 10\nx += 1\nx -= 2\nx *= 3")
	noErrors(t, "TR_kw_var x = 1.5\nx += 1")
	noErrors(t, "TR_kw_var x: TR_type_Float = 1\nx *= 2.5")

	// 'x = x + 0.5' gibi, Tam açıklamalı bir değişkene Ondalık sonuç atanamaz; açıklamasız
	// değişken (biriktirici) serbesttir.
	singleError(t, "TR_kw_var x: TR_type_Int = 1\nx += 0.5", messages.H1005, 2, `"x" değişkeni (Tam)`, `"Ondalık"`)
	singleError(t, "TR_kw_var x: TR_type_Int = 1\nx = x + 0.5", messages.H1005, 2, `"x" değişkeni (Tam)`, `"Ondalık"`)
	noErrors(t, "TR_kw_var x = 1\nx += 0.5")

	// '/' Tam/Tam'da tam bölünmezse Ondalık verir (10 / 4 = 2.5): sonuç "Tam ya da Ondalık"tır.
	noErrors(t, "TR_kw_var x = 10\nx /= 4")
	singleError(t, "TR_kw_var y: TR_type_Int = 10 / 4", messages.H1001, 1, `"Tam ya da Ondalık"`, "tam(...)")
	singleError(t, "TR_kw_var y: TR_type_Int = 10\ny /= 4", messages.H1005, 2, `"Tam ya da Ondalık"`)
}

func TestConstCannotChange(t *testing.T) {
	singleError(t, "TR_kw_const PI = 3.14\nPI = 3.0", messages.H1002, 2, `"PI"`)
	singleError(t, "TR_kw_const SAYAÇ = 1\nSAYAÇ += 1", messages.H1002, 2, `"SAYAÇ"`)
	singleError(t, "TR_kw_const a: TR_type_String = \"x\"\na = \"y\"", messages.H1002, 2)

	// Çoklu tanım ve çoklu atama
	singleError(t, "TR_kw_const a, b = 1, 2\nb = 3", messages.H1002, 2, `"b"`)
	singleError(t, "TR_kw_const a = 1\nTR_kw_var b = 2\nb, a = a, b", messages.H1002, 3, `"a"`)

	// İç kapsamlardan dış sabite atama da yakalanır
	singleError(t, "TR_kw_const a = 1\nTR_kw_if TR_kw_true {\n  a = 2\n}", messages.H1002, 3)
	singleError(t, "TR_kw_const a = 1\nTR_kw_func f() {\n  a = 2\n}", messages.H1002, 3)
	singleError(t, "TR_kw_const a = 1\nTR_kw_for i TR_kw_in 1..3 {\n  a = i\n}", messages.H1002, 3)

	// Hem sabit hem tip hatası aynı anda bildirilir
	errs, coded := checkErrs(t, "TR_kw_const a = 1\na = \"x\"")
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1002) || !messages.Has(coded[1], messages.H1005) {
		t.Errorf("önce sabit, sonra tip hatası bekleniyordu, gelen: %v", errs)
	}
}

// İç kapsamda aynı adla tanımlanan değişken/parametre dış sabiti gölgeler.
func TestConstShadowing(t *testing.T) {
	noErrors(t, "TR_kw_const x = 1\nTR_kw_if TR_kw_true {\n  TR_kw_var x = 2\n  x = 3\n}")
	noErrors(t, "TR_kw_const x = 1\nTR_kw_func f(x) {\n  x = 5\n}")
	noErrors(t, "TR_kw_const x = 1\nTR_kw_for x TR_kw_in [1, 2] {\n  x = 5\n}")
	noErrors(t, "TR_kw_const i = 1\nTR_kw_for i, x TR_kw_in [1, 2] {\n  i = 5\n}")
	noErrors(t, "TR_kw_const e = 1\nTR_kw_try {\n} TR_kw_catch e {\n  e = \"x\"\n}")
	// Blok bitince dış sabit yeniden geçerlidir
	singleError(t, "TR_kw_const x = 1\nTR_kw_if TR_kw_true {\n  TR_kw_var x = 2\n}\nx = 3", messages.H1002, 5)
}

// ---------------------------------------------------------------------------
// Fonksiyonlar
// ---------------------------------------------------------------------------

func TestFunctionParamTypes(t *testing.T) {
	def := "TR_kw_func kare(x: TR_type_Int) { TR_kw_return x * x }\n"
	singleError(t, def+`kare("üç")`, messages.H1004, 2, `"kare"`, "1. parametresi", `"Tam"`, `"Metin"`)
	noErrors(t, def+"kare(3)")
	noErrors(t, def+"kare(TR_fn_input())")
	noErrors(t, def+"kare(TR_kw_none)")

	two := "TR_kw_func böl(a: TR_type_Float, b: TR_type_Float) -> TR_type_Float { TR_kw_return a / b }\n"
	noErrors(t, two+"böl(1, 2)")
	singleError(t, two+`böl(1, "iki")`, messages.H1004, 0, "2. parametresi", `"Ondalık"`, `"Metin"`)

	// Tipsiz parametreler her değeri kabul eder
	noErrors(t, "TR_kw_func f(a, b: TR_type_Int) { }\nf(\"x\", 2)")
	singleError(t, "TR_kw_func f(a, b: TR_type_Int) { }\nf(\"x\", \"y\")", messages.H1004, 0, "2. parametresi")

	// Parametreler fonksiyon gövdesinde bildirilen tipleriyle bilinir
	singleError(t, "TR_kw_func f(a: TR_type_String) {\n  TR_kw_var b: TR_type_Int = a\n}", messages.H1001, 2, `"b"`, `"Metin"`)

	// Özyinelemeli çağrılar da denetlenir (ad, gövdeden önce tanımlanır)
	singleError(t, "TR_kw_func f(n: TR_type_Int) {\n  TR_kw_return f(\"x\")\n}", messages.H1004, 2)

	// İç içe çağrılar denetlenir
	singleError(t, def+`TR_fn_print(kare("a"))`, messages.H1004, 2)
	singleError(t, def+"kare(kare(\"a\"))", messages.H1004, 2)
	// Fazladan argümanlar ve isimli argümanlar içindeki çağrılar da denetlenir
	singleError(t, def+"kare(1, kare(\"a\"))", messages.H1004, 2)
	singleError(t, def+`TR_fn_print(1, ayraç: kare("a"))`, messages.H1004, 2)
}

func TestFunctionReturnTypeInference(t *testing.T) {
	def := "TR_kw_func topla(a: TR_type_Int, b: TR_type_Int) -> TR_type_Int { TR_kw_return a + b }\n"
	singleError(t, def+"TR_kw_var s: TR_type_String = topla(1, 2)", messages.H1001, 2, `"Metin"`, `"Tam"`)
	noErrors(t, def+"TR_kw_var s: TR_type_Float = topla(1, 2)")

	// Parametresiz fonksiyonların dönüş tipi de kullanılır
	singleError(t, "TR_kw_func ad(): TR_type_String { TR_kw_return \"Ali\" }\nTR_kw_var n: TR_type_Int = ad()", messages.H1001, 2, `"Tam"`, `"Metin"`)

	// Dönüş tipi belirtilmemişse sonuç bilinmez (Herhangi)
	noErrors(t, "TR_kw_func f(a: TR_type_Int) { TR_kw_return a }\nTR_kw_var s: TR_type_String = f(1)")
	noErrors(t, "TR_kw_func f() { TR_kw_return 1 }\nTR_kw_var s: TR_type_String = f()")
}

func TestReturnTypeCheck(t *testing.T) {
	singleError(t, "TR_kw_func f(): TR_type_Int {\n  TR_kw_return \"x\"\n}", messages.H1003, 2, `"Tam"`, `"Metin"`)
	singleError(t, "TR_kw_func f() -> TR_type_String {\n  TR_kw_return 1\n}", messages.H1003, 2)
	singleError(t, "TR_kw_func f(): TR_type_Int { TR_kw_return 2.5 }", messages.H1003, 0, `"Ondalık"`)
	// Tam / Tam Tam olmayabilir (7 / 2 = 3.5): ': Tam' bildiren fonksiyon bunu döndüremez
	singleError(t, "TR_kw_func ort(a: TR_type_Int, b: TR_type_Int): TR_type_Int { TR_kw_return a / b }", messages.H1003, 0, `"Tam ya da Ondalık"`)
	noErrors(t, "TR_kw_func ort(a: TR_type_Int, b: TR_type_Int): TR_type_Float { TR_kw_return a / b }")

	// İç bloklardaki dönüşler de denetlenir
	singleError(t, "TR_kw_func f(x): TR_type_Int {\n  TR_kw_if x {\n    TR_kw_return \"a\"\n  }\n  TR_kw_return 1\n}", messages.H1003, 3)
	singleError(t, "TR_kw_func f(x): TR_type_Int {\n  TR_kw_for i TR_kw_in x {\n    TR_kw_return TR_kw_true\n  }\n}", messages.H1003, 3)

	for _, source := range []string{
		"TR_kw_func f(): TR_type_Float { TR_kw_return 1 }",                 // Tam -> Ondalık
		"TR_kw_func f(): TR_type_List { TR_kw_return TR_kw_none }",         // 'yok' her tipe uyar
		"TR_kw_func f(): TR_type_Int { TR_kw_return }",                     // boş dönüş 'yok' sayılır
		"TR_kw_func f(x): TR_type_Int { TR_kw_return x }",                  // bilinmeyen tip
		"TR_kw_func f(x: TR_type_Int): TR_type_Int { TR_kw_return x * 2 }", // çıkarılan tip uyumlu
		"TR_kw_func f(): TR_type_Bool { TR_kw_return 1 < 2 }",              // karşılaştırma
		"TR_kw_func f(): TR_type_Function { TR_kw_return (x) => x }",
		`TR_kw_func f() { TR_kw_return "x" }`, // dönüş tipi yoksa denetim yok
		`TR_kw_return "x"`,                    // fonksiyon dışındaki dönüş denetlenmez
	} {
		noErrors(t, source)
	}

	// İç fonksiyonlar dış fonksiyonun dönüş tipini devralmaz
	noErrors(t, "TR_kw_func f(): TR_type_Int {\n  TR_kw_var g = (x) => \"metin\"\n  TR_kw_return 1\n}")
	noErrors(t, "TR_kw_func f(): TR_type_Int {\n  TR_kw_func g() { TR_kw_return \"metin\" }\n  TR_kw_return 1\n}")
	// İç fonksiyonun kendi dönüş tipi denetlenir; dış fonksiyonunki geri yüklenir
	errs, coded := checkErrs(t, "TR_kw_func f(): TR_type_Int {\n  TR_kw_func g(): TR_type_String { TR_kw_return 1 }\n  TR_kw_return \"x\"\n}")
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1003) || !messages.Has(coded[1], messages.H1003) ||
		!strings.Contains(errs[0], `"Metin"`) || !strings.Contains(errs[1], `"Tam"`) {
		t.Errorf("iç ve dış fonksiyon için birer dönüş hatası bekleniyordu, gelen: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Denetimin tüm deyim türlerine ulaşması
// ---------------------------------------------------------------------------

func TestNestedStatementsAreChecked(t *testing.T) {
	def := "TR_kw_func kare(x: TR_type_Int) { TR_kw_return x * x }\n"
	invalid := `kare("a")`
	tests := map[string]string{
		"eğer":             "TR_kw_if TR_kw_true { " + invalid + " }",
		"değilse eğer":     "TR_kw_if TR_kw_false { } TR_kw_else TR_kw_if TR_kw_true { " + invalid + " }",
		"değilse":          "TR_kw_if TR_kw_false { } TR_kw_else { " + invalid + " }",
		"eğer koşulu":      "TR_kw_if " + invalid + " > 1 { }",
		"iken":             "TR_kw_while TR_kw_false { " + invalid + " }",
		"iken koşulu":      "TR_kw_while " + invalid + " > 1 { }",
		"döngü":            "TR_kw_for i TR_kw_in 1..3 { " + invalid + " }",
		"döngü kaynağı":    "TR_kw_for i TR_kw_in " + invalid + " { }",
		"dene":             "TR_kw_try { " + invalid + " } TR_kw_catch h { }",
		"yakala":           "TR_kw_try { } TR_kw_catch h { " + invalid + " }",
		"eşle gövde":       "TR_kw_match 1 { 1 => { " + invalid + " } }",
		"eşle ifade":       "TR_kw_match 1 { 1 => " + invalid + " }",
		"eşle konusu":      "TR_kw_match " + invalid + " { TR_kw_default => 1 }",
		"ertele blok":      "TR_kw_defer { " + invalid + " }",
		"ertele çağrı":     "TR_kw_defer " + invalid,
		"sonuç":            "TR_kw_func g() { TR_kw_return " + invalid + " }",
		"fırlat":           "TR_kw_throw " + invalid,
		"eşzamanlı blok":   "TR_kw_async { " + invalid + " }",
		"eşzamanlı çağrı":  "TR_kw_async " + invalid,
		"kesme noktası":    "TR_kw_breakpoint TR_kw_if " + invalid + " > 1",
		"lambda":           "TR_kw_var g = (y) => " + invalid,
		"çoklu tanım":      "TR_kw_var a, b = 1, " + invalid,
		"çoklu atama":      "TR_kw_var a = 1\nTR_kw_var b = 2\na, b = " + invalid + ", 1",
		"atama değeri":     "TR_kw_var a = 1\na = " + invalid,
		"liste dışı ifade": "TR_fn_print(" + invalid + ")",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			singleError(t, def+body, messages.H1004, 0)
		})
	}
}

func TestMultipleErrorsInOrder(t *testing.T) {
	errs, coded := checkErrs(t, `TR_kw_var a: TR_type_Int = "x"
TR_kw_const b = 1
b = 2
TR_kw_func f(x: TR_type_String): TR_type_Int { TR_kw_return x }
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
		source := `TR_kw_var uzunluk = 5
TR_kw_func f(toplam, x) {
    TR_kw_return toplam + x
}
TR_kw_func ` + c.input + `() { TR_kw_return 1 }
TR_kw_for i, ` + c.str + ` TR_kw_in ["a"] { TR_fn_print(` + c.str + `) }
TR_kw_try { TR_kw_throw "x" } TR_kw_catch ekle { TR_fn_print(ekle) }
TR_kw_var a, sırala = 1, 2
TR_kw_struct Nokta {
    x: TR_type_Int
    TR_kw_func TR_mem_length() { TR_kw_return 1 }
}
TR_kw_var sayaç = uzunluk + sırala
TR_kw_var ` + c.other + ` = 1`
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
		{"tanımdan önce çağrı (öne alma yok)", "kare(\"a\")\nTR_kw_func kare(x: TR_type_Int) { }"},
		{"değişkene atanan fonksiyonun parametre tipleri unutulur", "TR_kw_var kare = TR_kw_func(x: TR_type_Int) { TR_kw_return x }\nkare(\"a\")"},
		{"lambda parametre tipleri", "TR_kw_var f = (x: TR_type_Int) => x\nf(\"a\")"},
		{"eksik argüman", "TR_kw_func f(a: TR_type_Int, b: TR_type_Int) { }\nf(1)"},
		{"fazla argüman", "TR_kw_func f(a: TR_type_Int) { }\nf(1, 2, 3)"},
		{"isimli argüman tipi", "TR_kw_func f(a: TR_type_Int) { }\nf(a: \"x\")"},
		{"dönüş tipi olan fonksiyonda eksik 'sonuç'", "TR_kw_func f(): TR_type_Int { }"},
		{"sabitin yeniden tanımlanması", "TR_kw_const x = 1\nTR_kw_var x = 2\nx = 3"},
		{"sabitin aynı adlı fonksiyonla ezilmesi", "TR_kw_const f = 1\nTR_kw_func f() { }\nf = TR_kw_func() { }"},
		{"sabit listenin elemanını değiştirme", "TR_kw_const l = [1, 2]\nl[0] = 5"},
		{"sabit sözlüğün alanını değiştirme", "TR_kw_const p = {\"x\": 1}\np.x = 5"},
		{"çoklu atamada tip denetimi yok", "TR_kw_var x: TR_type_Int = 1\nTR_kw_var y = 2\nx, y = \"a\", 2"},
		{"çoklu tanımda değişkenler tipsizdir", "TR_kw_var a, b = 1, 2\na = \"x\""},
		{"yapı alan tipleri", "TR_kw_struct Nokta {\n  x: TR_type_Int\n}\nTR_kw_var p = Nokta(\"a\")"},
		{"döngü değişkenleri tipsizdir", "TR_kw_for x TR_kw_in [1, 2] { TR_kw_var s: TR_type_String = x }"},
		{"uyumsuz işlenenler", "TR_kw_var a = \"x\" - 1\nTR_kw_var b = TR_kw_true + 1"},
		{"koşul tipi", "TR_kw_if 5 { }\nTR_kw_while \"x\" { }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noErrors(t, tt.source)
		})
	}

	// Yakalanan hata değişkeni açıklamasızdır: başka tipte değer alabilir.
	noErrors(t, "TR_kw_try { } TR_kw_catch h {\n  h = 5\n}")
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

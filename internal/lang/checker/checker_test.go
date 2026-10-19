package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// check kaynağı ayrıştırır (sözdizimi hatası testi düşürür) ve tip denetimi hatalarını döndürür.
func check(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	errs := c.CheckProgram(prog)
	if len(errs) != len(c.Errors()) {
		t.Errorf("CheckProgram ve Errors() farklı sayıda hata döndürdü: %v / %v", errs, c.Errors())
	}
	return errs
}

func noErrors(t *testing.T, source string) {
	t.Helper()
	if errs := check(t, source); len(errs) > 0 {
		t.Errorf("hata beklenmiyordu, gelen: %v\nkaynak:\n%s", errs, source)
	}
}

// singleError kaynağın tam olarak bir hata üretmesini ve hatanın tüm parçaları içermesini bekler.
func singleError(t *testing.T, source string, parts ...string) {
	t.Helper()
	errs := check(t, source)
	if len(errs) != 1 {
		t.Errorf("tek bir hata bekleniyordu, gelen %d: %v\nkaynak:\n%s", len(errs), errs, source)
		return
	}
	for _, p := range parts {
		if !strings.Contains(errs[0], p) {
			t.Errorf("hata %q parçasını içermeli, gelen: %q\nkaynak:\n%s", p, errs[0], source)
		}
	}
}

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
	singleError(t, `değişken a: ondalik = 1.5`, "Satır 1: 'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?")
	singleError(t, `değişken a: Mantık = doğru
değişken b: tensor = yok`, "Satır 2: 'tensor' TR'de bir tip adı değil; 'Tensör' yazmak mı istediniz?")
	singleError(t, `fonksiyon f(x: sozluk) { sonuç x }`, "'sozluk' TR'de bir tip adı değil; 'Sözlük'")
	singleError(t, `fonksiyon f(x) -> mantik { sonuç doğru }`, "'mantik' TR'de bir tip adı değil; 'Mantık'")
	// Kaldırılan eş yazımlar da hata verir ve asıl adı önerir.
	for name, canonical := range map[string]string{"tamsayı": "Tam", "int": "Tam", "sayı": "Ondalık", "float": "Ondalık",
		"yazı": "Metin", "str": "Metin", "string": "Metin", "mantıksal": "Mantık", "bool": "Mantık", "boş": "Yok",
		"null": "Yok", "dizi": "Liste", "list": "Liste", "harita": "Sözlük", "dict": "Sözlük", "map": "Sözlük",
		"Matris": "Tensör", "matrix": "Tensör", "TAMSAYI": "Tam"} {
		singleError(t, "değişken a: "+name, "'"+name+"' TR'de bir tip adı değil; '"+canonical+"' yazmak mı istediniz?")
	}
}

// Tip adının tek yazımı vardır (3.14): yalnızca büyük/küçük harfi farklı bir yazım da hata
// verir ve asıl yazımı söyler (tek hata: aynı satırda tip uyuşmazlığı bildirilmez).
func TestTypeNameCaseSpelling(t *testing.T) {
	for name, canonical := range map[string]string{"tam": "Tam", "TAM": "Tam", "ondalık": "Ondalık", "ONDALIK": "Ondalık",
		"METİN": "Metin", "mantık": "Mantık", "aralık": "Aralık", "tensör": "Tensör", "YOK": "Yok",
		"yerleşikFonksiyon": "YerleşikFonksiyon", "tip": "Tip"} {
		singleError(t, "değişken a: "+name, "Satır 1: '"+name+"' TR'de bir tip adı değil; tip adının tek yazımı vardır: '"+canonical+"' yazın")
	}
	singleError(t, `fonksiyon f(x: liste) { sonuç x }`, "'liste' TR'de bir tip adı değil", "'Liste' yazın")
	singleError(t, `fonksiyon f(x) -> sözlük { sonuç {} }`, "'sözlük' TR'de bir tip adı değil", "'Sözlük' yazın")
	singleError(t, "değişken a: tam = 1\ndeğişken b: Tam = a", "Satır 1: 'tam'")
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
		singleError(t, tt.source, "Satır 1:", "Tip uyuşmazlığı", `"`+tt.expected+`"`, `"`+tt.actual+`"`)
	}
}

func TestTypeErrorLineAndVarName(t *testing.T) {
	singleError(t, "değişken a = 1\n\ndeğişken öğrenci_sayısı: Tam = \"çok\"",
		"Satır 3:", `"öğrenci_sayısı" değişkeni`)
}

// ---------------------------------------------------------------------------
// Atamalar
// ---------------------------------------------------------------------------

func TestAssignmentTypeCheck(t *testing.T) {
	// Belirtilen tip korunur
	singleError(t, "değişken a: Tam = 1\na = \"x\"", "Satır 2:", "Tip uyuşmazlığı", `"a" değişkeni (Tam)`, `"Metin"`)
	noErrors(t, "değişken a: Ondalık = 1\na = 2.5\na = 3")

	// Dil dinamiktir: tip açıklaması olmayan değişkenin ilk değerden çıkarılan tipi kalıcı
	// bir bildirim değildir; başka tipte değer atanabilir. Sabitlerin tipi kalır.
	noErrors(t, "değişken a = 5\na = \"beş\"")
	noErrors(t, "x := doğru\nx = 1")
	noErrors(t, "değişken kayıp = 0\ndöngü i içinde 1..3 { kayıp += 0.5 }")
	singleError(t, "sabit S = 5\ndeğişken m: Metin = S", "Satır 2:", `"Metin"`, `"Tam"`)

	// 'yok' veya bilinmeyen tipte başlayan değişkenler serbestçe atanabilir
	noErrors(t, "değişken a = yok\na = 5\na = \"x\"")
	noErrors(t, "değişken a = girdi()\na = 5\na = \"x\"")
	noErrors(t, "değişken a\na = [1]")

	// Tanımlanmamış bir hedefe atama tanımsız ad hatasıdır (5.4, 5.14)
	singleError(t, "tanımsız = 5", "Tanımsız ad — 'tanımsız'")
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
	singleError(t, "değişken x: Tam = 1\nx += 0.5", "Satır 2:", `"x" değişkeni (Tam)`, `"Ondalık"`)
	singleError(t, "değişken x: Tam = 1\nx = x + 0.5", "Satır 2:", `"x" değişkeni (Tam)`, `"Ondalık"`)
	noErrors(t, "değişken x = 1\nx += 0.5")

	// '/' Tam/Tam'da tam bölünmezse Ondalık verir (10 / 4 = 2.5): sonuç "Tam ya da Ondalık"tır.
	noErrors(t, "değişken x = 10\nx /= 4")
	singleError(t, "değişken y: Tam = 10 / 4", "Satır 1:", `"Tam ya da Ondalık"`, "tam(...)")
	singleError(t, "değişken y: Tam = 10\ny /= 4", "Satır 2:", `"Tam ya da Ondalık"`)
}

func TestConstCannotChange(t *testing.T) {
	singleError(t, "sabit PI = 3.14\nPI = 3.0", "Satır 2:", "Sabit hatası", `"PI"`)
	singleError(t, "sabit SAYAÇ = 1\nSAYAÇ += 1", "Satır 2:", "Sabit hatası", `"SAYAÇ"`)
	singleError(t, "sabit a: Metin = \"x\"\na = \"y\"", "Satır 2:", "Sabit hatası")

	// Çoklu tanım ve çoklu atama
	singleError(t, "sabit a, b = 1, 2\nb = 3", "Satır 2:", "Sabit hatası", `"b"`)
	singleError(t, "sabit a = 1\ndeğişken b = 2\nb, a = a, b", "Satır 3:", "Sabit hatası", `"a"`)

	// İç kapsamlardan dış sabite atama da yakalanır
	singleError(t, "sabit a = 1\neğer doğru {\n  a = 2\n}", "Satır 3:", "Sabit hatası")
	singleError(t, "sabit a = 1\nfonksiyon f() {\n  a = 2\n}", "Satır 3:", "Sabit hatası")
	singleError(t, "sabit a = 1\ndöngü i içinde 1..3 {\n  a = i\n}", "Satır 3:", "Sabit hatası")

	// Hem sabit hem tip hatası aynı anda bildirilir
	errs := check(t, "sabit a = 1\na = \"x\"")
	if len(errs) != 2 || !strings.Contains(errs[0], "Sabit hatası") || !strings.Contains(errs[1], "Tip uyuşmazlığı") {
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
	singleError(t, "sabit x = 1\neğer doğru {\n  değişken x = 2\n}\nx = 3", "Satır 5:", "Sabit hatası")
}

// ---------------------------------------------------------------------------
// Fonksiyonlar
// ---------------------------------------------------------------------------

func TestFunctionParamTypes(t *testing.T) {
	def := "fonksiyon kare(x: Tam) { sonuç x * x }\n"
	singleError(t, def+`kare("üç")`, "Satır 2:", "Fonksiyon çağrısı tip hatası", `"kare"`, "1. parametresi", `"Tam"`, `"Metin"`)
	noErrors(t, def+"kare(3)")
	noErrors(t, def+"kare(girdi())")
	noErrors(t, def+"kare(yok)")

	two := "fonksiyon böl(a: Ondalık, b: Ondalık) -> Ondalık { sonuç a / b }\n"
	noErrors(t, two+"böl(1, 2)")
	singleError(t, two+`böl(1, "iki")`, "2. parametresi", `"Ondalık"`, `"Metin"`)

	// Tipsiz parametreler her değeri kabul eder
	noErrors(t, "fonksiyon f(a, b: Tam) { }\nf(\"x\", 2)")
	singleError(t, "fonksiyon f(a, b: Tam) { }\nf(\"x\", \"y\")", "2. parametresi")

	// Parametreler fonksiyon gövdesinde bildirilen tipleriyle bilinir
	singleError(t, "fonksiyon f(a: Metin) {\n  değişken b: Tam = a\n}", "Satır 2:", `"b"`, `"Metin"`)

	// Özyinelemeli çağrılar da denetlenir (ad, gövdeden önce tanımlanır)
	singleError(t, "fonksiyon f(n: Tam) {\n  sonuç f(\"x\")\n}", "Satır 2:", "Fonksiyon çağrısı tip hatası")

	// İç içe çağrılar denetlenir
	singleError(t, def+"yazdır(kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
	singleError(t, def+"kare(kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
	// Fazladan argümanlar ve isimli argümanlar içindeki çağrılar da denetlenir
	singleError(t, def+"kare(1, kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
	singleError(t, def+"yazdır(1, ayraç: kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
}

func TestFunctionReturnTypeInference(t *testing.T) {
	def := "fonksiyon topla(a: Tam, b: Tam) -> Tam { sonuç a + b }\n"
	singleError(t, def+"değişken s: Metin = topla(1, 2)", "Satır 2:", `"Metin"`, `"Tam"`)
	noErrors(t, def+"değişken s: Ondalık = topla(1, 2)")

	// Parametresiz fonksiyonların dönüş tipi de kullanılır
	singleError(t, "fonksiyon ad(): Metin { sonuç \"Ali\" }\ndeğişken n: Tam = ad()", "Satır 2:", `"Tam"`, `"Metin"`)

	// Dönüş tipi belirtilmemişse sonuç bilinmez (Herhangi)
	noErrors(t, "fonksiyon f(a: Tam) { sonuç a }\ndeğişken s: Metin = f(1)")
	noErrors(t, "fonksiyon f() { sonuç 1 }\ndeğişken s: Metin = f()")
}

func TestReturnTypeCheck(t *testing.T) {
	singleError(t, "fonksiyon f(): Tam {\n  sonuç \"x\"\n}", "Satır 2:", "Dönüş tipi uyuşmazlığı", `"Tam"`, `"Metin"`)
	singleError(t, "fonksiyon f() -> Metin {\n  sonuç 1\n}", "Satır 2:", "Dönüş tipi uyuşmazlığı")
	singleError(t, "fonksiyon f(): Tam { sonuç 2.5 }", "Dönüş tipi uyuşmazlığı", `"Ondalık"`)
	// Tam / Tam Tam olmayabilir (7 / 2 = 3.5): ': Tam' bildiren fonksiyon bunu döndüremez
	singleError(t, "fonksiyon ort(a: Tam, b: Tam): Tam { sonuç a / b }", "Dönüş tipi uyuşmazlığı", `"Tam ya da Ondalık"`)
	noErrors(t, "fonksiyon ort(a: Tam, b: Tam): Ondalık { sonuç a / b }")

	// İç bloklardaki dönüşler de denetlenir
	singleError(t, "fonksiyon f(x): Tam {\n  eğer x {\n    sonuç \"a\"\n  }\n  sonuç 1\n}", "Satır 3:", "Dönüş tipi uyuşmazlığı")
	singleError(t, "fonksiyon f(x): Tam {\n  döngü i içinde x {\n    sonuç doğru\n  }\n}", "Satır 3:", "Dönüş tipi uyuşmazlığı")

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
	errs := check(t, "fonksiyon f(): Tam {\n  fonksiyon g(): Metin { sonuç 1 }\n  sonuç \"x\"\n}")
	if len(errs) != 2 || !strings.Contains(errs[0], `"Metin"`) || !strings.Contains(errs[1], `fonksiyon "Tam"`) {
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
			singleError(t, def+body, "Fonksiyon çağrısı tip hatası")
		})
	}
}

func TestMultipleErrorsInOrder(t *testing.T) {
	errs := check(t, `değişken a: Tam = "x"
sabit b = 1
b = 2
fonksiyon f(x: Metin): Tam { sonuç x }
f(5)`)
	expected := []string{"Satır 1: Tip uyuşmazlığı", "Satır 3: Sabit hatası", "Satır 4: Dönüş tipi", "Satır 5: Fonksiyon çağrısı"}
	if len(errs) != len(expected) {
		t.Fatalf("%d hata bekleniyordu, gelen: %v", len(expected), errs)
	}
	for i, b := range expected {
		if !strings.HasPrefix(errs[i], b) {
			t.Errorf("hata[%d] %q ile başlamalı, gelen: %q", i, b, errs[i])
		}
	}
}

// Yerleşik bir adı kullanıcı adı olarak tanımlamak hata değil uyarıdır: program çalışır,
// ama o kapsamda yerleşik artık kullanılamaz.
func TestBuiltinShadowingWarning(t *testing.T) {
	warning := func(line int, name string) string {
		return fmt.Sprintf("Satır %d: '%s' yerleşik bir ad; bu kapsamda yerleşik '%s' artık kullanılamaz", line, name, name)
	}
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
	expected := []string{warning(2, "toplam"), warning(5, "girdi"), warning(6, "metin")}
	if got := c.Warnings(); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("uyarılar:\n%s\nbeklenen:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
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
		{"yapı metot gövdeleri", "fonksiyon kare(x: Tam) { }\nyapı A {\n  fonksiyon m() { kare(\"a\") }\n}"},
		{"döngü değişkenleri tipsizdir", "döngü x içinde [1, 2] { değişken s: Metin = x }"},
		{"uyumsuz işlenenler", "değişken a = \"x\" - 1\ndeğişken b = doğru + 1"},
		{"koşul tipi", "eğer 5 { }\niken \"x\" { }"},
		{"liste üreteci değişkeni", "fonksiyon kare(x: Tam) { }\ndeğişken l = [kare(\"a\") döngü x içinde 1..3]"},
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
	files, _ := filepath.Glob(reporoot.Yol("examples/*.tr"))
	if len(files) == 0 {
		t.Fatal("examples/*.tr bulunamadı")
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
	// uyarısız denetlenmeli: yerleşik adları gölgeleyen kullanıcı adı olmamalı.
	var all []string
	for _, pattern := range []string{"examples/*.tr", "examples/*/*.tr", "tests/*.tr", "tools/bench/tr/*.tr"} {
		d, _ := filepath.Glob(reporoot.Yol(pattern))
		all = append(all, d...)
	}
	for _, file := range all {
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
		if _, ok := typeSpellings[lowerCase(typ)]; ok {
			t.Errorf("%q kabul edilen bir yazım olmamalı (3.14)", lowerCase(typ))
		}
	}
	for _, c := range []struct{ in, want string }{{"", "Herhangi"}, {"TAM", "Tam"}, {"Herhangi", "Herhangi"}, {"herhangi", "herhangi"}, {"Nokta", "Nokta"}} {
		if got := normalizeTypeName(c.in); got != c.want {
			t.Errorf("normalizeTypeName(%q) = %q, %q bekleniyordu", c.in, got, c.want)
		}
	}
}

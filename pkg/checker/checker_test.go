package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

// denetle kaynağı ayrıştırır (sözdizimi hatası testi düşürür) ve tip denetimi hatalarını döndürür.
func denetle(t *testing.T, kaynak string) []string {
	t.Helper()
	p := parser.New(lexer.New(kaynak))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), kaynak)
	}
	c := New()
	errs := c.CheckProgram(prog)
	if len(errs) != len(c.Errors()) {
		t.Errorf("CheckProgram ve Errors() farklı sayıda hata döndürdü: %v / %v", errs, c.Errors())
	}
	return errs
}

func hataYok(t *testing.T, kaynak string) {
	t.Helper()
	if errs := denetle(t, kaynak); len(errs) > 0 {
		t.Errorf("hata beklenmiyordu, gelen: %v\nkaynak:\n%s", errs, kaynak)
	}
}

// tekHata kaynağın tam olarak bir hata üretmesini ve hatanın tüm parçaları içermesini bekler.
func tekHata(t *testing.T, kaynak string, parcalar ...string) {
	t.Helper()
	errs := denetle(t, kaynak)
	if len(errs) != 1 {
		t.Errorf("tek bir hata bekleniyordu, gelen %d: %v\nkaynak:\n%s", len(errs), errs, kaynak)
		return
	}
	for _, p := range parcalar {
		if !strings.Contains(errs[0], p) {
			t.Errorf("hata %q parçasını içermeli, gelen: %q\nkaynak:\n%s", p, errs[0], kaynak)
		}
	}
}

// ---------------------------------------------------------------------------
// Tip adları ve uyumluluk kuralları
// ---------------------------------------------------------------------------

func TestTipAdiNormallestirme(t *testing.T) {
	testler := map[string]string{
		"Tam": "Tam", "tam": "Tam", "TAM": "Tam",
		"Ondalık": "Ondalık", "ONDALIK": "Ondalık",
		"Metin": "Metin", "METİN": "Metin",
		"Mantık": "Mantık", "MANTIK": "Mantık",
		"Liste": "Liste", "Sözlük": "Sözlük", "Tensör": "Tensör", "Yok": "Yok",
		"": "Herhangi",
		// Bilinmeyen adlar (kullanıcı yapıları) olduğu gibi kalır
		"Nokta": "Nokta", "Fonksiyon": "Fonksiyon",
	}
	for girdi, want := range testler {
		if got := normalizeTypeName(girdi); got != want {
			t.Errorf("normalizeTypeName(%q) = %q, beklenen %q", girdi, got, want)
		}
	}
}

// Tip adlarının yalnızca Türkçe yazımı geçerlidir; ASCII yazım hata verir ve asıl adı
// önerir (tek hata: aynı satırda ayrıca tip uyuşmazlığı bildirilmez).
func TestTipAdiASCIIOneri(t *testing.T) {
	tekHata(t, `değişken a: ondalik = 1.5`, "Satır 1: 'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?")
	tekHata(t, `değişken a: MANTIK = doğru
değişken b: tensor = yok`, "Satır 2: 'tensor' TR'de bir tip adı değil; 'Tensör' yazmak mı istediniz?")
	tekHata(t, `fonksiyon f(x: sozluk) { sonuç x }`, "'sozluk' TR'de bir tip adı değil; 'Sözlük'")
	tekHata(t, `fonksiyon f(x) -> mantik { sonuç doğru }`, "'mantik' TR'de bir tip adı değil; 'Mantık'")
	// Kaldırılan eş yazımlar da hata verir ve asıl adı önerir.
	for ad, asil := range map[string]string{"tamsayı": "Tam", "int": "Tam", "sayı": "Ondalık", "float": "Ondalık",
		"yazı": "Metin", "str": "Metin", "string": "Metin", "mantıksal": "Mantık", "bool": "Mantık", "boş": "Yok",
		"null": "Yok", "dizi": "Liste", "list": "Liste", "harita": "Sözlük", "dict": "Sözlük", "map": "Sözlük",
		"Matris": "Tensör", "matrix": "Tensör", "TAMSAYI": "Tam"} {
		tekHata(t, "değişken a: "+ad, "'"+ad+"' TR'de bir tip adı değil; '"+asil+"' yazmak mı istediniz?")
	}
}

func TestTipUyumlulugu(t *testing.T) {
	testler := []struct {
		beklenen, gercek string
		uyumlu           bool
	}{
		{"Tam", "Tam", true},
		{"Ondalık", "Tam", true}, // Tam, Ondalık beklenen yere atanabilir
		{"Tam", "Ondalık", false},
		{"Metin", "Tam", false},
		{"Herhangi", "Metin", true},
		{"Metin", "Herhangi", true},
		{"Liste", "Yok", true}, // 'yok' her tipe atanabilir
		{"Yok", "Tam", false},
		{"tam", "Tam", true}, // tip adları büyük/küçük harfe duyarsızdır
		{"Nokta", "Nokta", true},
		{"Nokta", "Tam", false},
		{"Liste", "Sözlük", false},
	}
	for _, tt := range testler {
		if got := typesCompatible(tt.beklenen, tt.gercek); got != tt.uyumlu {
			t.Errorf("typesCompatible(%q, %q) = %v, beklenen %v", tt.beklenen, tt.gercek, got, tt.uyumlu)
		}
	}
}

// ---------------------------------------------------------------------------
// Tip belirtimli değişkenler
// ---------------------------------------------------------------------------

func TestTipBelirtimliDegiskenUyumlu(t *testing.T) {
	for _, kaynak := range []string{
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
		`değişken a: aralık = 0..3`,
		`değişken a: Ondalık = 7 / 2`,
		`değişken a: Ondalık = 2 ** -1`,
		`değişken a: Tam = 7 % 2`,
		`değişken a: Tam = (2 ** 3) * 4`,
		`değişken a = 7 / 2`,
		`değişken a: Liste = [x * 2 döngü x içinde 1..3]`,
		`değişken a: Sözlük = {"a": 1}`,
		`değişken a: Tensör = m1 @ m2`,
		`değişken a: Tam = yok`,
		`değişken a: Tam`, // değer verilmezse 'yok' atanır
		`değişken a: Tam = tanımsız_bir_şey`,
		`değişken a: Tam = yazdır("x")`,
		`değişken a: Tam = 5 |> kare`,
		`değişken a: Tam = (5 ?? 6)`,
		`değişken a: Metin = yok ?? "varsayılan"`,
		`değişken a: Tam = -5`,
		`değişken a: Tam = 2 ** 3`,
		`değişken a: TAM = 7 % 2`,
		`değişken a: Fonksiyon = fonksiyon(x) { sonuç x }`,
		`sabit a: Tam = 1`,
	} {
		hataYok(t, kaynak)
	}
}

func TestTipBelirtimliDegiskenUyumsuz(t *testing.T) {
	testler := []struct {
		kaynak, beklenen, gercek string
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
	for _, tt := range testler {
		tekHata(t, tt.kaynak, "Satır 1:", "Tip uyuşmazlığı", `"`+tt.beklenen+`"`, `"`+tt.gercek+`"`)
	}
}

func TestTipHatasiSatirNumarasiVeDegiskenAdi(t *testing.T) {
	tekHata(t, "değişken a = 1\n\ndeğişken öğrenci_sayısı: Tam = \"çok\"",
		"Satır 3:", `"öğrenci_sayısı" değişkeni`)
}

// ---------------------------------------------------------------------------
// Atamalar
// ---------------------------------------------------------------------------

func TestAtamaTipDenetimi(t *testing.T) {
	// Belirtilen tip korunur
	tekHata(t, "değişken a: Tam = 1\na = \"x\"", "Satır 2:", "Tip uyuşmazlığı", `"a" değişkeni (Tam)`, `"Metin"`)
	hataYok(t, "değişken a: Ondalık = 1\na = 2.5\na = 3")

	// Dil dinamiktir: tip açıklaması olmayan değişkenin ilk değerden çıkarılan tipi kalıcı
	// bir bildirim değildir; başka tipte değer atanabilir. Sabitlerin tipi kalır.
	hataYok(t, "değişken a = 5\na = \"beş\"")
	hataYok(t, "x := doğru\nx = 1")
	hataYok(t, "değişken kayıp = 0\ndöngü i içinde 1..3 { kayıp += 0.5 }")
	tekHata(t, "sabit S = 5\ndeğişken m: Metin = S", "Satır 2:", `"Metin"`, `"Tam"`)

	// 'yok' veya bilinmeyen tipte başlayan değişkenler serbestçe atanabilir
	hataYok(t, "değişken a = yok\na = 5\na = \"x\"")
	hataYok(t, "değişken a = bilinmeyen()\na = 5\na = \"x\"")
	hataYok(t, "değişken a\na = [1]")

	// Tanımlanmamış bir hedefe atama denetlenmez
	hataYok(t, "tanımsız = 5")
}

func TestBilesikAtama(t *testing.T) {
	// 's += 1' ile 's = s + 1' aynı şekilde değerlendirilmeli (Metin + Tam = Metin)
	hataYok(t, "değişken s = \"a\"\ns += 1")
	hataYok(t, "değişken s = \"a\"\ns = s + 1")
	hataYok(t, "değişken x = 10\nx += 1\nx -= 2\nx *= 3")
	hataYok(t, "değişken x = 1.5\nx += 1")
	hataYok(t, "değişken x: Ondalık = 1\nx *= 2.5")

	// 'x = x + 0.5' gibi, Tam açıklamalı bir değişkene Ondalık sonuç atanamaz; açıklamasız
	// değişken (biriktirici) serbesttir.
	tekHata(t, "değişken x: Tam = 1\nx += 0.5", "Satır 2:", `"x" değişkeni (Tam)`, `"Ondalık"`)
	tekHata(t, "değişken x: Tam = 1\nx = x + 0.5", "Satır 2:", `"x" değişkeni (Tam)`, `"Ondalık"`)
	hataYok(t, "değişken x = 1\nx += 0.5")

	// '/' Tam/Tam'da tam bölünmezse Ondalık verir (10 / 4 = 2.5): sonuç "Tam ya da Ondalık"tır.
	hataYok(t, "değişken x = 10\nx /= 4")
	tekHata(t, "değişken y: Tam = 10 / 4", "Satır 1:", `"Tam ya da Ondalık"`, "tam(...)")
	tekHata(t, "değişken y: Tam = 10\ny /= 4", "Satır 2:", `"Tam ya da Ondalık"`)
}

func TestSabitDegistirilemez(t *testing.T) {
	tekHata(t, "sabit PI = 3.14\nPI = 3.0", "Satır 2:", "Sabit hatası", `"PI"`)
	tekHata(t, "sabit SAYAÇ = 1\nSAYAÇ += 1", "Satır 2:", "Sabit hatası", `"SAYAÇ"`)
	tekHata(t, "sabit a: Metin = \"x\"\na = \"y\"", "Satır 2:", "Sabit hatası")

	// Çoklu tanım ve çoklu atama
	tekHata(t, "sabit a, b = 1, 2\nb = 3", "Satır 2:", "Sabit hatası", `"b"`)
	tekHata(t, "sabit a = 1\ndeğişken b = 2\nb, a = a, b", "Satır 3:", "Sabit hatası", `"a"`)

	// İç kapsamlardan dış sabite atama da yakalanır
	tekHata(t, "sabit a = 1\neğer doğru {\n  a = 2\n}", "Satır 3:", "Sabit hatası")
	tekHata(t, "sabit a = 1\nfonksiyon f() {\n  a = 2\n}", "Satır 3:", "Sabit hatası")
	tekHata(t, "sabit a = 1\ndöngü i içinde 1..3 {\n  a = i\n}", "Satır 3:", "Sabit hatası")

	// Hem sabit hem tip hatası aynı anda bildirilir
	errs := denetle(t, "sabit a = 1\na = \"x\"")
	if len(errs) != 2 || !strings.Contains(errs[0], "Sabit hatası") || !strings.Contains(errs[1], "Tip uyuşmazlığı") {
		t.Errorf("önce sabit, sonra tip hatası bekleniyordu, gelen: %v", errs)
	}
}

// İç kapsamda aynı adla tanımlanan değişken/parametre dış sabiti gölgeler.
func TestSabitGolgeleme(t *testing.T) {
	hataYok(t, "sabit x = 1\neğer doğru {\n  değişken x = 2\n  x = 3\n}")
	hataYok(t, "sabit x = 1\nfonksiyon f(x) {\n  x = 5\n}")
	hataYok(t, "sabit x = 1\ndöngü x içinde [1, 2] {\n  x = 5\n}")
	hataYok(t, "sabit i = 1\ndöngü i, x içinde [1, 2] {\n  i = 5\n}")
	hataYok(t, "sabit e = 1\ndene {\n} yakala e {\n  e = \"x\"\n}")
	// Blok bitince dış sabit yeniden geçerlidir
	tekHata(t, "sabit x = 1\neğer doğru {\n  değişken x = 2\n}\nx = 3", "Satır 5:", "Sabit hatası")
}

// ---------------------------------------------------------------------------
// Fonksiyonlar
// ---------------------------------------------------------------------------

func TestFonksiyonParametreTipleri(t *testing.T) {
	tanim := "fonksiyon kare(x: Tam) { sonuç x * x }\n"
	tekHata(t, tanim+`kare("üç")`, "Satır 2:", "Fonksiyon çağrısı tip hatası", `"kare"`, "1. parametresi", `"Tam"`, `"Metin"`)
	hataYok(t, tanim+"kare(3)")
	hataYok(t, tanim+"kare(bilinmeyen)")
	hataYok(t, tanim+"kare(yok)")

	iki := "fonksiyon böl(a: Ondalık, b: Ondalık) -> Ondalık { sonuç a / b }\n"
	hataYok(t, iki+"böl(1, 2)")
	tekHata(t, iki+`böl(1, "iki")`, "2. parametresi", `"Ondalık"`, `"Metin"`)

	// Tipsiz parametreler her değeri kabul eder
	hataYok(t, "fonksiyon f(a, b: Tam) { }\nf(\"x\", 2)")
	tekHata(t, "fonksiyon f(a, b: Tam) { }\nf(\"x\", \"y\")", "2. parametresi")

	// Parametreler fonksiyon gövdesinde bildirilen tipleriyle bilinir
	tekHata(t, "fonksiyon f(a: Metin) {\n  değişken b: Tam = a\n}", "Satır 2:", `"b"`, `"Metin"`)

	// Özyinelemeli çağrılar da denetlenir (ad, gövdeden önce tanımlanır)
	tekHata(t, "fonksiyon f(n: Tam) {\n  sonuç f(\"x\")\n}", "Satır 2:", "Fonksiyon çağrısı tip hatası")

	// İç içe çağrılar denetlenir
	tekHata(t, tanim+"yazdır(kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
	tekHata(t, tanim+"kare(kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
	// Fazladan argümanlar ve isimli argümanlar içindeki çağrılar da denetlenir
	tekHata(t, tanim+"kare(1, kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
	tekHata(t, tanim+"yazdır(1, ayraç: kare(\"a\"))", "Satır 2:", "Fonksiyon çağrısı tip hatası")
}

func TestFonksiyonDonusTipiCikarimi(t *testing.T) {
	tanim := "fonksiyon topla(a: Tam, b: Tam) -> Tam { sonuç a + b }\n"
	tekHata(t, tanim+"değişken s: Metin = topla(1, 2)", "Satır 2:", `"Metin"`, `"Tam"`)
	hataYok(t, tanim+"değişken s: Ondalık = topla(1, 2)")

	// Parametresiz fonksiyonların dönüş tipi de kullanılır
	tekHata(t, "fonksiyon ad(): Metin { sonuç \"Ali\" }\ndeğişken n: Tam = ad()", "Satır 2:", `"Tam"`, `"Metin"`)

	// Dönüş tipi belirtilmemişse sonuç bilinmez (Herhangi)
	hataYok(t, "fonksiyon f(a: Tam) { sonuç a }\ndeğişken s: Metin = f(1)")
	hataYok(t, "fonksiyon f() { sonuç 1 }\ndeğişken s: Metin = f()")
}

func TestDonusTipiDenetimi(t *testing.T) {
	tekHata(t, "fonksiyon f(): Tam {\n  sonuç \"x\"\n}", "Satır 2:", "Dönüş tipi uyuşmazlığı", `"Tam"`, `"Metin"`)
	tekHata(t, "fonksiyon f() -> Metin {\n  sonuç 1\n}", "Satır 2:", "Dönüş tipi uyuşmazlığı")
	tekHata(t, "fonksiyon f(): Tam { sonuç 2.5 }", "Dönüş tipi uyuşmazlığı", `"Ondalık"`)
	// Tam / Tam Tam olmayabilir (7 / 2 = 3.5): ': Tam' bildiren fonksiyon bunu döndüremez
	tekHata(t, "fonksiyon ort(a: Tam, b: Tam): Tam { sonuç a / b }", "Dönüş tipi uyuşmazlığı", `"Tam ya da Ondalık"`)
	hataYok(t, "fonksiyon ort(a: Tam, b: Tam): Ondalık { sonuç a / b }")

	// İç bloklardaki dönüşler de denetlenir
	tekHata(t, "fonksiyon f(x): Tam {\n  eğer x {\n    sonuç \"a\"\n  }\n  sonuç 1\n}", "Satır 3:", "Dönüş tipi uyuşmazlığı")
	tekHata(t, "fonksiyon f(x): Tam {\n  döngü i içinde x {\n    sonuç doğru\n  }\n}", "Satır 3:", "Dönüş tipi uyuşmazlığı")

	for _, kaynak := range []string{
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
		hataYok(t, kaynak)
	}

	// İç fonksiyonlar dış fonksiyonun dönüş tipini devralmaz
	hataYok(t, "fonksiyon f(): Tam {\n  değişken g = (x) => \"metin\"\n  sonuç 1\n}")
	hataYok(t, "fonksiyon f(): Tam {\n  fonksiyon g() { sonuç \"metin\" }\n  sonuç 1\n}")
	// İç fonksiyonun kendi dönüş tipi denetlenir; dış fonksiyonunki geri yüklenir
	errs := denetle(t, "fonksiyon f(): Tam {\n  fonksiyon g(): Metin { sonuç 1 }\n  sonuç \"x\"\n}")
	if len(errs) != 2 || !strings.Contains(errs[0], `"Metin"`) || !strings.Contains(errs[1], `fonksiyon "Tam"`) {
		t.Errorf("iç ve dış fonksiyon için birer dönüş hatası bekleniyordu, gelen: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Denetimin tüm deyim türlerine ulaşması
// ---------------------------------------------------------------------------

func TestIcIceDeyimlerDenetlenir(t *testing.T) {
	tanim := "fonksiyon kare(x: Tam) { sonuç x * x }\n"
	hatali := `kare("a")`
	testler := map[string]string{
		"eğer":             "eğer doğru { " + hatali + " }",
		"değilse eğer":     "eğer yanlış { } değilse eğer doğru { " + hatali + " }",
		"değilse":          "eğer yanlış { } değilse { " + hatali + " }",
		"eğer koşulu":      "eğer " + hatali + " > 1 { }",
		"iken":             "iken yanlış { " + hatali + " }",
		"iken koşulu":      "iken " + hatali + " > 1 { }",
		"döngü":            "döngü i içinde 1..3 { " + hatali + " }",
		"döngü kaynağı":    "döngü i içinde " + hatali + " { }",
		"dene":             "dene { " + hatali + " } yakala h { }",
		"yakala":           "dene { } yakala h { " + hatali + " }",
		"eşle gövde":       "eşle 1 { 1 => { " + hatali + " } }",
		"eşle ifade":       "eşle 1 { 1 => " + hatali + " }",
		"eşle konusu":      "eşle " + hatali + " { varsayılan => 1 }",
		"ertele blok":      "ertele { " + hatali + " }",
		"ertele çağrı":     "ertele " + hatali,
		"sonuç":            "fonksiyon g() { sonuç " + hatali + " }",
		"fırlat":           "fırlat " + hatali,
		"eşzamanlı blok":   "eşzamanlı { " + hatali + " }",
		"eşzamanlı çağrı":  "eşzamanlı " + hatali,
		"kesme noktası":    "kesme_noktası eğer " + hatali + " > 1",
		"lambda":           "değişken g = (y) => " + hatali,
		"çoklu tanım":      "değişken a, b = 1, " + hatali,
		"çoklu atama":      "değişken a = 1\ndeğişken b = 2\na, b = " + hatali + ", 1",
		"atama değeri":     "değişken a = 1\na = " + hatali,
		"liste dışı ifade": "yazdır(" + hatali + ")",
	}
	for ad, govde := range testler {
		t.Run(ad, func(t *testing.T) {
			tekHata(t, tanim+govde, "Fonksiyon çağrısı tip hatası")
		})
	}
}

func TestBirdenFazlaHataSirayla(t *testing.T) {
	errs := denetle(t, `değişken a: Tam = "x"
sabit b = 1
b = 2
fonksiyon f(x: Metin): Tam { sonuç x }
f(5)`)
	beklenen := []string{"Satır 1: Tip uyuşmazlığı", "Satır 3: Sabit hatası", "Satır 4: Dönüş tipi", "Satır 5: Fonksiyon çağrısı"}
	if len(errs) != len(beklenen) {
		t.Fatalf("%d hata bekleniyordu, gelen: %v", len(beklenen), errs)
	}
	for i, b := range beklenen {
		if !strings.HasPrefix(errs[i], b) {
			t.Errorf("hata[%d] %q ile başlamalı, gelen: %q", i, b, errs[i])
		}
	}
}

// Yerleşik bir adı kullanıcı adı olarak tanımlamak hata değil uyarıdır: program çalışır,
// ama o kapsamda yerleşik artık kullanılamaz.
func TestYerlesikGolgelemeUyarisi(t *testing.T) {
	uyari := func(satir int, ad string) string {
		return fmt.Sprintf("Satır %d: '%s' yerleşik bir ad; bu kapsamda yerleşik '%s' artık kullanılamaz", satir, ad, ad)
	}
	kaynak := `değişken uzunluk = 5
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
	p := parser.New(lexer.New(kaynak))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	c := New()
	if errs := c.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("gölgeleme hata değildir: %v", errs)
	}
	beklenen := []string{uyari(1, "uzunluk"), uyari(2, "toplam"), uyari(5, "girdi"), uyari(6, "metin"), uyari(7, "ekle"), uyari(8, "sırala")}
	if got := c.Warnings(); strings.Join(got, "\n") != strings.Join(beklenen, "\n") {
		t.Errorf("uyarılar:\n%s\nbeklenen:\n%s", strings.Join(got, "\n"), strings.Join(beklenen, "\n"))
	}
	// Yapı metotları ve alanları yerleşikleri gölgelemez (x.uzunluk() ile erişilir).
	if u := New().Warnings(); len(u) != 0 {
		t.Errorf("yeni denetleyicide uyarı olmamalı: %v", u)
	}
}

func TestBosProgram(t *testing.T) {
	hataYok(t, "")
	hataYok(t, "// yalnızca yorum")
	if errs := New().Errors(); errs == nil || len(errs) != 0 {
		t.Errorf("yeni denetleyicinin hata listesi boş (nil olmayan) olmalı, gelen: %#v", errs)
	}
}

// ---------------------------------------------------------------------------
// Bilinen sınırlamalar: denetleyici şu anda bunları YAKALAMAZ.
// Bu testler mevcut davranışı belgeler; bir sınırlama giderildiğinde ilgili
// satır hata bekleyecek şekilde güncellenmelidir.
// ---------------------------------------------------------------------------

func TestBilinenSinirlamalar(t *testing.T) {
	testler := []struct {
		ad, kaynak string
	}{
		{"tanımsız tanımlayıcı", "yazdır(tanımsız_değişken)"},
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
	for _, tt := range testler {
		t.Run(tt.ad, func(t *testing.T) {
			hataYok(t, tt.kaynak)
		})
	}

	// Yakalanan hata değişkeni açıklamasızdır: başka tipte değer alabilir.
	hataYok(t, "dene { } yakala h {\n  h = 5\n}")
}

// ---------------------------------------------------------------------------
// Örnek programlar yanlış alarm üretmemeli
// ---------------------------------------------------------------------------

func TestOrnekProgramlarHatasizDenetlenir(t *testing.T) {
	dosyalar, _ := filepath.Glob(filepath.Join("..", "..", "ornekler", "*.tr"))
	if len(dosyalar) == 0 {
		t.Skip("ornekler/*.tr bulunamadı")
	}
	for _, dosya := range dosyalar {
		t.Run(filepath.Base(dosya), func(t *testing.T) {
			veri, err := os.ReadFile(dosya)
			if err != nil {
				t.Fatal(err)
			}
			hataYok(t, string(veri))
		})
	}
	// Depodaki programlar (örnekler, dil tanımı testleri, kıyas ve çalışma dosyaları)
	// uyarısız denetlenmeli: yerleşik adları gölgeleyen kullanıcı adı olmamalı.
	var hepsi []string
	for _, desen := range []string{"ornekler/*.tr", "ornekler/*/*.tr", "testler/*.tr", "kiyas/tr/*.tr", "calisma/*.tr"} {
		d, _ := filepath.Glob(filepath.Join("..", "..", filepath.FromSlash(desen)))
		hepsi = append(hepsi, d...)
	}
	for _, dosya := range hepsi {
		veri, err := os.ReadFile(dosya)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(strings.TrimPrefix(string(veri), "\ufeff")))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Errorf("%s: %v", dosya, p.Errors())
			continue
		}
		c := New()
		c.CheckProgram(prog)
		if u := c.Warnings(); len(u) > 0 {
			t.Errorf("%s uyarısız denetlenmeli: %v", dosya, u)
		}
	}
}

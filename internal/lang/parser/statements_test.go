package parser

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// applyFix düzeltmeyi metne uygular: aralığın (satır, sütun karakterle, 1'den; bitiş dışlayıcı) yerine
// yeni metin yazılır.
func applyFix(t *testing.T, code string, f messages.Fix) string {
	t.Helper()
	lines := strings.SplitAfter(code, "\n")
	offset := func(line, col int) int {
		n := 0
		for i := 0; i < line-1; i++ {
			n += len([]rune(lines[i]))
		}
		return n + col - 1
	}
	r := []rune(code)
	start, end := offset(f.Line, f.Col), offset(f.EndLine, f.EndCol)
	if start < 0 || end > len(r) || start > end {
		t.Fatalf("%q: düzeltmenin aralığı metnin dışında: %+v", code, f)
	}
	return string(r[:start]) + f.Replacement + string(r[end:])
}

// fixedCase bir sözdizimi hatasının yeri, kodu ve tek düzeltmesidir: düzeltme uygulanınca metin
// hatasız ayrıştırılır (iletinin önerisi kopyalanınca çalışır).
type fixedCase struct {
	code, at   string
	want       messages.Code
	fixed      string
	applicable bool
}

func checkFixedCases(t *testing.T, cases []fixedCase) {
	t.Helper()
	for _, c := range cases {
		line, col := posOf(t, c.code, c.at)
		errs := parseErrs(c.code)
		if !hasErrAt(errs, line, col, c.want) {
			t.Errorf("%q: %d:%d'de %s bekleniyordu, alınan: %v", c.code, line, col, c.want, errs)
			continue
		}
		pos, _ := messages.Find(errs[0])
		if len(pos.Fixes) != 1 {
			t.Errorf("%q: tek düzeltme bekleniyordu: %+v", c.code, pos.Fixes)
			continue
		}
		if pos.Fixes[0].Applicable != c.applicable {
			t.Errorf("%q: uygulanabilir %v bekleniyordu", c.code, c.applicable)
		}
		got := applyFix(t, c.code, pos.Fixes[0])
		if got != c.fixed {
			t.Errorf("%q: düzeltilmiş metin %q, beklenen %q", c.code, got, c.fixed)
		}
		if errs := parseErrs(got); len(errs) != 0 {
			t.Errorf("%q: düzeltilmiş metin ayrıştırılamadı: %v", got, errs)
		}
	}
}

// Kural (4.9, 5.4): satır sonundaki '//' (sağında ifade yok) yorum alışkanlığıdır; hata '//'dedir,
// düzeltme '#' yazar ve parantez dışında olduğu gibi uygulanır. Parantez içinde kapanış da yoruma
// girerdi: düzeltme yalnız öneridir. '//=' bileşik atamadır, H0025 vermez.
func TestFloorDivisionWithoutRightSideSuggestsAComment(t *testing.T) {
	checkFixedCases(t, []fixedCase{
		{"TR_var a = 7 //", "//", messages.H0025, "TR_var a = 7 #", true},
		{"TR_var a = 7 // : not", "//", messages.H0025, "TR_var a = 7 # : not", true},
	})
	code := "TR_print(7 // )"
	errs := parseErrs(code)
	line, col := posOf(t, code, "//")
	if !hasErrAt(errs, line, col, messages.H0025) {
		t.Fatalf("%q: H0025 bekleniyordu: %v", code, errs)
	}
	if pos, _ := messages.Find(errs[0]); len(pos.Fixes) != 1 || pos.Fixes[0].Applicable || pos.Fixes[0].Replacement != "#" {
		t.Errorf("%q: parantez içinde uygulanamaz '#' önerisi bekleniyordu: %+v", code, pos.Fixes)
	}
	if errs := parseErrs("TR_var x = 9\nx //= 2"); len(errs) != 0 {
		t.Errorf("'//=' hata vermemeli: %v", errs)
	}
	// Hedef tam bölmenin sağında bir ifadeyse tam bölmedir: H0025 bir kez, '//'de.
	if errs := parseErrs("TR_var a = 7 // 2"); len(errs) != 0 {
		t.Errorf("tam bölme hata vermemeli: %v", errs)
	}
}

// Kural (5.1): kısa tanım (x := 5) yoktur, ileti ve düzeltme TR_var'ı yazar; TR_var ile ':=' de hatadır;
// ilk değeri yazılmamış tanım hatadır, önerilen '= TR_none' bugünkü anlamı korur ama asıl değer
// olmadığından olduğu gibi uygulanmaz.
func TestDeclarationNeedsTRVarAndAFirstValue(t *testing.T) {
	checkFixedCases(t, []fixedCase{
		{"b := 2", ":=", messages.H0250, "TR_var b = 2", true},
		{"TR_func f() {\n    toplam := [1]\n    TR_return toplam\n}", ":=", messages.H0250, "TR_func f() {\n    TR_var toplam = [1]\n    TR_return toplam\n}", true},
		{"TR_var x := 5", ":=", messages.H0251, "TR_var x = 5", true},
		{"TR_var a, b := 1, 2", ":=", messages.H0251, "TR_var a, b = 1, 2", true},
		{"TR_const K := 5", ":=", messages.H0251, "TR_const K = 5", true},
		{"TR_var y\nTR_print(y)", "y", messages.H0252, "TR_var y = TR_none\nTR_print(y)", false},
		{"TR_var a: TR_Int", "TR_Int", messages.H0252, "TR_var a: TR_Int = TR_none", false},
		{"TR_const K", "K", messages.H0252, "TR_const K = TR_none", false},
	})
	// İleti TR_var'ı kodlu dosyada yazımıyla söyler (kopyalanınca çalışır).
	if errs := parseErrs("b := 2"); len(errs) == 0 || !strings.Contains(errs[0].Error(), "TR_var b =") {
		t.Errorf("ileti TR_var b = yazmalıydı: %v", errs)
	}
}

// Kural (6.1, 6.4): dönüş tipi '->' ile yazılır; değer döndürmeyende '->' yazılmaz; Python'un def'i ve
// süsleyicisi yoktur; aynı blokta ikinci tanım ve varsayılanlıdan sonra varsayılansız parametre hatadır.
func TestFunctionDefinitionSpellings(t *testing.T) {
	checkFixedCases(t, []fixedCase{
		{"TR_func f(x: TR_Int): TR_Int { TR_return x }", ": TR_Int {", messages.H0256, "TR_func f(x: TR_Int) -> TR_Int { TR_return x }", true},
		{"TR_func f(x: TR_Int) : TR_Int { TR_return x }", ": TR_Int {", messages.H0256, "TR_func f(x: TR_Int) -> TR_Int { TR_return x }", true},
		{"TR_func f(x: TR_Int) -> TR_NoneType { TR_print(x) }", "TR_NoneType", messages.H0257, "TR_func f(x: TR_Int) { TR_print(x) }", true},
	})
	// def'in düzeltmesi yalnız sözcüğü değiştirir; gövde ve ':' de değişmeli, uygulanabilir değil.
	errs := parseErrs("def f(x):\n    TR_return x")
	if !hasErrAt(errs, 1, 1, messages.H0258) {
		t.Errorf("1:1'de H0258 bekleniyordu: %v", errs)
	} else if pos, _ := messages.Find(errs[0]); len(pos.Fixes) != 1 || pos.Fixes[0].Applicable || pos.Fixes[0].Replacement != "TR_func" {
		t.Errorf("def → TR_func önerisi bekleniyordu: %+v", pos.Fixes)
	}
	for _, d := range []struct {
		code, at string
		want     messages.Code
	}{
		{"@f\nTR_func g() { }", "@", messages.H0259},
		{"TR_func f(a: TR_Int = 1, b: TR_Int) { }", "b:", messages.H0261},
		{"TR_var g = (a: TR_Int = 1, b: TR_Int) => a", "b:", messages.H0261},
	} {
		line, col := posOf(t, d.code, d.at)
		if errs := parseErrs(d.code); !hasErrAt(errs, line, col, d.want) {
			t.Errorf("%q: %d:%d'de %s bekleniyordu, alınan: %v", d.code, line, col, d.want, errs)
		}
	}
	// İkinci tanımın hatası adındadır, ilgili konum ilk tanım; başka blokta ve iç içe kapsamda aynı ad serbest.
	code := "TR_func f() { }\nTR_func g() {\n    TR_func f() { }\n}\nTR_func f(x: TR_Int) { }"
	errs = parseErrs(code)
	if len(errs) != 1 || !hasErrAt(errs, 5, 9, messages.H0260) {
		t.Fatalf("5:9'da tek H0260 bekleniyordu: %v", errs)
	}
	pos, _ := messages.Find(errs[0])
	if len(pos.Related) != 1 || pos.Related[0].Line != 1 || pos.Related[0].Col != 9 {
		t.Errorf("ilgili konum ilk tanım (1:9) olmalı: %+v", pos.Related)
	}
	if errs := parseErrs("TR_func f(a: TR_Int, b: TR_Int = 2) -> TR_Int { TR_return a + b }"); len(errs) != 0 {
		t.Errorf("doğru yazım hata vermemeli: %v", errs)
	}
}

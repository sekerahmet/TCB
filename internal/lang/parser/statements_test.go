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

package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// checkBodyIn lang'de (nil: kodlu gövde) yazılmış kodu ayrıştırıp denetler; denetimin hatalarını verir.
func checkBodyIn(t *testing.T, code string, lang *codes.Language, interactive bool) []error {
	t.Helper()
	l := lexer.NewBody(code, true)
	if lang != nil {
		l = lexer.NewBodyIn(code, false, lang)
	}
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errs()) > 0 {
		t.Fatalf("%q: %v", code, p.Errs())
	}
	c := New()
	if interactive {
		c.SetInteractive()
	}
	c.CheckProgram(prog)
	return c.Errs()
}

// Çağrı içermeyen, değeri kullanılmayan ifade denetim hatasıdır (2.8): satır başındaki '-' önceki
// satırı sürdürmez, bu yüzden 'a = 1' ardından gelen '- b' bölünmüş bir formülün kopan parçasıdır.
// İkili biçimi de olan işleçle başlayan satırın iletisi işleci önceki satırın sonuna taşımayı söyler
// (H1017); öbürleri değeri bir değişkene atamayı ya da satırı silmeyi (H1023). Çağrı (x |> f de)
// deyimdir. Kabukta (etkileşimli) ifadenin değeri gösterildiği için hata yoktur.
func TestUnusedValueIsAnError(t *testing.T) {
	errs := checkBodyIn(t, "TR_var b = 2\nTR_var a = 1\n    - b", nil, false)
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1017) {
		t.Fatalf("H1017 bekleniyordu: %v", errs)
	}
	if line, _, _ := messages.Pos(errs[0]); line != 3 {
		t.Errorf("hata 3. satırda olmalı: %v", errs[0])
	}
	errs = checkBodyIn(t, "TR_var k = {ad: 1}\nk.ad", nil, false)
	if len(errs) != 1 || !messages.Has(errs[0], messages.H1023) {
		t.Fatalf("H1023 bekleniyordu: %v", errs)
	}
	if line, _, _ := messages.Pos(errs[0]); line != 2 {
		t.Errorf("hata 2. satırda olmalı: %v", errs[0])
	}
	for _, code := range []string{
		"TR_var b = 2\nTR_var a = 1 -\n    b\nTR_var c = -b",
		"TR_var l = [1]\nl.TR_append(2)",
		"TR_var x = 1\nTR_str(x + 2)",
		"TR_func f() { TR_return 1 }\n-f()",
	} {
		if errs := checkBodyIn(t, code, nil, false); len(errs) != 0 {
			t.Errorf("kullanılan değerde ya da çağrıda hata olmamalı: %q: %v", code, errs)
		}
	}
	if errs := checkBodyIn(t, "TR_var b = 2\n-b", nil, true); len(errs) != 0 {
		t.Errorf("kabukta hata olmamalı: %v", errs)
	}
}

// Değeri kullanılan bir bloğun son ifadesi değerdir: 'TR_var g = TR_if a { 0.01 } TR_else { -0.01 }'
// bloğun değeridir (iç içe eğer ve eşle de); değeri kullanılmayan eğer deyiminin bloğundaki '-x'
// hata olmaya devam eder. Bir dildeki yazım (kabuk satırı) önce kodlanır, denetleyici kodlu gövdeyi okur.
func TestBlockValueIsNotAnUnusedValue(t *testing.T) {
	for _, c := range []struct {
		lang   *codes.Language
		used   []string
		unused string
	}{
		{nil, []string{
			"TR_var a = TR_true\nTR_var g = TR_if a { 0.01 } TR_else { -0.01 }\nTR_print(g)",
			"TR_var a = TR_true\nTR_print(TR_if a { 1 } TR_else TR_if TR_not a { -1 } TR_else { TR_if a { -2 } TR_else { -3 } })",
			"TR_var a = 1\nTR_print(TR_match a { 1 => { -1 }; _ => { -2 } })",
		}, "TR_var a = TR_true\nTR_var b = 1\nTR_if a {\n    -b\n}"},
	} {
		name := "kodlu"
		if c.lang != nil {
			name = c.lang.Code
		}
		for _, code := range c.used {
			if errs := checkBodyIn(t, code, c.lang, false); len(errs) != 0 {
				t.Errorf("%s: değeri kullanılan bloğun son ifadesi hata vermemeli: %q: %v", name, code, errs)
			}
		}
		if errs := checkBodyIn(t, c.unused, c.lang, false); len(errs) != 1 || !messages.Has(errs[0], messages.H1017) {
			t.Errorf("%s: değeri kullanılmayan eğer deyimindeki '-b' hata vermeli: %v", name, errs)
		}
	}
}

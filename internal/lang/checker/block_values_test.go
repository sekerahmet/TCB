package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

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
	w := check("TR_var b = 2\nTR_var a = 1\n    - b", false)
	if len(w) != 1 || !messages.Has(w[0], messages.H1011) {
		t.Fatalf("H1011 uyarısı bekleniyordu: %v", w)
	}
	if line, _, _ := messages.Pos(w[0]); line != 3 {
		t.Errorf("uyarı 3. satırda olmalı: %v", w[0])
	}
	if w := check("TR_var b = 2\nTR_var a = 1 -\n    b\nTR_var c = -b", false); len(w) != 0 {
		t.Errorf("kullanılan değerde uyarı olmamalı: %v", w)
	}
	if w := check("TR_var b = 2\n-b", true); len(w) != 0 {
		t.Errorf("kabukta uyarı olmamalı: %v", w)
	}
}

// Değeri kullanılan bir bloğun son ifadesi etkisiz değildir: 'değişken g = eğer a { 0.01 } değilse
// { -0.01 }' bloğun değeridir (iç içe eğer ve eşle de); değeri kullanılmayan eğer deyiminin
// bloğundaki '-x' uyarmaya devam eder. İki yazım dilinde aynı.
func TestBlockValueIsNotAnUnusedNegation(t *testing.T) {
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	warnings := func(code string, lang *codes.Language) []error {
		p := parser.New(lexer.NewBodyIn(code, false, lang))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("%q: %v", code, p.Errors())
		}
		c := New()
		c.CheckProgram(prog)
		return c.Warnings()
	}
	for _, c := range []struct {
		lang   *codes.Language
		used   []string
		unused string
	}{
		{codes.Source(), []string{
			"TR_var a = TR_true\nTR_var g = TR_if a { 0.01 } TR_else { -0.01 }\nTR_print(g)",
			"TR_var a = TR_true\nTR_print(TR_if a { 1 } TR_else TR_if TR_not a { -1 } TR_else { TR_if a { -2 } TR_else { -3 } })",
			"TR_var a = 1\nTR_print(TR_match a { 1 => { -1 }; _ => { -2 } })",
		}, "TR_var a = TR_true\nTR_var b = 1\nTR_if a {\n    -b\n}"},
		{en, []string{
			"var a = true\nvar g = if a { 0.01 } else { -0.01 }\nprint(g)",
			"var a = true\nprint(if a { 1 } else if not a { -1 } else { if a { -2 } else { -3 } })",
			"var a = 1\nprint(match a { 1 => { -1 }; default => { -2 } })",
		}, "var a = true\nvar b = 1\nif a {\n    -b\n}"},
	} {
		for _, code := range c.used {
			if w := warnings(code, c.lang); len(w) != 0 {
				t.Errorf("%s: değeri kullanılan bloğun son ifadesi uyarmamalı: %q: %v", c.lang.Code, code, w)
			}
		}
		if w := warnings(c.unused, c.lang); len(w) != 1 || !messages.Has(w[0], messages.H1011) {
			t.Errorf("%s: değeri kullanılmayan eğer deyimindeki '-b' uyarmalı: %v", c.lang.Code, w)
		}
	}
}

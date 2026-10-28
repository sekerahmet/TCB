package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// recordsOf checks a coded program and returns the error and warning records with their places.
func recordsOf(t *testing.T, source string) (errs, warns []*messages.PosError) {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errs()) != 0 {
		t.Fatalf("%q: syntax errors %v", source, p.Errs())
	}
	c := New()
	c.CheckProgram(prog)
	for _, e := range c.Errs() {
		pos, ok := messages.Find(e)
		if !ok {
			t.Fatalf("%q: an error without a place: %v", source, e)
		}
		errs = append(errs, pos)
	}
	for _, w := range c.Warnings() {
		pos, ok := messages.Find(w)
		if !ok {
			t.Fatalf("%q: a warning without a place: %v", source, w)
		}
		warns = append(warns, pos)
	}
	return errs, warns
}

// rangeAt is the range of the first occurrence of needle on the given line (1-based) of source:
// columns count characters and the end is exclusive.
func rangeAt(t *testing.T, source string, line int, needle string) messages.Range {
	t.Helper()
	text := strings.Split(source, "\n")[line-1]
	i := strings.Index(text, needle)
	if i < 0 {
		t.Fatalf("%q is not on line %d of %q", needle, line, source)
	}
	col := len([]rune(text[:i])) + 1
	return messages.Range{Line: line, Col: col, EndLine: line, EndCol: col + len([]rune(needle))}
}

// Every check puts its error or warning at the node or name it is about, with a start column and an
// end (2.13); the text shows the column too.
func TestCheckerRecordsAreAtTheirNodes(t *testing.T) {
	cases := []struct {
		name   string
		source string
		code   messages.Code
		line   int
		needle string
	}{
		{"type mismatch is at the declaration", "TR_var a: TR_Int = \"x\"", messages.H1001, 1, "TR_var a: TR_Int = \"x\""},
		{"assignment to a constant is at the name", "TR_const b = 1\nb = 2", messages.H1002, 2, "b"},
		{"return type mismatch is at the return", "TR_func f(): TR_Int { TR_return \"x\" }", messages.H1003, 1, "TR_return \"x\""},
		{"argument type mismatch is at the argument", "TR_func f(x: TR_String) { TR_return x }\nf(5)", messages.H1004, 2, "5"},
		{"assignment type mismatch is at the assignment", "TR_var n: TR_Int = 1\nn = \"s\"", messages.H1005, 2, "n = \"s\""},
		{"unused value statement", "TR_var a = 1\na + 1", messages.H1023, 2, "a + 1"},
		{"look-alike name is at the second name", "TR_var m = 1\nTR_var rn = 2", messages.H1020, 2, "rn"},
		{"type compared with text is at the comparison", "TR_var t = TR_Type(1) == \"Int\"", messages.H1105, 1, "TR_Type(1) == \"Int\""},
		{"undefined name is at the name", "TR_print(prnt)", messages.H1010, 1, "prnt"},
		{"misspelled type is at the type name", "TR_var a: Tam = 1", messages.H1006, 1, "Tam"},
	}
	for _, c := range cases {
		errs, _ := recordsOf(t, c.source)
		var got *messages.PosError
		for _, e := range errs {
			if messages.Has(e, c.code) {
				got = e
			}
		}
		if got == nil {
			t.Errorf("%s: no %v among %v", c.name, c.code, errs)
			continue
		}
		want := rangeAt(t, c.source, c.line, c.needle)
		if got.Range() != want {
			t.Errorf("%s: range %+v, want %+v", c.name, got.Range(), want)
		}
		if !strings.Contains(got.Error(), "Sütun") {
			t.Errorf("%s: the text shows no column: %q", c.name, got.Error())
		}
	}
}

func TestCheckerWarningsAreAtTheirNodes(t *testing.T) {
	cases := []struct {
		name   string
		source string
		code   messages.Code
		line   int
		needle string
	}{
		{"variable name style", "TR_var MyVar = 1", messages.H1012, 1, "MyVar"},
		{"struct name style", "TR_struct point { x: TR_Int }", messages.H1014, 1, "point"},
		{"field name style", "TR_struct Point { xValue_Total: TR_Int }", messages.H1012, 1, "xValue_Total"},
		{"parameter name style", "TR_func f(BigName) { TR_return BigName }", messages.H1012, 1, "BigName"},
		{"loop variable name style", "TR_for BigName TR_in [1] { TR_print(BigName) }", messages.H1012, 1, "BigName"},
		{"foreign interpolation is at the text", "TR_var name = 1\nTR_print(\"hi {name}\")", messages.H1022, 2, "\"hi {name}\""},
	}
	for _, c := range cases {
		_, warns := recordsOf(t, c.source)
		var got *messages.PosError
		for _, w := range warns {
			if messages.Has(w, c.code) {
				got = w
			}
		}
		if got == nil {
			t.Errorf("%s: no %v among %v", c.name, c.code, warns)
			continue
		}
		want := rangeAt(t, c.source, c.line, c.needle)
		if got.Range() != want {
			t.Errorf("%s: range %+v, want %+v", c.name, got.Range(), want)
		}
	}
}

// A name warning carries the fix that renames the place it is at, and the fix is never applicable
// as it stands: a name is right only when all its uses change with it.
func TestNameStyleFixIsASuggestion(t *testing.T) {
	_, warns := recordsOf(t, "TR_var MyVar = 1")
	if len(warns) != 1 || len(warns[0].Fixes) != 1 {
		t.Fatalf("warnings %+v", warns)
	}
	f := warns[0].Fixes[0]
	if f.Replacement != "my_var" || f.Applicable || f.Range != warns[0].Range() {
		t.Errorf("fix %+v at %+v", f, warns[0].Range())
	}
}

// The closest name is a guess (TR_str is closest to TR_std and means TR_String): the fix names it and
// is not applicable.
func TestUndefinedNameFixIsASuggestion(t *testing.T) {
	errs, _ := recordsOf(t, "TR_print(prnt)")
	if len(errs) != 1 || len(errs[0].Fixes) != 1 {
		t.Fatalf("errors %+v", errs)
	}
	f := errs[0].Fixes[0]
	if f.Replacement != "TR_print" || f.Applicable || f.Range != errs[0].Range() {
		t.Errorf("fix %+v at %+v", f, errs[0].Range())
	}
}

// Two look-alike names point at the first one as the related place.
func TestSkeletonErrorRelatesToTheFirstName(t *testing.T) {
	source := "TR_var m = 1\nTR_var rn = 2"
	errs, _ := recordsOf(t, source)
	if len(errs) != 1 || len(errs[0].Related) != 1 {
		t.Fatalf("errors %+v", errs)
	}
	rel := errs[0].Related[0]
	if rel.Range != rangeAt(t, source, 1, "m") || !messages.Has(rel.Err, messages.H8300) || rel.File != "" {
		t.Errorf("related %+v", rel)
	}
}

// A fix puts the '$' escape back only for a name that was written with it. The escape exists in a text
// read in a writing language (a file shown in a language, a shell line); a coded file has none, so a name
// there is replaced by the plain name, also when its range is longer than its characters (a name written
// in decomposed form: 'e' and a combining accent are two characters, the name has one).
func TestNameFixesPutBackTheDollarOnlyWhenItWasWritten(t *testing.T) {
	decomposed := "e\u0301"
	cases := []struct {
		name   string
		lexer  func(string) *lexer.Lexer
		source string
		code   messages.Code
		want   string
	}{
		{"escaped name in a shown file", func(s string) *lexer.Lexer { return lexer.NewBodyIn(s, false, codes.Source()) },
			"TR_var $MyVar = 1", messages.H1012, "$my_var"},
		{"plain name in a shown file", func(s string) *lexer.Lexer { return lexer.NewBodyIn(s, false, codes.Source()) },
			"TR_var MyVar = 1", messages.H1012, "my_var"},
		{"name in a coded file", func(s string) *lexer.Lexer { return lexer.NewBody(s, true) },
			"TR_var MyVar = 1", messages.H1012, "my_var"},
		{"decomposed undefined name in a coded file", func(s string) *lexer.Lexer { return lexer.NewBody(s, true) },
			"TR_var " + decomposed + "b = 1\nTR_print(" + decomposed + "c)", messages.H1010, "éb"}, // the name is kept composed, the file has it decomposed
	}
	for _, c := range cases {
		p := parser.New(c.lexer(c.source))
		prog := p.ParseProgram()
		if len(p.Errs()) != 0 {
			t.Fatalf("%s: syntax errors %v", c.name, p.Errs())
		}
		chk := New()
		chk.CheckProgram(prog)
		var got *messages.PosError
		for _, e := range append(chk.Errs(), chk.Warnings()...) {
			if pos, ok := messages.Find(e); ok && messages.Has(e, c.code) && len(pos.Fixes) > 0 {
				got = pos
			}
		}
		if got == nil {
			t.Errorf("%s: no %v with a fix among %v %v", c.name, c.code, chk.Errs(), chk.Warnings())
			continue
		}
		if f := got.Fixes[0]; f.Replacement != c.want || f.Applicable {
			t.Errorf("%s: fix %+v, want replacement %q not applicable", c.name, f, c.want)
		}
	}
}

package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// A token ends after its last character as it is written in the file (an identity TR_var counts
// its eight characters, not the length of the word it names; a text counts its quotes and escapes,
// not its value), and the end is exclusive (2.13).
func TestTokenRangeEnds(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []messages.Range // the tokens before the end of file
	}{
		{"names and numbers", "TR_var ağırlık = 1.5", []messages.Range{
			{Line: 1, Col: 1, EndLine: 1, EndCol: 7},
			{Line: 1, Col: 8, EndLine: 1, EndCol: 15},
			{Line: 1, Col: 16, EndLine: 1, EndCol: 17},
			{Line: 1, Col: 18, EndLine: 1, EndCol: 21},
		}},
		{"two-character operator", "a >= 0x1F", []messages.Range{
			{Line: 1, Col: 1, EndLine: 1, EndCol: 2},
			{Line: 1, Col: 3, EndLine: 1, EndCol: 5},
			{Line: 1, Col: 6, EndLine: 1, EndCol: 10},
		}},
		{"text with an escape", `x = "a\tb"`, []messages.Range{
			{Line: 1, Col: 1, EndLine: 1, EndCol: 2},
			{Line: 1, Col: 3, EndLine: 1, EndCol: 4},
			{Line: 1, Col: 5, EndLine: 1, EndCol: 11},
		}},
		{"token before a newline", "a\nb", []messages.Range{
			{Line: 1, Col: 1, EndLine: 1, EndCol: 2},
			{Line: 2, Col: 1, EndLine: 2, EndCol: 2},
		}},
		{"raw string over two lines", "`ab\ncd` x", []messages.Range{
			{Line: 1, Col: 1, EndLine: 2, EndCol: 4},
			{Line: 2, Col: 5, EndLine: 2, EndCol: 6},
		}},
	}
	for _, c := range cases {
		toks, errs := lexTokens(t, NewBody(c.source, true), c.source)
		if len(errs) != 0 {
			t.Errorf("%s: lexer errors %v", c.name, errs)
			continue
		}
		var got []messages.Range
		for _, tok := range toks {
			if tok.Type != TokEOF {
				got = append(got, tok.Range())
			}
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: %d tokens, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: token %d range %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestTokenRangeOfATokenWithoutAnEnd(t *testing.T) {
	cases := []struct {
		tok  Token
		want messages.Range
	}{
		{Token{Line: 2, Column: 4, Literal: "abc"}, messages.Range{Line: 2, Col: 4, EndLine: 2, EndCol: 7}},
		{Token{Line: 2, Column: 4}, messages.Range{Line: 2, Col: 4, EndLine: 2, EndCol: 5}},
		{Token{Line: 2}, messages.Range{Line: 2}},
	}
	for _, c := range cases {
		if got := c.tok.Range(); got != c.want {
			t.Errorf("%+v.Range() = %+v, want %+v", c.tok, got, c.want)
		}
	}
}

// An error ends where the lexer stood when it was raised: the text it is about is read by then (a
// string that is never closed is read to the end of the file).
func TestLexerErrorEnds(t *testing.T) {
	cases := []struct {
		name   string
		source string
		code   messages.Code
		want   messages.Range
	}{
		{"unclosed text runs to the end of the file", "x = \"abc\ny = 1", messages.H0010,
			messages.Range{Line: 1, Col: 5, EndLine: 2, EndCol: 6}},
		{"number with a capital prefix", "x = 0X1F + 1", messages.H0018,
			messages.Range{Line: 1, Col: 5, EndLine: 1, EndCol: 9}},
	}
	for _, c := range cases {
		_, errs := lexTokens(t, NewBody(c.source, true), c.source)
		found := false
		for _, e := range errs {
			if p, ok := messages.Find(e); ok && messages.Has(e, c.code) {
				found = true
				if got := p.Range(); got != c.want {
					t.Errorf("%s: range %+v, want %+v", c.name, got, c.want)
				}
			}
		}
		if !found {
			t.Errorf("%s: no %v among %v", c.name, c.code, errs)
		}
	}
}

// 0X1F is fixed by 0x1F, and the fix applies as it stands; when the literal has another fault the
// small prefix alone does not make it right, so the fix is only a suggestion.
func TestCapitalPrefixFix(t *testing.T) {
	cases := []struct {
		source     string
		replace    string
		applicable bool
		end        int
	}{
		{"x = 0X1F + 1", "0x1F", true, 9},
		{"x = 0B101", "0b101", true, 10},
		{"x = 0X1G", "0x1G", false, 9},
	}
	for _, c := range cases {
		_, errs := lexTokens(t, NewBody(c.source, true), c.source)
		var fixes []messages.Fix
		for _, e := range errs {
			if p, ok := messages.Find(e); ok && messages.Has(e, messages.H0018) {
				fixes = p.Fixes
			}
		}
		if len(fixes) != 1 {
			t.Errorf("%q: %d fixes among %v", c.source, len(fixes), errs)
			continue
		}
		want := messages.Fix{Range: messages.Range{Line: 1, Col: 5, EndLine: 1, EndCol: c.end}, Replacement: c.replace, Applicable: c.applicable}
		if fixes[0] != want {
			t.Errorf("%q: fix %+v, want %+v", c.source, fixes[0], want)
		}
	}
}

// A suggested identity is a guess (TR_str has the single closest identity TR_std, but means
// TR_String): the fix replaces the name and is never applicable as it stands.
func TestIdentitySuggestionFixIsNotApplicable(t *testing.T) {
	for _, source := range []string{"x = TR_pritn(1)", "x = TR_str(1)", "TR_var TR_x = 1"} {
		_, errs := lexTokens(t, NewBody(source, true), source)
		if len(errs) != 1 {
			t.Fatalf("%q: errors %v", source, errs)
		}
		p, ok := messages.Find(errs[0])
		if !ok || len(p.Fixes) != 1 {
			t.Fatalf("%q: fixes %+v", source, p)
		}
		f := p.Fixes[0]
		if f.Applicable || f.Replacement == "" || f.Range != p.Range() {
			t.Errorf("%q: fix %+v at %+v", source, f, p.Range())
		}
	}
}

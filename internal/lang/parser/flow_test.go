package parser

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Döngü deseni (5.7) öğenin yerinde durur: TR_for (a, b) TR_in l ve TR_for i, (a, b) TR_in l; desenli
// döngünün tek adı (ValueVar) boştur, kaynağın metni iletiler için saklanır. Parantezli koşul
// (TR_for (x) { ... }) koşullu döngü kalır. Parantezli tek ad H0040'tır, döngü o adla okunur;
// iç içe desen yoktur (5.3'te yok), sözdizimi hatasıdır.
func TestParseLoopPattern(t *testing.T) {
	cases := []struct{ src, key, value, pattern, iterable string }{
		{"TR_for (a, b) TR_in l { }", "", "", "a b", "l"},
		{"TR_for i, (a, _, c) TR_in f(l, 2) { }", "i", "", "a _ c", "f(l, 2)"},
		{"TR_for i, x TR_in l { }", "i", "x", "", "l"},
		{"TR_for x TR_in 0..3 { }", "", "x", "", "0..3"},
	}
	for _, c := range cases {
		p := New(codedLexer(c.src))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Errorf("%q: %v", c.src, p.Errors())
			continue
		}
		s, ok := prog.Statements[0].(*ast.ForInStatement)
		if !ok {
			t.Errorf("%q: döngü değil: %T", c.src, prog.Statements[0])
			continue
		}
		var pattern []string
		for _, n := range s.Pattern {
			pattern = append(pattern, n.Name)
		}
		if s.KeyVar != c.key || s.ValueVar != c.value || strings.Join(pattern, " ") != c.pattern || s.IterableToken.Literal != c.iterable {
			t.Errorf("%q: anahtar %q, öğe %q, desen %v, kaynak %q", c.src, s.KeyVar, s.ValueVar, pattern, s.IterableToken.Literal)
		}
	}

	p := New(codedLexer("TR_var x = TR_true\nTR_for (x) { TR_break }"))
	prog := p.ParseProgram()
	if _, ok := prog.Statements[1].(*ast.WhileStatement); !ok || len(p.Errors()) > 0 {
		t.Errorf("parantezli koşul koşullu döngü kalmalı: %T %v", prog.Statements[1], p.Errors())
	}

	for src, head := range map[string]string{
		"TR_for (a) TR_in l { }":       "a",
		"TR_for i, (a) TR_in f(l) { }": "i, a",
	} {
		p := New(codedLexer(src))
		prog := p.ParseProgram()
		errs := p.Errs()
		if len(errs) != 1 || !messages.Has(errs[0], messages.H0040) {
			t.Errorf("%q: tek bir H0040 bekleniyordu: %v", src, errs)
			continue
		}
		if !strings.Contains(errs[0].Error(), "TR_for "+head+" TR_in ") {
			t.Errorf("%q: öneri %q başını taşımıyor: %v", src, head, errs[0])
		}
		if s, ok := prog.Statements[0].(*ast.ForInStatement); !ok || s.ValueVar != "a" || s.Pattern != nil {
			t.Errorf("%q: döngü tek adla okunmalı: %#v", src, prog.Statements[0])
		}
	}

	if errs := parseErrs("TR_for ((a, b), c) TR_in l { }"); len(errs) == 0 || hasCode(errs, messages.H0040) {
		t.Errorf("iç içe desen H0040 dışında bir sözdizimi hatası olmalı: %v", errs)
	}
}

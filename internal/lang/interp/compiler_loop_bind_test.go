package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// loopError programın ilk hatasıdır: sözdizimi, denetim ya da çalışma; hata yoksa nil.
func loopError(t *testing.T, src string) error {
	t.Helper()
	p := parser.New(bodyLexer(src))
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) > 0 {
		return errs[0]
	}
	c := checker.New()
	c.CheckProgram(prog)
	if errs := c.Errs(); len(errs) > 0 {
		return errs[0]
	}
	_, _, err := runTROut(t, src)
	return err
}

// loopSuggestion iletinin önerdiği döngü başlığıdır: H0040 ve H2731'de son ters tırnaklı yazım,
// H1026 ve H2709'da sondaki parantezin içindeki örnek.
func loopSuggestion(t *testing.T, err error) string {
	t.Helper()
	text := err.Error()
	if messages.Has(err, messages.H1026) || messages.Has(err, messages.H2709) {
		i := strings.LastIndex(text, "("+codes.Spelling(codes.KwFor)+" ")
		if i < 0 {
			t.Fatalf("iletide örnek yok: %s", text)
		}
		depth := 0
		for j := i; j < len(text); j++ {
			switch text[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return text[i+1 : j]
				}
			}
		}
		t.Fatalf("örneğin parantezi kapanmıyor: %s", text)
	}
	end := strings.LastIndex(text, "`")
	start := strings.LastIndex(text[:end], "`")
	if start < 0 {
		t.Fatalf("iletide ters tırnaklı öneri yok: %s", text)
	}
	return text[start+1 : end]
}

// Her önerinin kopyası çalışır (5.7): iletinin önerdiği döngü başlığı programdaki başlığın yerine
// konunca program sözdiziminden, denetimden ve çalışmadan hatasız geçer ve beklenen çıktıyı verir.
// Tipi bilinmeyen kaynak (fonksiyon değerinin dönüşü) hatayı çalışırken aynı kodla verir.
func TestLoopPatternSuggestionsRun(t *testing.T) {
	cases := []struct {
		code                      messages.Code
		setup, header, body, want string
	}{
		{messages.H0040, "TR_var l = [(1, 2)]", "TR_for (a) TR_in l", "TR_print(a)", "(1, 2)\n"},
		{messages.H0040, "TR_var l = [(1, 2)]", "TR_for i, (a) TR_in l", "TR_print(i, a)", "0 (1, 2)\n"},
		{messages.H2731, "TR_var m = [[1, 2]]", "TR_for (a, b) TR_in m", "TR_print(a)", "[1, 2]\n"},
		{messages.H2731, "TR_var m = [3]", "TR_for i, (a, _) TR_in m", "TR_print(i, a)", "0 3\n"},
		{messages.H2731, "TR_var g = (n: TR_Int) => [[n, n]]", "TR_for (a, b) TR_in g(1)", "TR_print(a)", "[1, 1]\n"},
		{messages.H1026, "TR_var l = [(1, 2)]", "TR_for (a, b, c) TR_in l", "TR_print(a, b)", "1 2\n"},
		{messages.H1026, "TR_var l = [(1, 2, 3)]", "TR_for i, (a, b) TR_in l", "TR_print(i, a, b)", "0 1 2\n"},
		{messages.H2709, "TR_var h = (n: TR_Int) => [(n, n, n)]", "TR_for (a, b) TR_in h(1)", "TR_print(a, b)", "1 1\n"},
	}
	for _, c := range cases {
		program := func(header string) string { return c.setup + "\n" + header + " {\n    " + c.body + "\n}\n" }
		err := loopError(t, program(c.header))
		if !messages.Has(err, c.code) {
			t.Errorf("%s: %s bekleniyordu, alınan: %v", c.header, c.code, err)
			continue
		}
		fixed := loopSuggestion(t, err)
		if err := loopError(t, program(fixed)); err != nil {
			t.Errorf("%s: öneri %q hata verdi: %v", c.header, fixed, err)
			continue
		}
		if out, _, _ := runTROut(t, program(fixed)); out != c.want {
			t.Errorf("%s: öneri %q çıktısı %q, beklenen %q", c.header, fixed, out, c.want)
		}
	}
}

// Desen tipi bilinmeyen öğeyi çalışırken açar: demetin öğeleri adlara gider; demet olmayan öğe H2731,
// sayısı tutmayan demet H2709 (denetimde yakalanmayan yol). Desen sözlükte tek adın yerinde anahtarı,
// indeksli biçimde değeri açar; döngü değişkeninin yerine her tur yeni değer gelir.
func TestLoopPatternUnpacksAtRunTime(t *testing.T) {
	out, _, err := runTROut(t, `TR_var h = (n: TR_Int) => [(n, n * 10), (n + 1, n * 20)]
TR_for i, (a, b) TR_in h(1) {
    TR_print(i, a, b)
}
TR_var d = {(1, 2): (3, 4)}
TR_for (a, b) TR_in d {
    TR_print(a, b)
}
TR_for k, (a, _) TR_in d {
    TR_print(k, a)
}`)
	if err != nil || out != "0 1 10\n1 2 20\n1 2\n(1, 2) 3\n" {
		t.Errorf("çıktı %q, hata %v", out, err)
	}
	for src, code := range map[string]messages.Code{
		"TR_var g = (n: TR_Int) => [n]\nTR_for (a, b) TR_in g(1) { TR_print(a) }":         messages.H2731,
		"TR_var g = (n: TR_Int) => [(n, n, n)]\nTR_for (a, b) TR_in g(1) { TR_print(a) }": messages.H2709,
	} {
		if _, _, err := runTROut(t, src); !messages.Has(err, code) {
			t.Errorf("%q: %s bekleniyordu, alınan: %v", src, code, err)
		}
	}
}

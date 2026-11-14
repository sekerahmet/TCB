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

// loopSuggestion iletinin önerdiği döngü başlığıdır: H0040, H2730 ve H2731'de sondan back'inci
// (0: son) ters tırnaklı yazım, H1026 ve H2709'da sondaki parantezin içindeki örnek.
func loopSuggestion(t *testing.T, err error, back int) string {
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
	parts := strings.Split(text, "`")
	if i := len(parts) - 2 - 2*back; i >= 1 && len(parts)%2 == 1 {
		return parts[i]
	}
	t.Fatalf("iletide sondan %d. ters tırnaklı öneri yok: %s", back, text)
	return ""
}

// Her önerinin kopyası çalışır (5.7): iletinin önerdiği döngü başlığı programdaki başlığın yerine
// konunca program sözdiziminden, denetimden ve çalışmadan hatasız geçer ve beklenen çıktıyı verir.
// Tipi bilinmeyen kaynak (fonksiyon değerinin dönüşü) hatayı çalışırken aynı kodla verir. H2730'un
// iki önerisi (indekssiz desen, indeksli desen) ayrı ayrı denenir; indeksin adı desendekilerden
// ayrıdır (desende i varken j). H0040'ın iki yolu (ayrıştırıcı öğenin tipini bilmez): demet olmayan
// öğede parantezsiz ad, demette (a, _) deseni; üç öğeli demette desen H1026 verir ve onun önerisi
// (then) doğru sayıya götürür (TCB 13:29).
func TestLoopPatternSuggestionsRun(t *testing.T) {
	cases := []struct {
		code                      messages.Code
		back                      int
		setup, header, body, want string
		then                      messages.Code
	}{
		{messages.H0040, 1, "TR_var l = [(1, 2)]", "TR_for (a) TR_in l", "TR_print(a)", "(1, 2)\n", 0},
		{messages.H0040, 0, "TR_var l = [(1, 2)]", "TR_for (a) TR_in l", "TR_print(a)", "1\n", 0},
		{messages.H0040, 1, "TR_var l = [5, 6]", "TR_for i, (a) TR_in l", "TR_print(i, a)", "0 5\n1 6\n", 0},
		{messages.H0040, 0, "TR_var l = [(1, 2)]", "TR_for i, (a) TR_in l", "TR_print(i, a)", "0 1\n", 0},
		{messages.H0040, 0, "TR_var l = [(1, 2, 3)]", "TR_for i, (a) TR_in l", "TR_print(i, a)", "0 1\n", messages.H1026},
		{messages.H2731, 0, "TR_var m = [[1, 2]]", "TR_for (a, b) TR_in m", "TR_print(a)", "[1, 2]\n", 0},
		{messages.H2731, 0, "TR_var m = [3]", "TR_for i, (a, _) TR_in m", "TR_print(i, a)", "0 3\n", 0},
		{messages.H2731, 0, "TR_var g = (n: TR_Int) => [[n, n]]", "TR_for (a, b) TR_in g(1)", "TR_print(a)", "[1, 1]\n", 0},
		{messages.H1026, 0, "TR_var l = [(1, 2)]", "TR_for (a, b, c) TR_in l", "TR_print(a, b)", "1 2\n", 0},
		{messages.H1026, 0, "TR_var l = [(1, 2, 3)]", "TR_for i, (a, b) TR_in l", "TR_print(i, a, b)", "0 1 2\n", 0},
		{messages.H2709, 0, "TR_var h = (n: TR_Int) => [(n, n, n)]", "TR_for (a, b) TR_in h(1)", "TR_print(a, b)", "1 1\n", 0},
		{messages.H2730, 1, "TR_var l = [(1, 2), (3, 4)]", "TR_for a, b TR_in l", "TR_print(a, b)", "1 2\n3 4\n", 0},
		{messages.H2730, 0, "TR_var l = [(1, 2), (3, 4)]", "TR_for a, b TR_in l", "TR_print(a, b)", "1 2\n3 4\n", 0},
		{messages.H2730, 1, "TR_var l = [(1, 2, 3)]", "TR_for i, x TR_in l", "TR_print(i, x)", "1 2\n", 0},
		{messages.H2730, 0, "TR_var l = [(1, 2, 3)]", "TR_for i, x TR_in l", "TR_print(i, x)", "1 2\n", 0},
		{messages.H2730, 0, "TR_var h = (n: TR_Int) => [(n, n)]", "TR_for a, b TR_in h(5)", "TR_print(a + b)", "10\n", 0},
	}
	for _, c := range cases {
		program := func(header string) string { return c.setup + "\n" + header + " {\n    " + c.body + "\n}\n" }
		err := loopError(t, program(c.header))
		if !messages.Has(err, c.code) {
			t.Errorf("%s: %s bekleniyordu, alınan: %v", c.header, c.code, err)
			continue
		}
		fixed := loopSuggestion(t, err, c.back)
		if c.then != 0 {
			err := loopError(t, program(fixed))
			if !messages.Has(err, c.then) {
				t.Errorf("%s: öneri %q için %s bekleniyordu, alınan: %v", c.header, fixed, c.then, err)
				continue
			}
			fixed = loopSuggestion(t, err, 0)
		}
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
		"TR_var g = (n: TR_Int) => [(n, n)]\nTR_for a, b TR_in g(1) { TR_print(a) }":      messages.H2730,
	} {
		if _, _, err := runTROut(t, src); !messages.Has(err, code) {
			t.Errorf("%q: %s bekleniyordu, alınan: %v", src, code, err)
		}
	}
}

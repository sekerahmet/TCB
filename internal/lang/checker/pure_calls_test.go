package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// checkFileErrs kaynağı tr check gibi dosyasıyla (SetFile) denetler: öneri dosyanın satırını verir.
func checkFileErrs(t *testing.T, source string) []error {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.SetFile(ProgramFile, source)
	c.CheckProgram(prog)
	return c.Errs()
}

// Kural (13.10): "pure" işaretli yerleşiğin (13.2–13.4'ün yeni değer veren metotları, 13.7'nin
// hesapları) çağrısı tek başına deyim olamaz; ileti atamayı önerir. Alıcı atanabilir bir adsa
// düzeltme uygulanabilir, değilse ileti biçimi "<ad> = ..." ile gösterir.
func TestPureCallValueUnused(t *testing.T) {
	cases := []struct{ source, suggestion string }{
		{"TR_var l = [3, 1, 2]\nl.TR_sort()", "`l = l.TR_sort()`"},                                       // 13.3
		{"TR_var m = \" a \"\nm.TR_strip()", "`m = m.TR_strip()`"},                                       // 13.2
		{"TR_var d = {\"a\": 1}\nd.TR_keys()", "`d = d.TR_keys()`"},                                      // 13.4
		{"TR_var x = 3.7\nTR_floor(x)", "`<ad> = TR_floor(x)`"},                                          // 13.7
		{"TR_func f() -> TR_List[TR_Int] { TR_return [2, 1] }\nf().TR_sort()", "`<ad> = f().TR_sort()`"}, // alıcı ad değil
		{"TR_const L = [2, 1]\nL.TR_reverse()", "`<ad> = L.TR_reverse()`"},                               // sabite atanmaz
	}
	for _, tc := range cases {
		errs := checkFileErrs(t, tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1115) {
			t.Errorf("tek H1115 beklenirdi, gelen %v\nkaynak:\n%s", errs, tc.source)
			continue
		}
		if text := messages.ErrorIn(errs[0], "tr"); !strings.Contains(text, tc.suggestion) {
			t.Errorf("ileti %s önerisini içermeli: %s", tc.suggestion, text)
		}
		pos, _ := messages.Find(errs[0])
		fixed := pos != nil && len(pos.Fixes) == 1 && pos.Fixes[0].Applicable
		if strings.HasPrefix(tc.suggestion, "`<ad>") == fixed {
			t.Errorf("düzeltme yalnız atanabilir adda uygulanabilir: %+v\nkaynak:\n%s", pos, tc.source)
		}
	}
	// İşaretsiz yerleşik (durum değiştiren, giriş/çıkış), atanan ya da kullanılan değer ve bloğun
	// değeri hatasızdır.
	for _, source := range []string{
		"TR_var l = [1]\nl.TR_append(2)",
		"TR_print([2, 1].TR_sort())",
		"TR_var l = [2, 1]\nl = l.TR_sort()",
		"TR_var l = [2, 1]\nTR_var s = TR_if l.TR_length > 1 { l.TR_sort() } TR_else { l }",
	} {
		noErrors(t, source)
	}
}

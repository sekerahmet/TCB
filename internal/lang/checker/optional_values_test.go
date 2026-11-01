package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (3.5): ad olmayan bir T? ifadesi isteğe bağlı olmayan T beklenen yere verilemez; her yerde
// konumun kendi kodu: tanım H1001, atama H1005, dönüş H1003, kullanıcı fonksiyonunun argümanı H1004,
// kurucu H1125, alana atama H1124, yerleşik metodun argümanı H2423, .TR_append H1123; değişmezin
// öğesinde de aynı kod; öğesi isteğe bağlı olmayan kabın indeksine atama H1135. H1001, H1003, H1005
// ve H1135'in ipucu H1130'dur: ifadenin kendi yazımıyla '??'. Ad (daraltma 4b), '??' ile açılan,
// isteğe bağlı ve TR_Any beklenen yer geçer.
func TestUnopenedOptional(t *testing.T) {
	const empty = "TR_var l: TR_List[TR_Int] = []\n"
	cases := []struct {
		name, src string
		code      messages.Code
		line      int
	}{
		{"tanım", empty + "TR_var x: TR_Int = l.TR_first", messages.H1001, 2},
		{"atama", empty + "TR_var x = 0\nx = l.TR_last", messages.H1005, 3},
		{"dönüş", "TR_func f(l: TR_List[TR_Int]) -> TR_Int {\n    TR_return l.TR_last\n}", messages.H1003, 2},
		{"argüman", empty + "TR_func g(n: TR_Int) -> TR_Int { TR_return n }\nTR_print(g([1].TR_get(5)))", messages.H1004, 3},
		{"kurucu", empty + "TR_struct P { a: TR_Int }\nTR_var p = P(l.TR_first)", messages.H1125, 3},
		{"alan", empty + "TR_struct P { a: TR_Int }\nTR_var p = P(1)\np.a = l.TR_first", messages.H1124, 4},
		{"yerleşik", empty + "TR_print([1].TR_get(l.TR_first))", messages.H2423, 2},
		{"ekle", empty + "l.TR_append(l.TR_first)", messages.H1123, 2},
		{"indeksli", "TR_var l: TR_List[TR_Int?] = [1]\nTR_var x: TR_Int = l[0]", messages.H1001, 2},
		{"liste değişmezi", empty + "TR_var k: TR_List[TR_Int] = [1, l.TR_first]", messages.H1001, 2},
		{"sözlük değişmezi", empty + "TR_var d: TR_Dict[TR_String, TR_Int] = {\"a\": l.TR_first}", messages.H1001, 2},
		{"iç içe değişmez", empty + "TR_var d: TR_List[TR_List[TR_Int]] = [[l.TR_first]]", messages.H1001, 2},
		{"demet değişmezi", empty + "TR_var d: (TR_Int, TR_Int) = (1, l.TR_first)", messages.H1001, 2},
		{"liste indeksi", empty + "TR_var m = [1]\nm[0] = l.TR_first", messages.H1135, 3},
		{"sözlük indeksi", empty + "TR_var d = {\"a\": 1}\nd[\"a\"] = l.TR_first", messages.H1135, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, tc.code, tc.line) })
	}
	noErrors(t, empty+"TR_var x: TR_Int? = l.TR_first\nTR_var y: TR_Int = l.TR_first ?? 0\nTR_var a = x\nTR_var b: TR_Int = x\nb = x")
	noErrors(t, empty+"TR_func f(v: TR_Int?) { }\nf(l.TR_first)\nTR_print(l.TR_first)\nTR_var k: TR_List[TR_Int?] = [l.TR_first]\nTR_var m: TR_List[TR_Int?] = [1]\nm[0] = l.TR_first")
	noErrors(t, "TR_struct S { n: TR_Int = 0 }\nTR_var s: S? = TR_none\nTR_var v = s?.n\nTR_var k = [1]\nk[0] = s?.n ?? 0")
}

// Kural (3.5, H1130): ipucu açılmamış ifadeyi dosyadaki yazımıyla anar, varsayılanı beklenen tipe
// göre yazar; varsayılanı yazılmayan tipte (mantık değeri, sözlük, tensör) '??'yi ve TR_none'la karşılaştırmayı
// anlatır. Yazım dosyada bilinmiyorsa ipucu yoktur.
func TestUnopenedOptionalHint(t *testing.T) {
	cases := map[string]string{
		"TR_var l: TR_List[TR_Int] = []\nTR_var x: TR_Int = l.TR_first":                                         "`l.TR_first ?? 0`",
		"TR_var l: TR_List[TR_Float] = []\nTR_var x: TR_Float = l.TR_get(2)":                                    "`l.TR_get(2) ?? 0.0`",
		"TR_var l: TR_List[TR_String] = []\nTR_var x: TR_String = l.TR_last":                                    "`l.TR_last ?? \"\"`",
		"TR_var l: TR_List[TR_Bool?] = []\nTR_var x: TR_Bool = l[0]":                                            "`l[0]` is optional",
		"TR_var l: TR_List[TR_List[TR_Int]] = []\nTR_var x: TR_List[TR_Int] = l.TR_first":                       "`l.TR_first ?? []`",
		"TR_var l: TR_List[TR_Dict[TR_String, TR_Int]] = []\nTR_var x: TR_Dict[TR_String, TR_Int] = l.TR_first": "`TR_none`",
	}
	for src, want := range cases {
		lines := strings.Split(src, "\n")
		p := parser.New(lexer.NewBody(src, true))
		prog := p.ParseProgram()
		c := New()
		c.SetFile(ProgramFile, src)
		c.CheckProgram(prog)
		errs := c.Errs()
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1130) {
			t.Errorf("%q: tek H1130 ipuçlu hata bekleniyordu, alınan %v", lines[len(lines)-1], errs)
			continue
		}
		if text := messages.ErrorIn(errs[0], "en"); !strings.Contains(text, want) {
			t.Errorf("%q: ipucunda %s yok: %s", lines[len(lines)-1], want, text)
		}
	}
	errs := check(t, "TR_var l: TR_List[TR_Int] = []\nTR_var x: TR_Int = l.TR_first")
	if len(errs) != 1 || strings.Contains(errs[0], "??") {
		t.Errorf("dosyası verilmemiş denetimde ipucu beklenmiyordu: %v", errs)
	}
}

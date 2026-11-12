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
		// 4a4: argüman (H1004), alan (H1124), kurucu (H1125), .TR_append (H1123), yerleşiğin argümanı (H2423)
		"TR_var l: TR_List[TR_Int] = []\nTR_func f(x: TR_Int) { }\nf(l.TR_first)":                      "`l.TR_first ?? 0`",
		"TR_struct P { x: TR_Int }\nTR_var l: TR_List[TR_Int] = []\nTR_var p = P(1)\np.x = l.TR_first": "`l.TR_first ?? 0`",
		"TR_struct P { x: TR_Float }\nTR_var l: TR_List[TR_Float] = []\nTR_var p = P(l.TR_last)":       "`l.TR_last ?? 0.0`",
		"TR_var l: TR_List[TR_Int] = []\nTR_var m = [1]\nm.TR_append(l.TR_first)":                      "`l.TR_first ?? 0`",
		"TR_var l: TR_List[TR_String] = []\nTR_var s = \"a,b\".TR_split(l.TR_first)":                   "`l.TR_first ?? \" \"`",
		"TR_var l: TR_List[TR_Float] = []\nTR_print(TR_abs(l.TR_first))":                               "`l.TR_first ?? 0.0`",
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

// Kural (4.7, H1030): sol yan T? ise '??'nin sağ yanının tipi tam T ya da tam T?'dir. Tam sayı
// TR_Float'ın, değişmez TR_JSON'ın yerine geçmez ('??' çalışma anında çevirmez); öğesiz [] ve {}
// beklenen liste ve sözlüğün yerine geçer; TR_none hiçbir şeyi açmaz. Özellik ve metot yazımı,
// tipli ve tipsiz bildirim aynıdır. Sağ yan T? ise sonuç T?'dir: T beklenen yere verilmez (H1001).
func TestCoalesceRightSideType(t *testing.T) {
	const (
		ints   = "TR_var l: TR_List[TR_Int] = []\n"
		floats = "TR_var f: TR_Float? = TR_none\n"
		js     = "TR_var j = TR_json_parse(\"{}\")\n"
	)
	wrong := map[string]string{
		"özellik, tipli":    ints + "TR_var x: TR_Int = l.TR_first ?? \"ab\"",
		"özellik, tipsiz":   ints + "TR_var x = l.TR_first ?? \"ab\"",
		"metot":             "TR_var d: TR_Dict[TR_String, TR_Int] = {}\nTR_var x = d.TR_get(\"k\") ?? \"ab\"",
		"TR_none":           ints + "TR_var x = l.TR_first ?? TR_none",
		"tam sayı ondalığa": floats + "TR_var x = f ?? 0",
		"tamsayı değişkeni": floats + "TR_var n = 3\nTR_var x = f ?? n",
		"JSON'a mantık":     js + "TR_var x = j.TR_get(\"k\") ?? TR_false",
		"JSON'a değişmez":   js + "TR_var x = j.TR_get(\"k\") ?? {\"a\": 1}",
		"JSON'a öğesiz {}":  js + "TR_var x = j.TR_get(\"k\") ?? {}",
		"JSON'a öğesiz []":  js + "TR_var x = j.TR_get(\"k\") ?? []",
		"mantığa metin":     "TR_var b: TR_Bool? = TR_none\nTR_var x = b ?? \"x\"",
		"öğeli değişmez":    "TR_var m: TR_List[TR_List[TR_Float]] = []\nTR_var x = m.TR_first ?? [1, 2]",
	}
	for name, src := range wrong {
		t.Run(name, func(t *testing.T) { singleError(t, src, messages.H1030, strings.Count(src, "\n")+1) })
	}
	right := []string{
		ints + "TR_var x: TR_Int = l.TR_first ?? 0",
		floats + "TR_var x: TR_Float = f ?? 0.0",
		"TR_var l: TR_List[TR_List[TR_Int]] = []\nTR_var x: TR_List[TR_Int] = l.TR_first ?? []",
		"TR_var l: TR_List[TR_Dict[TR_String, TR_Int]] = []\nTR_var x: TR_Dict[TR_String, TR_Int] = l.TR_first ?? {}",
		ints + "TR_var y: TR_Int? = TR_none\nTR_var x: TR_Int? = l.TR_first ?? y",
		js + "TR_var x: TR_JSON = j.TR_get(\"k\") ?? TR_JSON(TR_false)",
		"TR_interface Ölçen { TR_func ölç(x: TR_Float) -> TR_Float }\nTR_struct Kare { TR_func ölç(x: TR_Float) -> TR_Float { TR_return x * x } }\nTR_func kare(a: Ölçen?) -> Ölçen {\n    TR_return a ?? Kare()\n}",
	}
	for _, src := range right {
		noErrors(t, src)
	}
	// Sağ yan T?: sonuç T?, T beklenen yere verilmez.
	singleError(t, ints+"TR_var y: TR_Int? = TR_none\nTR_var x: TR_Int = l.TR_first ?? y", messages.H1001, 3)
}

// Kural (4.7, H1031): sol yanın tipi bilinen ve isteğe bağlı olmayan bir tipse '??'nin sağ yanı hiç
// kullanılmaz: denetim hatasıdır; TR_JSON da isteğe bağlı değildir (3.7a). Zincir soldan birleşir:
// ilk '??'nin sonucu T ise ikinci '??' H1031 verir, T? ise geçer. Sol yan TR_none, TR_Any ya da tipi
// bilinmeyen ad (arayüz) ise hata yoktur.
func TestCoalesceNeedsAnOptionalLeft(t *testing.T) {
	const ints = "TR_var l: TR_List[TR_Int] = []\n"
	wrong := map[string]string{
		"sayı":          "TR_var a: TR_Int = (5 ?? 6)",
		"mantık":        "TR_print(TR_false ?? 5)",
		"JSON":          "TR_var j = TR_json_parse(\"1\")\nTR_print(j ?? TR_JSON(2))",
		"zincirin T'si": ints + "TR_print(l.TR_first ?? 0 ?? 1)",
	}
	for name, src := range wrong {
		t.Run(name, func(t *testing.T) { singleError(t, src, messages.H1031, strings.Count(src, "\n")+1) })
	}
	noErrors(t, "TR_var x = TR_none ?? 2.5")
	noErrors(t, ints+"TR_var y: TR_Int? = TR_none\nTR_print(l.TR_first ?? y ?? 1)")
	noErrors(t, "TR_interface Ölçen { TR_func ölç(x: TR_Float) -> TR_Float }\nTR_func seç(a: Ölçen, b: Ölçen) -> Ölçen {\n    TR_return a ?? b\n}")
	noErrors(t, "TR_var x = "+unknownValue+" ?? 1") // tipi bilinmeyen sol yan
}

// Kural (4.7, H1032, H1033, H1031): öneri sol yanın asıl tipinden ve programın kendi yazımıyladır:
// TR_Int 0, TR_Float 0.0, TR_String "", TR_List [], TR_Bool TR_false, TR_JSON sağ yanı TR_JSON(...)
// ile çevirir, öbür tipte genel öneri; H1031 sağ yanı siler. Yazım dosyada bilinmiyorsa öneri yoktur.
func TestCoalesceSuggestion(t *testing.T) {
	cases := map[string]string{
		"TR_var l: TR_List[TR_Int] = []\nTR_var x = l.TR_first ?? \"a\"":                 "`l.TR_first ?? 0`",
		"TR_var f: TR_Float? = TR_none\nTR_var x = f ?? 0":                               "`f ?? 0.0`",
		"TR_var l: TR_List[TR_String] = []\nTR_var x = l.TR_last ?? 1":                   "`l.TR_last ?? \"\"`",
		"TR_var l: TR_List[TR_List[TR_Int]] = []\nTR_var x = l.TR_first ?? 1":            "`l.TR_first ?? []`",
		"TR_var b: TR_Bool? = TR_none\nTR_var x = b ?? 1":                                "`b ?? TR_false`",
		"TR_var j = TR_json_parse(\"{}\")\nTR_var x = j.TR_get(\"k\") ?? TR_false":       "`j.TR_get(\"k\") ?? TR_JSON(TR_false)`",
		"TR_var l: TR_List[TR_Dict[TR_String, TR_Int]] = []\nTR_var x = l.TR_first ?? 1": "`l.TR_first ?? …`",
		"TR_print(1 ?? 2)": "the right side: `1`",
		"TR_var l: TR_List[TR_Int] = []\nTR_print(l.TR_first ?? 0 ?? 1)": "the right side: `l.TR_first ?? 0`",
	}
	for src, want := range cases {
		p := parser.New(lexer.NewBody(src, true))
		prog := p.ParseProgram()
		c := New()
		c.SetFile(ProgramFile, src)
		c.CheckProgram(prog)
		errs := c.Errs()
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1030) && !messages.Has(errs[0], messages.H1031) {
			t.Errorf("%q: tek H1030 ya da H1031 bekleniyordu, alınan %v", src, errs)
			continue
		}
		if text := messages.ErrorIn(errs[0], "en"); !strings.Contains(text, want) {
			t.Errorf("%q: öneride %s yok: %s", src, want, text)
		}
	}
	noHint := map[string]string{
		"TR_var l: TR_List[TR_Int] = []\nTR_var x = l.TR_first ?? \"a\"": "l.TR_first ?? 0",
		"TR_print(1 ?? 2)": "`1`",
	}
	for src, hint := range noHint {
		if errs := check(t, src); len(errs) != 1 || strings.Contains(errs[0], hint) {
			t.Errorf("dosyası verilmemiş denetimde öneri (%s) beklenmiyordu: %v", hint, errs)
		}
	}
}

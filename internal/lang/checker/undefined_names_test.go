package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (5.14): tanımsız ad çalışmadan önce hatadır; ipucu çalışma anındakiyle aynıdır. Konum
// denetlenen gövdeye göredir (kodlu gövdede TR_fn_print( 12 sütundur; kişinin görünümündeki
// konumu source.view verir, 17.5).
func TestUndefinedNameIsError(t *testing.T) {
	singleErrorAt(t, "yazdir(1)", 1, 1, "'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?")
	singleErrorAt(t, "TR_kw_var öğrenci_sayısı = 1\nTR_fn_print(öğrenci_sayis)", 2, 13, "'öğrenci_sayis' adında bir değişken ya da fonksiyon yok", "Bunu mu demek istediniz: 'öğrenci_sayısı'?")
	singleErrorAt(t, "tanımsız = 3", 1, 1, "'tanımsız'")
	singleError(t, "TR_kw_if TR_kw_true { TR_kw_var iç = 1 }\nTR_fn_print(iç)", messages.H1010, 2, "'iç'")
	singleError(t, "TR_kw_func f() { TR_kw_return g() }", messages.H1010, 1, "'g'")
	singleError(t, "TR_ctx_test \"a\" { TR_kw_var yerel = 1 }\nTR_ctx_test \"b\" { TR_fn_print(yerel) }", messages.H1010, 2, "'yerel'")
	singleError(t, `TR_fn_print("{bilinmeyen}")`, messages.H1010, 1, "'bilinmeyen'")
	singleError(t, "TR_kw_var l = [1, 2]\nTR_fn_print([x * 2 TR_kw_for x TR_kw_in l TR_kw_if y > 0])", messages.H1010, 2, "'y'")
	singleError(t, "TR_kw_self.n = 1", messages.H1010, 1, "'kendi'")
	// Aynı ad tek kez bildirilir.
	singleError(t, "TR_fn_print(a)\nTR_fn_print(a + 1)", messages.H1010, 1, "'a'")
}

// Kural (17, kitaplık kodu): kitaplık kodu yerleşik değildir; kitaplığın fonksiyon ve tip adları her
// kapsamda tanımlı sayılmaz.
func TestLibraryNameIsNotABuiltinName(t *testing.T) {
	markLibrary(t, map[codes.Code]string{codes.FnSin: "tr/deneme", codes.TypeOptimizer: "tr/deneme"})
	// Program içeride kimliklerle çalışır (17.7): kaynak kodlu gövdedir.
	for _, src := range []string{"TR_fn_print(TR_fn_sin(1))", "TR_fn_print(TR_type_Optimizer)"} {
		if errs := checkCoded(t, src); len(errs) != 1 || !messages.Has(errs[0], messages.H1010) {
			t.Errorf("%s: tek bir tanımsız ad hatası (H1010) bekleniyordu: %v", src, errs)
		}
	}
	if errs := checkCoded(t, "TR_fn_print(TR_fn_cos(1), TR_type_Int)"); len(errs) > 0 {
		t.Errorf("çekirdek adları tanımlı kalmalı: %v", errs)
	}
}

// checkCoded kodlu bir gövdeyi ayrıştırıp denetler, hataları kodlu döndürür.
func checkCoded(t *testing.T, source string) []error {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.CheckProgram(prog)
	return c.Errs()
}

// markLibrary kodları test boyunca yapay bir modülün kitaplık kodu sayar; yerleşik ad önbelleği
// yeniden kurulur.
func markLibrary(t *testing.T, modules map[codes.Code]string) {
	t.Helper()
	t.Cleanup(codes.SetLibraryModulesForTest(modules))
	builtinCache.lang = nil
	t.Cleanup(func() { builtinCache.lang = nil })
}

// singleErrorAt tek bir tanımsız ad hatası (H1010) bekler ve konumunu (satır, sütun) sınar.
// parts, hint paketinden gelen iç metindeki ad ve önerilerdir (o paket hâlâ metin döndürür).
func singleErrorAt(t *testing.T, source string, line, col int, parts ...string) {
	t.Helper()
	singleError(t, source, messages.H1010, line, parts...)
	_, coded := checkErrs(t, source)
	if len(coded) == 1 {
		if _, c, _ := messages.Pos(coded[0]); c != col {
			t.Errorf("sütun %d bekleniyordu, gelen %d: %v", col, c, coded[0])
		}
	}
}

// Dinamik dilde yanlış alarm yok: aşağıdakilerin hepsi tanımlıdır.
func TestUndefinedNameNoFalseAlarm(t *testing.T) {
	cases := map[string]string{
		"sonra tanımlanan fonksiyon":  "TR_kw_func a() { TR_kw_return b() }\nTR_kw_func b() { TR_kw_return 1 }\nTR_fn_print(a())",
		"özyineleme":                  "TR_kw_func f(n) { TR_kw_return TR_kw_if n < 1 { 0 } TR_kw_else { f(n - 1) } }",
		"sondaki blok o":              "TR_fn_print([1, 2].TR_mem_map { TR_ctx_it * 2 }.TR_mem_filter { TR_ctx_it > 2 })",
		"ok fonksiyonu":               "TR_kw_var k = (x) => x * x\nTR_fn_print(k(3), [1].TR_mem_map(y => y + 1))",
		"yakala hata":                 `TR_kw_try { TR_kw_throw "x" } TR_kw_catch { TR_fn_print(TR_ctx_error) }`,
		"yakala adlı":                 `TR_kw_try { TR_kw_throw "x" } TR_kw_catch h { TR_fn_print(h) }`,
		"döngü değişkenleri":          "TR_kw_for i, x TR_kw_in [5, 6] { TR_fn_print(i, x) }",
		"liste üreteci":               "TR_fn_print([x * x TR_kw_for x TR_kw_in 0..3 TR_kw_if x > 0])",
		"yapı ve kendi":               "TR_kw_struct Sayaç { n = 0; TR_kw_func artır(k) { TR_kw_self.n += k } }\nTR_kw_var s = Sayaç()\ns.artır(2)\nTR_fn_print(s.n)",
		"üye ve adlı argüman":         "TR_kw_var l = [3, 1]\nTR_fn_print(l.TR_mem_sort(ters: TR_kw_true).TR_mem_length)",
		"sabit ve çoklu":              "TR_kw_const PI2 = 6.28\nTR_kw_var a, b = 1, 2\na, b = b, a\nTR_fn_print(PI2, a, b)",
		"dış değişkene atama":         "TR_kw_var n = 0\nTR_kw_func art() { n += 1 }\nart()",
		"eşle":                        "TR_kw_var p = 5\nTR_kw_var s = 3\nTR_fn_print(TR_kw_match p { s => \"eşit\"\n 1..10 => \"aralık\"\n TR_kw_default => \"diğer\" })",
		"ertele ve eşzamanlı":         "TR_kw_func f() { TR_kw_var iz = []\n TR_kw_defer iz.TR_mem_append(1)\n TR_kw_async { TR_fn_print(iz) }\n TR_kw_return iz }",
		"takma adsız aktar":           "TR_kw_import \"yardimci_modul\"\nTR_fn_print(kare(PI))",
		"takma adlı aktar":            "TR_kw_import \"yardimci_modul\" TR_ctx_as ym\nTR_fn_print(ym.kare(2))",
		"ifade içinde adlı fonksiyon": "TR_kw_var l = [1].TR_mem_map(TR_kw_func ikiye(x) { TR_kw_return x * 2 })\nTR_fn_print(ikiye(3))",
		"test bloğu genel ad":         "TR_kw_var g = 1\nTR_ctx_test \"t\" { TR_fn_assert_equal(1, g) }",
		"kesme noktası koşulu":        "TR_kw_var x = 1\nTR_kw_breakpoint TR_kw_if x > 10",
		"varsayılan parametre":        "TR_kw_var taban = 2\nTR_kw_func üs(x, k = taban) { TR_kw_return x ** k }",
	}
	for name, source := range cases {
		if errs := check(t, source); len(errs) > 0 {
			t.Errorf("%s: hata beklenmiyordu: %v\n%s", name, errs, source)
		}
	}
}

// Tip hatası, tanımsız ad ve gölgeleme uyarısı birlikte bildirilir; biri ötekini gizlemez.
// Hatalar satır sırasıyladır.
func TestTypeErrorUndefinedNameAndWarningTogether(t *testing.T) {
	p := parser.New(lexer.NewBody("TR_kw_var toplam = [1, 2]\nyazdir(toplam)\nTR_kw_var x: TR_type_Int = \"metin\"\n", true))
	prog := p.ParseProgram()
	c := New()
	errs := c.CheckProgram(prog)
	coded := c.Errs()
	if len(errs) != 2 || !messages.Has(coded[0], messages.H1010) || !messages.Has(coded[1], messages.H1001) {
		t.Fatalf("tanımsız ad ve tip hatası satır sırasıyla gelmeli: %q", errs)
	}
	if l, col, _ := messages.Pos(coded[0]); l != 2 || col != 1 {
		t.Errorf("tanımsız ad konumu 2:1 olmalı, gelen %d:%d", l, col)
	}
	if l, _, _ := messages.Pos(coded[1]); l != 3 {
		t.Errorf("tip hatası 3. satırda olmalı, gelen %d", l)
	}
	// Metin biçimi: "Satır N: ..." (tanımsız adda sütun da var).
	if !strings.HasPrefix(errs[0], "Satır 2, Sütun 1: ") || !strings.HasPrefix(errs[1], "Satır 3: ") {
		t.Errorf("hata metni 'Satır N: ...' biçiminde olmalı: %q", errs)
	}
	if u := c.Warnings(); len(u) != 1 || !messages.Has(u[0], messages.H1009) || !strings.Contains(u[0].Error(), "'toplam' yerleşik bir ad") {
		t.Fatalf("gölgeleme uyarısı da verilmeli: %q", u)
	}
}

// Etkileşimli kabukta denetleyici satırlar arasında kalır: önceki satırın tanımı görünür.
func TestUndefinedNameAcrossREPLLines(t *testing.T) {
	c := New()
	line := func(source string) []string {
		p := parser.New(lexer.NewBody(source, true))
		return c.CheckProgram(p.ParseProgram())
	}
	if errs := line("TR_kw_var a = 1"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := line("TR_fn_print(a)"); len(errs) != 0 {
		t.Fatalf("önceki satırın tanımı görünmeli: %v", errs)
	}
	if errs := line("TR_fn_print(b)"); len(errs) != 1 {
		t.Fatalf("tanımsız ad bildirilmeli: %v", errs)
	}
}

// Modül aktaranın genel kapsamının altında çalışır (11.1): genel adlar modülde tanımlıdır;
// adlar bilinmiyorsa (dinamik) tanımsız ad aranmaz.
func TestUndefinedNameModuleGlobalScope(t *testing.T) {
	module := "yükleme_kaydı.TR_mem_append(1)\nTR_fn_print(bilinmeyen)"
	checkModule := func(names []string, dynamic bool) []string {
		c := New()
		c.SetGlobalScope(names, dynamic)
		return c.CheckProgram(parser.New(lexer.NewBody(module, true)).ParseModule())
	}
	errs := checkModule([]string{"yükleme_kaydı"}, false)
	if len(errs) != 1 || !strings.Contains(errs[0], "'bilinmeyen'") { // ad, ileti parametresidir
		t.Fatalf("yalnızca bilinmeyen ad bildirilmeli: %v", errs)
	}
	if errs := checkModule(nil, true); len(errs) != 0 {
		t.Fatalf("dinamik genel kapsamda tanımsız ad aranmamalı: %v", errs)
	}
	names, dynamic := TopLevelNames(parser.New(lexer.NewBody("TR_kw_var a = 1\nTR_kw_struct Y { x }\nTR_kw_func f() {}\nTR_kw_import \"m\" TR_ctx_as mm\nTR_kw_if TR_kw_true { TR_kw_var iç = 2 }", true)).ParseProgram())
	if strings.Join(names, ",") != "Y,a,f,mm" || dynamic {
		t.Fatalf("üst düzey adlar: %v %v", names, dynamic)
	}
	if _, dynamic := TopLevelNames(parser.New(lexer.NewBody(`TR_kw_import "m"`, true)).ParseProgram()); !dynamic {
		t.Fatal("takma adsız aktar adları belirsiz kılmalı")
	}
}

// Kural (B11): denetleyicinin tanımsız ad önerisi de adın yazıldığı dosyanın dilini izler: İngilizce
// dosyada prnt → 'print'.
func TestUndefinedNameSuggestionFollowsTheFileLanguage(t *testing.T) {
	p := parser.New(lexer.New("prnt(1)\n"))
	prog := p.ParseProgram()
	c := New()
	c.SetFileLanguage("en")
	c.CheckProgram(prog)
	var text []string
	for _, err := range c.Errs() {
		text = append(text, messages.ErrorIn(err, "en"))
	}
	if got := strings.Join(text, "\n"); !strings.Contains(got, "Did you mean 'print'?") {
		t.Errorf("İngilizce dosyada öneri: %s", got)
	}
}

package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (4D): ileti ve içindeki adlar tek dildedir. Denetleyici tipleri kanonik adla taşır; tip
// hatası ve tip açıklaması önerisi cümleyi iletinin dilinde yazar. Tip adı dosyada kimlik
// yazımıyladır (TR_Int; dosyanın dili yoktur, 2.1), yazılan satırda iletinin dilindedir
// (İngilizcede Int); kullanıcının sınıf adı olduğu gibi kalır.
func TestTypeNamesFollowTheMessageLanguage(t *testing.T) {
	for _, c := range []struct{ source, typed, file string }{
		{"TR_var x: TR_Int = \"a\"\n", `type "Int" but a value of type "String"`, `type "TR_Int" but a value of type "TR_String"`},
		{"TR_var x: TR_Int = 6 / 3\n", `type "Int" but a value of type "Float"`, `type "TR_Int" but a value of type "TR_Float"`},
		{"TR_var x: TR_Int = 6 / 3\n", "always gives Float", "always gives TR_Float"},
		{"TR_var x: float = 1.5\n", "write 'Float'", "write 'TR_Float'"},
		{"TR_struct Nokta {\n  x: TR_Int\n}\nTR_var p: Nokta = 1\n", `declared with type "Nokta" but a value of type "Int"`, `declared with type "Nokta" but a value of type "TR_Int"`},
	} {
		typed, _ := checkErrsIn(t, "en", c.source)
		_, errs := checkErrs(t, c.source)
		var file []string
		for _, err := range errs {
			file = append(file, messages.ErrorIn(err, "en"))
		}
		for _, x := range []struct{ got, want string }{{strings.Join(typed, "\n"), c.typed}, {strings.Join(file, "\n"), c.file}} {
			if !strings.Contains(x.got, x.want) || strings.Contains(x.got, "Tam") || strings.Contains(x.got, "Ondalık") {
				t.Errorf("%s: İngilizce iletide %q yok ya da Türkçe tip adı var:\n%s", c.source, x.want, x.got)
			}
		}
	}
}

// Kural: tip açıklaması önerisi adın yazıldığı dosyanın dilini izler. İngilizce dosyada int, Int'in
// harf yazımıdır (H1006); Türkçe dosyada Tam'ın yabancı adıdır (H1007). Başka bir yazım dilinin
// tip adı kendi dilinin adını önerir (İngilizce dosyada Tam → Int); bir dilin harf yazımı öbür
// dilde ipucu almaz.
func TestTypeAnnotationSuggestionFollowsTheFileLanguage(t *testing.T) {
	t.Skip("katman 4 (tipler): dosyanın dili kalktı (2.1); tip açıklamasının ipucu kimlik yazımıyla yeniden kurulur")
	for _, c := range []struct {
		lang, name string
		code       messages.Code // 0: H1006 de H1007 de yok
		want       string
	}{
		{"en", "int", messages.H1006, "'Int'"},
		{"en", "INT", messages.H1006, "'Int'"},
		{"en", "tam", 0, ""},
		{"en", "Tam", messages.H1007, "'Int'"},
		{"tr", "Int", messages.H1007, "'Tam'"},
		{"tr", "int", messages.H1007, "'Tam'"},
		{"tr", "ondalik", messages.H1007, "'Ondalık'"},
		{"tr", "METİN", messages.H1006, "'Metin'"},
	} {
		p := parser.New(lexer.New(codes.KwVar.String() + " a: " + c.name + "\n"))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("sözdizimi: %v", p.Errors())
		}
		chk := New()
		chk.SetFileLanguage(c.lang)
		chk.CheckProgram(prog)
		var got []string
		hinted := false
		for _, err := range chk.Errs() {
			got = append(got, messages.ErrorIn(err, c.lang))
			hinted = hinted || messages.Has(err, messages.H1006) || messages.Has(err, messages.H1007)
		}
		text := strings.Join(got, "\n")
		if c.code == 0 && hinted || c.code != 0 && (len(got) != 1 || !messages.Has(chk.Errs()[0], c.code) || !strings.Contains(text, c.want)) {
			t.Errorf("%s dosyasında %q: %s, beklenen %v %s", c.lang, c.name, text, c.code, c.want)
		}
	}
}

// Kural (3.5, 3.6, 3.6a, 3.7, 6.9, 15.1): bileşik tip bugün asıl adına indirgenir; denetleyici
// onunla yeni hata vermez. Yalnız yapı açılır (TR_List[...] → TR_List, T? → T, demet tipi her
// şeyle uyumlu, fonksiyon tipi bugünkü gibi TR_Function); isteğe bağlı tipe TR_none verilir. İçteki
// adlar bugünkü gibi denetlenir.
func TestCompoundTypesReduceToTheirBaseName(t *testing.T) {
	noErrors(t, `TR_var l: TR_List[TR_Int] = [1, 2]
TR_var bos: TR_List[TR_Int] = []
TR_var x: TR_Int? = TR_none
x = 3
x = TR_none
TR_var d: TR_Dict[TR_String, TR_List[TR_Int?]] = {"a": [1, TR_none]}
TR_var k: TR_Dict[(TR_Int, TR_Int), TR_Int] = {}
TR_var t: (TR_Int, TR_String) = TR_none
TR_var f: (TR_Int) -> TR_Bool = (a) => a > 0
TR_var g: () -> (TR_Int, TR_Int) = () => 1
TR_var tensor: TR_Tensor[TR_Int] = TR_Tensor([1, 2])
TR_func h(a: TR_List[TR_Int], b: TR_Tensor? = TR_none, c: TR_String? = TR_none) -> TR_List[TR_Int]? {
    TR_if c == TR_none { TR_return TR_none }
    TR_return a
}
TR_func m() -> (TR_Int, TR_Int) -> TR_Int { TR_return (a, b) => a + b }
TR_func ikili() -> (TR_Int, TR_String) { TR_return 1, "a" }
TR_struct S {
    a: TR_List[TR_Float]
    b: TR_String? = TR_none
}
TR_var y: TR_Int = h(l, TR_none, "c")?.TR_length ?? 0
TR_print(l, bos, x, d, k, t, f, g, tensor, h(l), m(), ikili(), S([1.0]), y)
`)
	for _, c := range []struct {
		source string
		code   messages.Code
	}{
		{"TR_var l: TR_List[TR_Int] = 1\n", messages.H1001},               // indirgenen tip bugünkü gibi denetlenir
		{"TR_var x: TR_Int? = \"a\"\n", messages.H1001},                   // T? → T
		{"TR_var f: (TR_Int) -> TR_Bool = 3\n", messages.H1001},           // fonksiyon tipi → TR_Function
		{"TR_func f() -> () -> TR_Int { TR_return 3 }\n", messages.H1003}, // fonksiyon döndüren
		{"TR_var l: TR_List[tam] = [1]\n", messages.H1006},                // içteki ad
		{"TR_func f(a: (Metin) -> TR_Bool) { }\n", messages.H1006},        // fonksiyon tipinin parametresi
	} {
		errs, coded := checkErrs(t, c.source)
		if len(coded) != 1 || !messages.Has(coded[0], c.code) {
			t.Errorf("%q: tek %v hatası bekleniyordu, gelen: %v", c.source, c.code, errs)
		}
	}
}

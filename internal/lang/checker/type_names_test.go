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
// yazımıyladır (TR_Int, TR_Int or TR_Float; dosyanın dili yoktur, 2.1), yazılan satırda iletinin
// dilindedir (İngilizcede Int, Int or Float); kullanıcının sınıf adı olduğu gibi kalır.
func TestTypeNamesFollowTheMessageLanguage(t *testing.T) {
	for _, c := range []struct{ source, typed, file string }{
		{"TR_var x: TR_Int = \"a\"\n", `type "Int" but a value of type "String"`, `type "TR_Int" but a value of type "TR_String"`},
		{"TR_var x: TR_Int = 6 / 3\n", `type "Int" but a value of type "Int or Float"`, `type "TR_Int" but a value of type "TR_Int or TR_Float"`},
		{"TR_var x: TR_Int = 6 / 3\n", "gives Float if the quotient", "gives TR_Float if the quotient"},
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

// Kural (4E): denetleyicinin iç tip kimliği Go'da yazılmış sözcük taşımaz ("Tam ya da Ondalık"
// değil); okunur ad yalnız iletide, iletinin dilinde kurulur.
func TestTypeNumberIdentityHasNoWords(t *testing.T) {
	if strings.ContainsAny(typeNumber, " \t") {
		t.Errorf("typeNumber iç kimliği sözcük taşıyor: %q", typeNumber)
	}
	if tr, en := typeText(typeNumber).MessageText("tr"), typeText(typeNumber).MessageText("en"); tr != "Tam ya da Ondalık" || en != "Int or Float" {
		t.Errorf("iletide %q / %q, beklenen \"Tam ya da Ondalık\" / \"Int or Float\"", tr, en)
	}
}

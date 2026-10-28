package hint

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Kural (ne yazılacağını söyler, öneri her zaman asıl addır): Türkçe yazılmış metinde bir yerleşiğin
// ya da anahtar kelimenin Türkçe harfsiz yazımı (yazdir) ipucu alır, ipucu asıl adı söyler; ASCII
// biçim ya da yabancı ad önerilmez. Türkçe yazım diline özgü kuraldır, dil açıkça verilir. Önerilen
// ad kod başvurusudur: iletinin dilinde yazılır (Türkçe iletide yazdır, İngilizcede print). Dilin
// her yerleşiği ve anahtar kelimesi denenir; adlar koddan gelir.
func TestAsciiSpellingSuggestsRealName(t *testing.T) {
	tr := codes.Source()
	tried := 0
	for _, k := range codes.Codes() {
		name := tr.Name(k)
		ascii := turkish.ASCII(name)
		if ascii == name {
			continue
		}
		if _, valid := tr.Find(codes.NamespaceGlobal, ascii); valid {
			continue // ASCII biçim başka bir geçerli ad
		}
		var got, want *messages.Error
		switch {
		case k.IsKeyword():
			got, want = KeywordSpellingHint(ascii, tr), messages.New(messages.H1103, ascii, codes.NameRef(k))
		case k.IsBuiltin():
			got, want = BuiltinSpellingHint(ascii, tr), NotInTRHint(ascii, codes.NameRef(k))
		default:
			continue
		}
		tried++
		if got == nil {
			t.Errorf("%s (%s): ipucu yok", k, ascii)
			continue
		}
		for _, lang := range []string{"tr", "en"} {
			g, w := messages.ErrorIn(got, lang), messages.ErrorIn(want, lang)
			if g != w {
				t.Errorf("%s (%s, %s): ipucu %q, beklenen %q", k, ascii, lang, g, w)
			}
			if s := NameSuggestion(ascii, nil, tr); s == nil || messages.ErrorIn(s, lang) != w {
				t.Errorf("%s (%s): NameSuggestion(%q) = %v, beklenen %q", k, lang, ascii, s, w)
			}
			if real := codes.NameRef(k).MessageText(lang); !strings.Contains(g, "'"+real+"'") {
				t.Errorf("%s (%s): ipucu asıl adı (%q) söylemiyor: %q", k, lang, real, g)
			}
		}
	}
	if tried == 0 {
		t.Fatal("hiç ad denenmedi")
	}
}

// Geçerli bir ad için ipucu yoktur; yazım hatası için en yakın görünen ad önerilir.
func TestValidNamesHaveNoHintAndTyposGetClosest(t *testing.T) {
	if h := BuiltinSpellingHint(codes.Name(codes.FnPrint), nil); h != nil {
		t.Errorf("geçerli ad için ipucu olmamalı: %q", h)
	}
	got := UndefinedNameError("öğrenci_sayis", []string{"öğrenci_sayısı", "toplam"}, nil).Error()
	if !strings.Contains(got, "'öğrenci_sayis'") || !strings.Contains(got, "'öğrenci_sayısı'") {
		t.Errorf("tanımsız ad iletisi yazılan adı ve en yakın adı söylemeli: %q", got)
	}
	if got := NameSuggestion("zzzzzz", []string{"toplam"}, nil); got != nil {
		t.Errorf("benzer ad yokken öneri olmamalı: %q", got)
	}
}

// Kural: öneri metnin yazıldığı dilin adını verir (dosyanın dili; nil: bilinmiyor, bütün yazım
// dilleri). Aynı yanlış yazım İngilizce metinde İngilizce, Türkçe metinde Türkçe adı bulur; bir
// dilin ASCII biçimi öbür dilde ipucu almaz.
func TestSuggestionFollowsTheTextLanguage(t *testing.T) {
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	tr := codes.Source()
	in := func(e *messages.Error, lang string) string {
		if e == nil {
			return ""
		}
		return messages.ErrorIn(e, lang)
	}
	print := codes.FnPrint.String()
	for _, c := range []struct {
		what string
		got  *messages.Error
		lang string
		want string // "": ipucu yok
	}{
		{"BuiltinSpellingHint tr", BuiltinSpellingHint("yazdir", tr), "tr", "'yazdır'"},
		{"BuiltinSpellingHint en", BuiltinSpellingHint("yazdir", en), "en", ""},
		{"BuiltinSpellingHint dosya", BuiltinSpellingHint("yazdir", nil), "tr", "'TR_print'"},
		{"KeywordSpellingHint tr", KeywordSpellingHint("degisken", tr), "tr", "'değişken'"},
		{"KeywordSpellingHint en", KeywordSpellingHint("degisken", en), "en", ""},
		{"ClosestSuggestion en", ClosestSuggestion("prnt", []string{print}, en), "en", "'print'"},
		{"ClosestSuggestion tr", ClosestSuggestion("yazdr", []string{print}, tr), "tr", "'yazdır'"},
		{"ClosestSuggestion en, Türkçe yazım", ClosestSuggestion("yazdr", []string{print}, en), "en", ""},
		{"ClosestSuggestion dosya", ClosestSuggestion("prnt", []string{print}, nil), "en", "'TR_print'"},
		{"TypeTextComparisonError en", TypeTextComparisonError("==", "Float", codes.NameRef(codes.TypeInt), en), "tr", "== Ondalık"},
		{"TypeTextComparisonError tr", TypeTextComparisonError("==", "Ondalik", codes.NameRef(codes.TypeInt), tr), "tr", "== Ondalık"},
		{"TypeTextComparisonError en, Türkçe yazım", TypeTextComparisonError("==", "Ondalik", codes.NameRef(codes.TypeInt), en), "tr", "== Tam"},
		{"TypeTextComparisonError dosya", TypeTextComparisonError("==", "Float", codes.NameRef(codes.TypeInt), nil), "tr", "== TR_Float"},
	} {
		got := in(c.got, c.lang)
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, beklenen %q", c.what, got, c.want)
		}
	}
}

// Kural (17, kitaplık kodu): kitaplık kodu ne anahtar kelime ne yerleşik ipucu alır; adı yalnız
// aktaran dosyada görünür (öneri görünen adlardan gelir).
func TestLibraryCodeGetsNoSpellingHint(t *testing.T) {
	t.Cleanup(codes.SetLibraryModulesForTest(map[codes.Code]string{codes.FnPrint: "tr/deneme"}))
	ascii := turkish.ASCII(codes.Name(codes.FnPrint))
	if i := KeywordSpellingHint(ascii, nil); i != nil {
		t.Errorf("KeywordSpellingHint(%q) = %v, ipucu beklenmiyordu", ascii, i)
	}
	if i := BuiltinSpellingHint(ascii, nil); i != nil {
		t.Errorf("BuiltinSpellingHint(%q) = %v, ipucu beklenmiyordu", ascii, i)
	}
}

// Kural (3.8, 3.12): başka dilin range(n)'i tip çağrısını değil aralık yazımını önerir (0..n,
// TR_List(0..n)); öbür tanıdık adlar TR_ yazımlarını (str → TR_String).
func TestRangeSuggestsTheRangeSpelling(t *testing.T) {
	if got := NameSuggestion("range", nil, nil); !messages.Has(got, messages.H0243) {
		t.Errorf("range: %v; want H0243", got)
	}
	if got := NameSuggestion("str", nil, nil); !messages.Has(got, messages.H1021) || !strings.Contains(got.Error(), "TR_String") {
		t.Errorf("str: %v; want H1021 suggesting TR_String", got)
	}
}

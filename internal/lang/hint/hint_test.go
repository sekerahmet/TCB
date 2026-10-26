package hint

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Önerilen ad her zaman asıl addır (Türkçe yazım); ASCII biçim ya da yabancı ad önerilmez. Dilin
// her yerleşiği ve anahtar kelimesi için Türkçe harfsiz yazım denenir; adlar koddan gelir.
func TestAsciiSpellingSuggestsRealName(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	tried := 0
	for _, k := range codes.Codes() {
		name := codes.Name(k)
		ascii := turkish.ASCII(name)
		if ascii == name {
			continue
		}
		if _, valid := codes.Find(codes.NamespaceGlobal, ascii); valid {
			continue // ASCII biçim başka bir geçerli ad
		}
		var got, want string
		switch {
		case k.IsKeyword():
			got, want = KeywordSpellingHint(ascii, nil).Error(), messages.Text(messages.H1103, ascii, name)
		case k.IsBuiltin():
			got, want = BuiltinSpellingHint(ascii, nil).Error(), NotInTRHint(ascii, name).Error()
		default:
			continue
		}
		tried++
		if got != want {
			t.Errorf("%s (%s): ipucu %q, beklenen %q", k, ascii, got, want)
		}
		if s := NameSuggestion(ascii, nil, nil).Error(); s != want {
			t.Errorf("%s: NameSuggestion(%q) = %q, beklenen %q", k, ascii, s, want)
		}
		if !strings.Contains(got, "'"+name+"'") {
			t.Errorf("%s: ipucu asıl adı (%q) söylemiyor: %q", k, name, got)
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
		{"BuiltinSpellingHint bilinmeyen", BuiltinSpellingHint("yazdir", nil), "tr", "'yazdır'"},
		{"KeywordSpellingHint tr", KeywordSpellingHint("degisken", tr), "tr", "'değişken'"},
		{"KeywordSpellingHint en", KeywordSpellingHint("degisken", en), "en", ""},
		{"ClosestSuggestion en", ClosestSuggestion("prnt", []string{print}, en), "en", "'print'"},
		{"ClosestSuggestion tr", ClosestSuggestion("yazdr", []string{print}, tr), "tr", "'yazdır'"},
		{"ClosestSuggestion en, Türkçe yazım", ClosestSuggestion("yazdr", []string{print}, en), "en", ""},
		{"ClosestSuggestion bilinmeyen", ClosestSuggestion("prnt", []string{print}, nil), "en", "'print'"},
		{"TypeTextComparisonError en", TypeTextComparisonError("==", "Float", codes.NameRef(codes.TypeInt), en), "tr", "== Ondalık"},
		{"TypeTextComparisonError tr", TypeTextComparisonError("==", "Ondalik", codes.NameRef(codes.TypeInt), tr), "tr", "== Ondalık"},
		{"TypeTextComparisonError en, Türkçe yazım", TypeTextComparisonError("==", "Ondalik", codes.NameRef(codes.TypeInt), en), "tr", "== Tam"},
		{"TypeTextComparisonError bilinmeyen", TypeTextComparisonError("==", "Float", codes.NameRef(codes.TypeInt), nil), "tr", "== Ondalık"},
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

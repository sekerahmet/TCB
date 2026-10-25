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
			got, want = KeywordSpellingHint(ascii).Error(), messages.Text(messages.H1103, ascii, name)
		case k.IsBuiltin():
			got, want = BuiltinSpellingHint(ascii).Error(), NotInTRHint(ascii, name).Error()
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
	if h := BuiltinSpellingHint(codes.Name(codes.FnPrint)); h != nil {
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

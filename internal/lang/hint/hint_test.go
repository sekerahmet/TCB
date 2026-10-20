package hint

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Önerilen ad her zaman asıl addır (Türkçe yazım); ASCII biçim ya da eski ad önerilmez. Dilin
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
			got, want = KeywordSpellingHint(ascii), messages.Text(messages.H1103, ascii, name)
		case k.IsBuiltin():
			got, want = BuiltinSpellingHint(ascii), NotInTRHint(ascii, name)
		default:
			continue
		}
		tried++
		if got != want {
			t.Errorf("%s (%s): ipucu %q, beklenen %q", k, ascii, got, want)
		}
		if s := NameSuggestion(ascii, nil); s != want {
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
	if h := BuiltinSpellingHint(codes.Name(codes.G100)); h != "" {
		t.Errorf("geçerli ad için ipucu olmamalı: %q", h)
	}
	got := UndefinedNameMessage("öğrenci_sayis", []string{"öğrenci_sayısı", "toplam"})
	if !strings.Contains(got, "'öğrenci_sayis'") || !strings.Contains(got, "'öğrenci_sayısı'") {
		t.Errorf("tanımsız ad iletisi yazılan adı ve en yakın adı söylemeli: %q", got)
	}
	if got := NameSuggestion("zzzzzz", []string{"toplam"}); got != "" {
		t.Errorf("benzer ad yokken öneri olmamalı: %q", got)
	}
}

// Kaldırılan ya da taşınan her genel ad hata yolunda sessiz kalmaz: kaldırılmışsa "kaldırıldı",
// üyeye taşınmışsa yeni yazımı söyler.
func TestDeadGlobalNamesAlwaysExplained(t *testing.T) {
	checked := 0
	for _, k := range codes.DeadCodes() {
		e, _ := codes.Active().Entry(k)
		if k.Namespace() != codes.NamespaceGlobal {
			continue
		}
		for _, name := range append([]string{e.Name}, e.Old...) {
			if _, live := codes.Find(codes.NamespaceGlobal, name); live {
				continue // yaşayan başka bir adla çakışan yazım (ör. yeniden kullanılan sözcük)
			}
			got := MovedGlobalNameError(name)
			if codes.Removed(k) {
				checked++
				if got != RemovedError(name) {
					t.Errorf("%s (%s): kaldırılma iletisi bekleniyordu, gelen %q", k, name, got)
				}
				continue
			}
			if to, _ := codes.MovedTo(k); to.Namespace() == codes.NamespaceMember {
				checked++
				if want := messages.Text(messages.H1102, name, codes.MemberSpelling(to)); got != want {
					t.Errorf("%s (%s): yeni yazım söylenmeli: %q, beklenen %q", k, name, got, want)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("hiç taşınmış ya da kaldırılmış genel ad denenmedi")
	}
}

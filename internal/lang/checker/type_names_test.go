package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (4D): ileti ve içindeki adlar tek dildedir. Denetleyici tipleri kanonik adla taşır;
// tip hatası ve tip açıklaması önerisi tip adlarını iletinin dilinde yazar (İngilizcede Int,
// Int or Float), kullanıcının sınıf adı olduğu gibi kalır.
func TestTypeNamesFollowTheMessageLanguage(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"değişken x: Tam = \"a\"\n", `type "Int" but a value of type "String"`},
		{"değişken x: Tam = 6 / 3\n", `type "Int" but a value of type "Int or Float"`},
		{"değişken x: Tam = 6 / 3\n", "gives Float if the quotient"},
		{"değişken x: ondalik = 1.5\n", "did you mean 'Float'?"},
		{"yapı Nokta {\n  x: Tam\n}\ndeğişken p: Nokta = 1\n", `declared with type "Nokta" but a value of type "Int"`},
	} {
		_, errs := checkErrs(t, c.source)
		var text []string
		for _, err := range errs {
			text = append(text, messages.ErrorIn(err, "en"))
		}
		if got := strings.Join(text, "\n"); !strings.Contains(got, c.want) || strings.Contains(got, "Tam") || strings.Contains(got, "Ondalık") {
			t.Errorf("%s: İngilizce iletide %q yok ya da Türkçe tip adı var:\n%s", c.source, c.want, got)
		}
	}
}

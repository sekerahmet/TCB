package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (11.8, 11.2; 13.11'in emsali): takma adlı aktarımın modül değerinin adına atama çalışma
// anının koduyla denetim hatasıdır (H2901); modül değeri köşeli ayraçla okunmaz ve atanmaz (H2904,
// ileti noktayla erişimi önerir). Değerin içi değişebilir; yerel adın gölgelediği takma ad, ad gibi
// yazılmamış indeks ve modül olmayan değer hatasızdır.
func TestModuleValueRules(t *testing.T) {
	head := "TR_import \"yardimci\" TR_as m\n"
	cases := []struct {
		source string
		code   messages.Code
		want   string
	}{
		{"m.PI = 7", messages.H2901, "'PI'"},
		{"m.sayac += 1", messages.H2901, "'sayac'"},
		{"TR_print(m[\"PI\"])", messages.H2904, "`m.PI`"},
		{"m[\"PI\"] = 9", messages.H2904, "`m.PI`"},
	}
	for _, tc := range cases {
		_, errs := checkErrs(t, head+tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], tc.code) {
			t.Errorf("tek %s beklenirdi, gelen %v\nkaynak:\n%s", tc.code, errs, tc.source)
			continue
		}
		if text := messages.ErrorIn(errs[0], "tr"); !strings.Contains(text, tc.want) {
			t.Errorf("ileti %s içermeli: %s", tc.want, text)
		}
	}
	for _, source := range []string{
		"m.yer.x = 1",
		"m.kayit.TR_append(1)",
		"TR_var k = \"PI\"\nTR_print(m[k])",
		"TR_print(m[\"a b\"])",
		"TR_func f() {\n    TR_var m = {\"PI\": 1}\n    m[\"PI\"] = 2\n}",
	} {
		noErrors(t, head+source)
	}
}

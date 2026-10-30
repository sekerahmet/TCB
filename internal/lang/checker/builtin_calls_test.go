package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (13.11): tipi bilinen alıcıda (Metin, Liste, Sözlük, Aralık) yerleşik metodun konumlu
// argüman sayısı tanım dosyasının imzasına uymazsa denetim hatasıdır; kod ve ileti çalışma
// anındakidir (H2011): ad ve kullanım kimlikle, görünüm adı ve <nil> olmadan.
func TestMemberArgCount(t *testing.T) {
	cases := []struct{ source, want string }{
		{"TR_var l = [3, 1, 2]\nTR_print(l.TR_sort(1))", "`.TR_sort()`"},
		{`TR_print("abc".TR_replace("a"))`, "`.TR_replace(eski, yeni)`"},
		{`TR_print({"a": 1}.TR_get())`, "`.TR_get(anahtar)`"},
		{"TR_print((0..3).TR_reverse(1))", "`.TR_reverse()`"},
		{"[1].TR_append()", "`.TR_append(eleman, ...)`"},
	}
	for _, tc := range cases {
		_, errs := checkErrs(t, tc.source)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H2011) {
			t.Errorf("tek H2011 beklenirdi, gelen %v\nkaynak:\n%s", errs, tc.source)
			continue
		}
		text := messages.ErrorIn(errs[0], "tr")
		if !strings.Contains(text, tc.want) || strings.Contains(text, "<nil>") || strings.Contains(text, "liste.") {
			t.Errorf("ileti %s kullanımını kimlikle vermeli: %s", tc.want, text)
		}
	}
	// .TR_sort()'a verilen anahtar fonksiyonu: ileti bugünkü yolu (demet sıralaması) da gösterir.
	_, errs := checkErrs(t, "TR_var l = [3, 1, 2]\nTR_print(l.TR_sort((x) => -x))")
	if len(errs) != 1 || !strings.Contains(messages.ErrorIn(errs[0], "en"), "].TR_sort().TR_map((p) => l[p.1])`") {
		t.Errorf("anahtarlı sıralamada demet yolu beklenirdi: %v", errs)
	}
	// Doğru sayı, tipi bilinmeyen alıcı (TR_JSON denetleyicide tanınmıyor), isteğe bağlı alıcı
	// ve henüz sayısı sınanmayan alıcı tipi (tensör) hatasızdır.
	for _, source := range []string{
		"TR_var l = [3, 1, 2]\nTR_print(l.TR_sort(), l.TR_join(), [\"a\"].TR_join(\",\"))",
		`TR_print(TR_json_parse("[1]").TR_sort(1))`,
		"TR_var l: TR_List[TR_Int]? = TR_none\nTR_print(l?.TR_sort(1))",
	} {
		noErrors(t, source)
	}
}

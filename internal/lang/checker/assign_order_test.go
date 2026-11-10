package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (5.3): çoklu atamada bir hedefin yeri (indeks, alanın sahibi) aynı atamada solunda atanan bir
// adı ya da yolu okuyorsa H1160; sağında atanan adı okumak, takas ve ilgisiz yollar serbesttir. İleti
// hedefi ve okunan adı söyler; değerler hedeflerle bire bir yazılmışsa ve ikinci yarının değerleri
// ilk yarıda atananı okumuyorsa ikiye ayrılmış yazımı önerir (önerinin sonucu: tests/deyimler_test.tr).
func TestMultiAssignPlaceReadsALeftTarget(t *testing.T) {
	head := "TR_struct K {\n    b: TR_Int\n    iç: TR_List[TR_Int]\n}\nTR_var l = [1, 2, 3]\nTR_var i = 0\nTR_var j = 1\nTR_var a = K(1, [0])\n"
	for _, tc := range []struct{ line, want string }{
		{"i, l[i] = 1, 5", "`l[i]` hedefinin yeri aynı atamada solunda atanan `i` adını ya da yolunu okuyor (5.3): yer atamadan önce, eski değerle hesaplanır (Python'da yeni değerle); atamayı ikiye ayırın: `i = 1; l[i] = 5`"},
		{"j, i, l[i + j] = 1, 2, 3", "atamayı ikiye ayırın: `j, i = 1, 2; l[i + j] = 3`"},
		{"a, a.b = K(2, [0]), 7", "`a.b` hedefinin yeri aynı atamada solunda atanan `a`"},
		{"a.iç, a.iç[0] = [4], 9", "`a.iç[0]` hedefinin yeri aynı atamada solunda atanan `a.iç`"},
		{"i, l[i] = l[0], i", "değerleri önce ayrı değişkenlere alıp atamayı ikiye ayırın"},
		{"i, l[i] = [1, 5]", "değerleri önce ayrı değişkenlere alıp atamayı ikiye ayırın"},
	} {
		errs := checkFileErrs(t, head+tc.line)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1160) {
			t.Errorf("%q: tek H1160 bekleniyordu, gelen %v", tc.line, errs)
			continue
		}
		if text := errs[0].Error(); !strings.Contains(text, tc.want) {
			t.Errorf("%q: ileti %q içermeli:\n%s", tc.line, tc.want, text)
		}
	}
	for _, line := range []string{
		"l[i], i = 9, 1",          // sağında atanan ad
		"l[0], l[2] = l[2], l[0]", // takas
		"a.b, a.iç = 3, [5]",      // ilgisiz yollar
		"i, j = j, i",
		"_, l[0] = 1, 2",
	} {
		if errs := checkFileErrs(t, head+line); len(errs) > 0 {
			t.Errorf("%q hatasız olmalı: %v", line, errs)
		}
	}
}

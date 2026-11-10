package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (13.1): TR_input giriş bitince TR_none verir; döngü koşulu TR_input çağrısını TR_none olmayan
// bir değerle '!=' ile karşılaştırıyorsa girdi bitince döngü durmaz, denetim hatasıdır (H1162). İleti
// girdi sonunu da bitiş sayan yazımı verir, `(çağrı ?? bitiş) != bitiş`, ve o yazım hatasızdır;
// girdi sonunda durduğunu interp.TestInputLoopSuggestionEnds koşarak sınar. TR_none'la karşılaştırma,
// '==' ve değişkene alınmış satır geçer.
func TestInputLoopNeverEnds(t *testing.T) {
	for _, tc := range []struct{ src, fix string }{
		{"TR_for TR_input() != \"\" {\n}", "(TR_input() ?? \"\") != \"\""},
		{"TR_for TR_input(\"> \") != \"son\" {\n}", "(TR_input(\"> \") ?? \"son\") != \"son\""},
		{"TR_for \"son\" != TR_input() {\n}", "(TR_input() ?? \"son\") != \"son\""},
	} {
		errs := checkFileErrs(t, tc.src)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1162) {
			t.Errorf("%q: tek H1162 bekleniyordu, gelen %v", tc.src, errs)
			continue
		}
		if !strings.Contains(errs[0].Error(), "`"+tc.fix+"`") {
			t.Errorf("%q: ileti %q yazımını vermeli:\n%s", tc.src, tc.fix, errs[0])
		}
		if errs := checkFileErrs(t, "TR_for "+tc.fix+" {\n}"); len(errs) > 0 {
			t.Errorf("önerilen yazım %q hatasız olmalı: %v", tc.fix, errs)
		}
	}
	for _, src := range []string{
		"TR_for TR_input() != TR_none {\n}",
		"TR_for TR_input() == \"e\" {\n}",
		"TR_var s = TR_input()\nTR_for s != TR_none {\n    s = TR_input()\n}",
	} {
		if errs := checkFileErrs(t, src); len(errs) > 0 {
			t.Errorf("%q hatasız olmalı: %v", src, errs)
		}
	}
}

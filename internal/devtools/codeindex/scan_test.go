package codeindex

import "testing"

// Kural: codeScanner yalnızca kod karakterlerini verir: çift tırnaklı metinler (kaçışlarıyla) ve
// açıksa `//` yorumları atlanır; çağıran i ile atlayabilir, end ile kesebilir.
// Neden: dört tarayıcı (blockEnd, valueEnd, paramPositions, defaultText) aynı durum makinesini
// ayrı ayrı yazıyordu; parantez sayımı ve konumlar çağıranda kaldı.
func TestCodeScannerSkipsStringsAndComments(t *testing.T) {
	collect := func(text string, comments bool) string {
		runes := []rune(text)
		sc := newCodeScanner(runes, 0, len(runes), comments)
		var out []rune
		for _, c, ok := sc.next(); ok; _, c, ok = sc.next() {
			out = append(out, c)
		}
		return string(out)
	}
	for _, c := range []struct {
		text     string
		comments bool
		want     string
	}{
		{`f(a, "x)y")`, true, "f(a, )"},
		{`"a\"(b" (`, true, " ("},
		{"a // (b", true, "a "},
		{"a // (b", false, "a // (b"},
		{`"açık`, true, ""},
		{`é(ş)`, true, "é(ş)"},
	} {
		if got := collect(c.text, c.comments); got != c.want {
			t.Errorf("tarama(%q, yorum=%v) = %q, beklenen %q", c.text, c.comments, got, c.want)
		}
	}
	runes := []rune("ab cd")
	sc := newCodeScanner(runes, 0, len(runes), true)
	sc.next()
	sc.i = 3 // 'ab ' atlanır
	if pos, c, _ := sc.next(); pos != 3 || c != 'c' {
		t.Errorf("atlamadan sonra konum %d %q", pos, c)
	}
	sc.end = 4
	sc.next()
	if _, _, ok := sc.next(); ok {
		t.Error("end kesince tarama bitmeli")
	}
}

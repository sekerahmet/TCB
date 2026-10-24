package codeindex

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// headerCounts metin çıktısının başlık satırlarındaki (öğe satırları dört boşlukla girintilidir) ilk sayılardır.
func headerCounts(text string) []string {
	count := regexp.MustCompile(`\((\d+)`)
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "    ") {
			continue
		}
		if m := count.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// Kural: metin çıktısının her başlığı listenin toplamını yazar, liste --sınır ile kesilse de (bağımsız denetim grup 6:
// `sınayanlar (12)` 42 testin ilk 12'siydi; etki ve yeniden adlandırma grupları da gösterilen satırı sayıyordu).
func TestTextHeadersCountTheWholeList(t *testing.T) {
	r := fixtureReader(t)
	cut := false
	for name, run := range map[string]func(limit int) (*Result, string){
		"kullananlar": func(n int) (*Result, string) {
			res := r.References("alan", QueryOptions{Limit: n})
			return res, Text(res)
		},
		"sınayanlar": func(n int) (*Result, string) {
			res := r.TestedBy("Kare", QueryOptions{Limit: n})
			return res, Text(res)
		},
		"etki": func(n int) (*Result, string) { res := r.Impact("geo", QueryOptions{Limit: n}); return res, Text(res) },
		"yeniden_adlandır": func(n int) (*Result, string) {
			res := r.RenameImpact("alan", "yüzey", RenameOptions{Root: fixtureDir, Limit: n})
			return res, RenameText(res)
		},
	} {
		_, all := run(0)
		res, one := run(1)
		cut = cut || res.Truncated
		if a, o := headerCounts(all), headerCounts(one); len(a) == 0 || !slices.Equal(a, o) {
			t.Errorf("%s: header counts %v without a limit, %v with --sınır 1:\n%s", name, a, o, one)
		}
	}
	if !cut {
		t.Error("no list was cut by --sınır 1; the fixture does not exercise the rule")
	}
}

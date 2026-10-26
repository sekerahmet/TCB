package codeindex

import (
	"regexp"
	"slices"
	"strconv"
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

// Kural: --sınır ile kesilen her grup, altında kaç satırın gizlendiğini yazar (eksikler listesi, ince ayar, DENETİM
// 2026-10-26: `tr dizin etki TR_type_GPT` "bunu aktaran modüller (13)" altında 12 satır verip susuyordu, kullananlar
// grubu sayısız "daha fazlası var" diyordu). Başlığın sayısından grubun gösterilen satırları çıkınca kalan, grubun notunda
// geçmelidir.
func TestEveryCutGroupSaysHowManyLinesAreHidden(t *testing.T) {
	r := fixtureReader(t)
	header := regexp.MustCompile(`^( *)\S.*\((\d+)`)
	cut := 0
	for name, text := range map[string]string{
		"kullananlar":      Text(r.References("alan", QueryOptions{Limit: 1})),
		"sınayanlar":       Text(r.TestedBy("Kare", QueryOptions{Limit: 1})),
		"etki":             Text(r.Impact("geo", QueryOptions{Limit: 1})),
		"yeniden_adlandır": RenameText(r.RenameImpact("alan", "yüzey", RenameOptions{Root: fixtureDir, Limit: 1})),
	} {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			m := header.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			want, _ := strconv.Atoi(m[2])
			shown, note := 0, ""
			for _, item := range lines[i+1:] {
				if len(item)-len(strings.TrimLeft(item, " ")) <= len(m[1]) {
					break
				}
				if strings.HasPrefix(strings.TrimSpace(item), "... ") {
					note = item
				} else {
					shown++
				}
			}
			if shown >= want {
				continue
			}
			cut++
			if !strings.Contains(note, strconv.Itoa(want-shown)) {
				t.Errorf("%s: %q shows %d of %d lines, its note %q does not say %d are hidden:\n%s", name, strings.TrimSpace(line), shown, want, note, want-shown, text)
			}
		}
	}
	if cut < 4 {
		t.Errorf("only %d groups were cut by --sınır 1; the fixture does not exercise the rule", cut)
	}
}

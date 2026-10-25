package main

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

// Bu dosya tests-for'un dosya ve belge cevabının grupları: dosyayı anan testler (cites), belgenin kurallarını koruyan
// testler ve boş cevapta neye bakıldığı.

// citingTests n'yi yorumunda ya da iletisinde anan (`cites`), başka bir grupta yazılmamış testlerdir.
func (ix *index) citingTests(n *Node, listed []*Node) []*Edge {
	shown := map[string]bool{}
	for _, t := range listed {
		if t != nil {
			shown[t.ID] = true
		}
	}
	var out []*Edge
	for _, e := range ix.in[n.ID] {
		if e.Kind == edgeCites && !shown[e.From] {
			out = append(out, e)
		}
	}
	return out
}

// docRules bir belgenin (başlıkları altındaki) kurallarıdır, koruyan testleri olanlar; belge değilse boş. README'nin
// "Denetim:" maddesi kuralı teste bağlar (guarded_by); tests-for <README> o testleri verir: kuralın metni ile testi birlikte
// değişir. Kural belgesi daha önce "no tests found" diyordu (TCB 2026-10-25, 2. madde: internal/README.md).
func (ix *index) docRules(n *Node) []*Node {
	if n.Kind != kindDoc {
		return nil
	}
	var out []*Node
	seen := map[string]bool{n.ID: true}
	queue := []string{n.ID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range ix.out[cur] {
			c := ix.byID[e.To]
			if e.Kind != edgeContains || c == nil || seen[c.ID] || c.File != n.File {
				continue
			}
			seen[c.ID] = true
			queue = append(queue, c.ID)
			if c.Kind == kindRule && slices.ContainsFunc(ix.out[c.ID], func(g *Edge) bool { return g.Kind == edgeGuardedBy }) {
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// printDocRules belgenin kurallarını ve her birini koruyan testleri yazar.
func (ix *index) printDocRules(w io.Writer, rules []*Node, limit int) {
	if len(rules) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests that guard the rules it states (%d rules; a rule's text and its test change together):\n", len(rules))
	forEachLimited(w, rules, limit, "    ", moreHint, func(r *Node) string {
		var names []string
		for _, g := range ix.out[r.ID] {
			if t := ix.byID[g.To]; g.Kind == edgeGuardedBy && t != nil {
				names = append(names, t.Name)
			}
		}
		sort.Strings(names)
		return fmt.Sprintf("%s  <- %s", describe(r), strings.Join(names, ", "))
	})
}

// printCites anan testleri okuyanlardan ayrı yazar: test dosyayı okumaz, kuralını ya da adını anar.
func (ix *index) printCites(w io.Writer, cites []*Edge, limit int) {
	if len(cites) == 0 {
		return
	}
	sort.SliceStable(cites, func(i, j int) bool { return ix.byID[cites[i].From].Name < ix.byID[cites[j].From].Name })
	fmt.Fprintf(w, "  <- tests that cite it (%d; they name this path in a comment or a message and do not read it: the rule it states is checked there):\n", len(cites))
	forEachLimited(w, cites, limit, "    ", moreHint, func(e *Edge) string { return ix.edgeLine(e, e.From, true) })
}

// searchedFor "no tests found"un ardına neye bakıldığını yazar: dosya ya da belgede bu, aranıp bulunmadı demektir
// (izlenmeyen bir dosya grafikte yoktur, resolve onu ayrıca söyler; TCB 2026-10-25, 2. madde).
func searchedFor(n *Node) string {
	if n.Kind != kindFile && n.Kind != kindDoc {
		return ""
	}
	return " (searched: path constants, globs and go:embed in Go code, and the comments and messages of Go tests)"
}

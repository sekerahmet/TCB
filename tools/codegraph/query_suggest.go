package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// suggestion yakın ad önerisidir; score küçüldükçe iyi: 0 içerme, sonrası düzenleme uzaklığı.
type suggestion struct {
	node  *Node
	score int
}

// maxEdit sorgu uzunluğuna göre izin verilen en büyük düzenleme uzaklığıdır (1..4).
func maxEdit(q string) int {
	return min(4, 1+len([]rune(q))/4)
}

// suggest bulunamayan bir ada yakın düğümleri verir: ad (ya da son parçası, takma ad) sorguyu
// içeriyorsa ya da sorguya Levenshtein uzaklığı küçükse; kinds doluysa yalnızca o türler.
func (ix *index) suggest(q string, kinds []string) []*Node {
	lq := codeindex.Fold(q)
	maxD := maxEdit(q)
	var found []suggestion
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		if len(kinds) > 0 && !contains(kinds, n.Kind) {
			continue
		}
		if s := nameScore(n, lq, maxD); s >= 0 {
			found = append(found, suggestion{n, s})
		} else if movedMember(n, lq) {
			found = append(found, suggestion{n, 1})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.score != b.score {
			return a.score < b.score
		}
		if len(a.node.Name) != len(b.node.Name) {
			return len(a.node.Name) < len(b.node.Name)
		}
		return a.node.ID < b.node.ID
	})
	out := make([]*Node, len(found))
	for i, f := range found {
		out[i] = f.node
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// nameScore düğümün adlarından sorguya en yakınının puanıdır; uygun değilse -1.
func nameScore(n *Node, lq string, maxD int) int {
	cands := append([]string{n.Name}, n.Aliases...)
	if i := strings.LastIndexAny(n.Name, "./"); i >= 0 {
		cands = append(cands, n.Name[i+1:])
	}
	best := -1
	for _, c := range cands {
		lc := codeindex.Fold(c)
		if strings.Contains(lc, lq) {
			return 0
		}
		if d := turkish.Distance(lq, lc, maxD); d <= maxD && (best < 0 || d < best) {
			best = d
		}
	}
	return best
}

// movedMember lq bir üyenin katlanmış tam adıysa (`interp.vmshared.gilsince`) n aynı paketteki aynı kısa adlı üye mi:
// alan ya da metot başka bir tipe taşınınca eski tam ad yeniye düzenleme uzaklığıyla yakın düşmez (gaps, "Taşınan
// alanın eski tam adı öneri vermiyor": `fields interp.vmShared.gilSince` düz "not found" veriyordu).
func movedMember(n *Node, lq string) bool {
	pkg, _, ok := strings.Cut(lq, ".")
	tail := lq[strings.LastIndex(lq, ".")+1:]
	if !ok || strings.Count(lq, ".") < 2 || strings.Count(n.Name, ".") < 2 {
		return false
	}
	ln := codeindex.Fold(n.Name)
	return strings.HasPrefix(ln, pkg+".") && strings.HasSuffix(ln, "."+tail)
}

// printSuggestions "did you mean" listesini yazar (varsa): en yakın altısı, kalanı öbür çıktılar gibi "... N more"
// ile (--all hepsi; bağımsız denetim O3: liste sessizce kesiliyordu).
func (ix *index) printSuggestions(w io.Writer, q string, kinds []string, all bool) {
	sug := ix.suggest(q, kinds)
	if len(sug) == 0 {
		return
	}
	limit := 6
	if all {
		limit = 0
	}
	fmt.Fprintln(w, "did you mean:")
	forEachLimited(w, sug, limit, "  ", moreHint, describe)
}

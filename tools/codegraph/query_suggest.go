package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
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
func (ix *index) suggest(q string, limit int, kinds []string) []*Node {
	lq := strings.ToLower(q)
	maxD := maxEdit(q)
	var found []suggestion
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		if len(kinds) > 0 && !contains(kinds, n.Kind) {
			continue
		}
		if s := nameScore(n, lq, maxD); s >= 0 {
			found = append(found, suggestion{n, s})
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
	if len(found) > limit {
		found = found[:limit]
	}
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
		lc := strings.ToLower(c)
		if strings.Contains(lc, lq) {
			return 0
		}
		if d := editDistance(lq, lc, maxD); d <= maxD && (best < 0 || d < best) {
			best = d
		}
	}
	return best
}

// editDistance iki metnin Levenshtein uzaklığıdır; bound'u aşacağı belliyse bound+1 verir.
func editDistance(a, b string, bound int) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > bound || -d > bound {
		return bound + 1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			rowMin = min(rowMin, cur[j])
		}
		if rowMin > bound {
			return bound + 1
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// printSuggestions "did you mean" listesini yazar (varsa).
func (ix *index) printSuggestions(w io.Writer, q string, kinds []string) {
	sug := ix.suggest(q, 6, kinds)
	if len(sug) == 0 {
		return
	}
	fmt.Fprintln(w, "did you mean:")
	for _, n := range sug {
		fmt.Fprintf(w, "  %s\n", describe(n))
	}
}

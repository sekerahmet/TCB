package main

import (
	"fmt"
	"io"
)

// step yolun bir adımıdır: from'dan to'ya e kenarı; forward=false ise kenar ters yönde gidilmiştir.
type step struct {
	e       *Edge
	to      string
	forward bool
}

// pathMode yol aramasının nasıl yapılacağıdır.
type pathMode int

const (
	pathDirected    pathMode = iota // yalnızca kenar yönünde (contains hariç): "A, B'ye ulaşır mı"
	pathUndirected                  // yön yok sayılır, ama tür/paket/dosya düğümlerinden geçmez
	pathThroughHubs                 // yön yok sayılır, çok bağlı düğümlerden de geçer (işaretlenir)
)

// isHub çok bağlı bir düğüm mü: tür, paket, dosya ve belge. Yönsüz bir yol bunlardan geçince her şeyi her şeye bağlar
// ("Attention -uses-> Tensor <-uses- Tensor.Softmax": iki işlev aynı türü kullanıyor diye çağrı yolu yoktur).
func isHub(n *Node) bool {
	switch n.Kind {
	case kindType, kindPackage, kindFile, kindDoc:
		return true
	}
	return false
}

// shortestPath from'dan to'ya en kısa yolu bulur. Varsayılan yalnızca yönlü yol arar ve yoksa söyler; yönsüz arama ve
// çok bağlı düğümlerden geçen arama açık seçenektir (mode).
func (ix *index) shortestPath(from, to string, mode pathMode) (steps []step, ok bool) {
	switch mode {
	case pathDirected:
		return ix.bfs(from, to, true, false, false)
	case pathUndirected:
		return ix.bfs(from, to, false, false, false)
	}
	return ix.bfs(from, to, false, true, true)
}

// bfs yön ve hub kurallarıyla en kısa yolu arar; ara düğüm hub ise (hubs=false iken) geçilmez, uçlar serbesttir.
func (ix *index) bfs(from, to string, directed, withContains, hubs bool) ([]step, bool) {
	prev := map[string]step{}
	prevFrom := map[string]string{}
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == to {
			break
		}
		var next []step
		for _, e := range ix.out[cur] {
			next = append(next, step{e, e.To, true})
		}
		if !directed {
			for _, e := range ix.in[cur] {
				next = append(next, step{e, e.From, false})
			}
		}
		for _, s := range next {
			if seen[s.to] || (!withContains && s.e.Kind == edgeContains) || (!hubs && s.to != to && isHub(ix.byID[s.to])) {
				continue
			}
			seen[s.to] = true
			prev[s.to], prevFrom[s.to] = s, cur
			queue = append(queue, s.to)
		}
	}
	if !seen[to] {
		return nil, false
	}
	var rev []step
	for at := to; at != from; at = prevFrom[at] {
		rev = append(rev, prev[at])
	}
	steps := make([]step, len(rev))
	for i, s := range rev {
		steps[len(rev)-1-i] = s
	}
	return steps, true
}

func (ix *index) printPath(w io.Writer, a, b *Node, mode pathMode) bool {
	if a.ID == b.ID {
		fmt.Fprintln(w, describe(a))
		fmt.Fprintln(w, "  (same node)")
		return true
	}
	steps, ok := ix.shortestPath(a.ID, b.ID, mode)
	if !ok {
		return false
	}
	label := map[pathMode]string{pathDirected: "directed", pathUndirected: "undirected, not through types, packages or files",
		pathThroughHubs: "undirected, may pass through types, packages and files"}[mode]
	fmt.Fprintf(w, "path (%s, %d steps):\n", label, len(steps))
	fmt.Fprintf(w, "  %s\n", describe(a))
	for _, s := range steps {
		arrow := fmt.Sprintf("-%s->", s.e.Kind)
		if !s.forward {
			arrow = fmt.Sprintf("<-%s-", s.e.Kind)
		}
		tag := ""
		if s.e.Confidence == inferred {
			tag = "  [INFERRED]"
		}
		if n := ix.byID[s.to]; isHub(n) && n.ID != b.ID {
			tag += "  [hub: connects everything that uses it]"
		}
		fmt.Fprintf(w, "  %s %s%s\n", arrow, describe(ix.byID[s.to]), tag)
	}
	return true
}

// noPathMessage yol bulunamayınca söylenen cümledir; yönlü aramada ne yapılabileceğini de söyler.
func noPathMessage(a, b *Node, mode pathMode) string {
	switch mode {
	case pathDirected:
		return fmt.Sprintf("no directed path from %s to %s (edges are followed in their direction: a call path exists only if %s reaches %s through calls, uses and dispatches).\n"+
			"  --undirected looks for any connection ignoring direction, never through a type, package or file; --through-hubs allows those too.", a.Name, b.Name, a.Name, b.Name)
	case pathUndirected:
		return fmt.Sprintf("no connection between %s and %s that avoids types, packages and files (--through-hubs allows them, marked [hub])", a.Name, b.Name)
	}
	return fmt.Sprintf("no path between %s and %s", a.Name, b.Name)
}

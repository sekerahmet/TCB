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

// shortestPath from'dan to'ya en kısa yolu bulur. Üç deneme sırayla: yönlü (contains hariç),
// yönsüz (contains hariç), yönsüz (hepsi). Hangisinin tuttuğu mode'da döner.
func (ix *index) shortestPath(from, to string) (steps []step, mode string, ok bool) {
	for _, try := range []struct {
		mode               string
		directed, contains bool
	}{{"directed", true, false}, {"undirected", false, false}, {"undirected, through containment", false, true}} {
		if s, found := ix.bfs(from, to, try.directed, try.contains); found {
			return s, try.mode, true
		}
	}
	return nil, "", false
}

func (ix *index) bfs(from, to string, directed, withContains bool) ([]step, bool) {
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
			if seen[s.to] || (!withContains && s.e.Kind == edgeContains) {
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

func (ix *index) printPath(w io.Writer, a, b *Node) bool {
	if a.ID == b.ID {
		fmt.Fprintln(w, describe(a))
		fmt.Fprintln(w, "  (same node)")
		return true
	}
	steps, mode, ok := ix.shortestPath(a.ID, b.ID)
	if !ok {
		return false
	}
	fmt.Fprintf(w, "path (%s, %d steps):\n", mode, len(steps))
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
		fmt.Fprintf(w, "  %s %s%s\n", arrow, describe(ix.byID[s.to]), tag)
	}
	return true
}

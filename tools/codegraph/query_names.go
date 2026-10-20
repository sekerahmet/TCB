package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// printNameChain ad kodu düğümü için zinciri yazar: TR/İngilizce adlar → Go sabiti → kodun
// kullanıldığı yerler (kayıt tabloları, işlevler) ve oradaki satırda anılan uygulama işlevleri →
// belgedeki başlık. "query zaman_ms" bu zinciri verir.
func (ix *index) printNameChain(w io.Writer, n *Node, limit int) {
	if len(n.Aliases) > 0 {
		fmt.Fprintf(w, "  names: %s\n", strings.Join(n.Aliases, " · "))
	}
	for _, e := range ix.in[n.ID] {
		if e.Kind == edgeDenotes {
			fmt.Fprintf(w, "  Go constant: %s\n", describe(ix.byID[e.From]))
		}
	}
	var lines []string
	for _, e := range ix.in[n.ID] {
		if e.Kind != edgeUsesName {
			continue
		}
		owner := ix.byID[e.From]
		if inTestFile(owner) || strings.HasSuffix(e.File, "_test.go") {
			continue
		}
		line := fmt.Sprintf("%s at %s", owner.Name, where(e.File, e.Line))
		if impl := ix.implementationsAt(owner, e); len(impl) > 0 {
			line += " -> " + strings.Join(impl, ", ")
		}
		lines = append(lines, line)
	}
	sort.Strings(lines)
	if len(lines) > 0 {
		fmt.Fprintf(w, "  registered or used in Go (%d; implementation named on the same line):\n", len(lines))
		printLimited(w, lines, limit, "    ")
	}
	for i := range ix.g.Nodes {
		h := &ix.g.Nodes[i]
		if h.Kind == kindHeading && (h.Name == n.Name || strings.HasPrefix(h.Name, n.Name+" ")) {
			fmt.Fprintf(w, "  documented: %s\n", describe(h))
		}
	}
}

// implementationsAt kodun kullanıldığı satırda aynı sahibin başvurduğu işlevleri verir (kayıt
// tablosunda {codes.G502, timeMsBuiltin} satırının timeMsBuiltin'i).
func (ix *index) implementationsAt(owner *Node, use *Edge) []string {
	var out []string
	for _, e := range ix.out[owner.ID] {
		if e.Kind == edgeCalls && e.File == use.File && e.Line == use.Line {
			if t := ix.byID[e.To]; isFuncNode(t) {
				out = append(out, fmt.Sprintf("%s [%s]", t.Name, where(t.File, t.Line)))
			}
		}
	}
	sort.Strings(out)
	return out
}

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
		if e.Kind != edgeUsesName && e.Kind != edgeUsesEnv {
			continue
		}
		owner := ix.byID[e.From]
		// Yalnızca Go kodu: .tr dosyalarının ve iletilerin kullanımı uses_name ile gelir ama kayıt yeri değildir; yüzlerce
		// TR kullanıcısı arasında yerleşiğin Go'daki kaydı (TR_fn_print → interp.printBuiltin) kayboluyordu.
		if inTestFile(owner) || !strings.HasSuffix(e.File, ".go") || strings.HasSuffix(e.File, "_test.go") {
			continue
		}
		line := fmt.Sprintf("%s at %s", owner.Name, where(e.File, e.Line))
		if impl := ix.implementationsAt(owner, e); len(impl) > 0 {
			line += " -> " + strings.Join(impl, ", ")
		}
		lines = append(lines, line)
	}
	sort.SliceStable(lines, func(i, j int) bool { // uygulamayı adlandıran kayıt satırları önce
		ai, bj := strings.Contains(lines[i], " -> "), strings.Contains(lines[j], " -> ")
		if ai != bj {
			return ai
		}
		return lines[i] < lines[j]
	})
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
// tablosunda {codes.FnTimeMs, timeMsBuiltin} satırının timeMsBuiltin'i). Satırda ad koduyla anahtarlanmış çiftler varsa
// (`codes.CmdDebug: debugCommand, codes.CmdEncode: encodeCommand`) uygulama sorulan kodun çiftinin değeridir (Edge.Via).
func (ix *index) implementationsAt(owner *Node, use *Edge) []string {
	var all, keyed []string
	for _, e := range ix.out[owner.ID] {
		if e.Kind == edgeCalls && e.File == use.File && e.Line == use.Line {
			if t := ix.byID[e.To]; isFuncNode(t) {
				name := fmt.Sprintf("%s [%s]", t.Name, where(t.File, t.Line))
				all = append(all, name)
				if e.Via == use.To {
					keyed = append(keyed, name)
				}
			}
		}
	}
	if len(keyed) > 0 {
		all = keyed
	}
	sort.Strings(all)
	return all
}

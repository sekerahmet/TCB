package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// index okunmuş grafiğin arama tablolarıdır.
type index struct {
	g    *Graph
	byID map[string]*Node
	out  map[string][]*Edge
	in   map[string][]*Edge
}

func newIndex(g *Graph) *index {
	ix := &index{g: g, byID: map[string]*Node{}, out: map[string][]*Edge{}, in: map[string][]*Edge{}}
	for i := range g.Nodes {
		ix.byID[g.Nodes[i].ID] = &g.Nodes[i]
	}
	for i := range g.Edges {
		e := &g.Edges[i]
		ix.out[e.From] = append(ix.out[e.From], e)
		ix.in[e.To] = append(ix.in[e.To], e)
	}
	return ix
}

// minTextQuery metin aramasının en kısa sorgusudur (kısa parçalar her şeye uyar).
const minTextQuery = 3

// lookup bir sorgu metnini düğümlere çözer (kinds doluysa yalnızca o türlerde arar); ilk eşleşme veren basamaktaki bütün düğümleri döner
// (birden çoksa belirsizdir): kimlik, kimliğin "tür:" önekleri atılmış hali, tam ad, sınırda
// biten ad ("New" → "messages.New", yalnızca '.' ve '/' sınırında), büyük-küçük harfsiz tam ad; hiçbiri tutmazsa metin aramaları: adın başı, adın alt dizesi, açıklama
// metninin alt dizesi (en az minTextQuery karakter).
func (ix *index) lookup(q string, kinds ...string) []*Node {
	if n := ix.byID[q]; n != nil && (len(kinds) == 0 || contains(kinds, n.Kind)) {
		return []*Node{n}
	}
	tiers := []func(n *Node) bool{
		// Takma ad: ad kodunun dildeki adı (zaman_ms, print) ya da bir .tr dosyasının modül adı
		// (katman, tr/katman); düğümü, aynı adlı belge başlığından önce gelir.
		func(n *Node) bool { return contains(n.Aliases, q) },
		func(n *Node) bool {
			return idTailEquals(n.ID, q) || idTailEquals(strings.ReplaceAll(n.ID, ix.g.Module+"/", ""), q)
		},
		func(n *Node) bool { return n.Name == q },
		func(n *Node) bool { return strings.HasSuffix(n.Name, "."+q) || strings.HasSuffix(n.Name, "/"+q) },
		func(n *Node) bool { return strings.HasSuffix(n.ID, "/"+q) }, // yolla nitelenmiş: internal/lang/messages.New
		func(n *Node) bool { return strings.EqualFold(n.Name, q) },
	}
	// Metin aramaları: ad ya da (ileti, kural, başlık) açıklama metninin başı/alt dizesi.
	if len([]rune(q)) >= minTextQuery {
		lq := strings.ToLower(q)
		tiers = append(tiers,
			func(n *Node) bool { return strings.HasPrefix(strings.ToLower(n.Name), lq) },
			func(n *Node) bool { return strings.Contains(strings.ToLower(n.Name), lq) },
			func(n *Node) bool { return strings.Contains(strings.ToLower(n.Detail), lq) },
		)
	}
	for _, match := range tiers {
		var found []*Node
		for i := range ix.g.Nodes {
			if n := &ix.g.Nodes[i]; (len(kinds) == 0 || contains(kinds, n.Kind)) && match(n) {
				found = append(found, n)
			}
		}
		if len(found) > 0 {
			return found
		}
	}
	return nil
}

// idTailEquals kimliğin ':' sonrası kuyruklarından biri q mu (doc:docs/a.md#b için docs/a.md#b).
func idTailEquals(id, q string) bool {
	for i := 0; i < len(id); i++ {
		if id[i] == ':' && id[i+1:] == q {
			return true
		}
	}
	return false
}

func where(file string, line int) string {
	switch {
	case file == "":
		return ""
	case line > 0:
		return fmt.Sprintf("%s:%d", file, line)
	}
	return file
}

// describe düğümü tek satırda yazar: ad [tür] dosya:satır.
func describe(n *Node) string {
	s := fmt.Sprintf("%s [%s]", n.Name, n.Kind)
	if w := where(n.File, n.Line); w != "" {
		s += " " + w
	}
	return s
}

// printCandidates belirsiz bir adın adaylarını yazar.
func (ix *index) printCandidates(w io.Writer, q string, cands []*Node, limit int) {
	fmt.Fprintf(w, "%q is ambiguous: %d candidates (repeat with an id or a longer name)\n", q, len(cands))
	sorted := append([]*Node(nil), cands...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for i, n := range sorted {
		if limit > 0 && i >= limit {
			fmt.Fprintf(w, "  ... %d more\n", len(sorted)-limit)
			break
		}
		fmt.Fprintf(w, "  %s  (id %s)\n", describe(n), n.ID)
	}
}

// edgeLine bir kenarın karşı ucunu yazar; INFERRED etiketi yalnızca çıkarımda görünür.
func (ix *index) edgeLine(e *Edge, other string, withSite bool) string {
	n := ix.byID[other]
	s := describe(n)
	if withSite {
		if w := where(e.File, e.Line); w != "" {
			s = fmt.Sprintf("%s [%s] at %s", n.Name, n.Kind, w)
		}
	}
	if e.Count > 1 {
		s += fmt.Sprintf(" x%d", e.Count)
	}
	if e.Confidence == inferred {
		s += "  [INFERRED]"
	}
	return s
}

// printGroups kenarları türe göre gruplayıp yazar (EXTRACTED önce, sonra ada göre).
func (ix *index) printGroups(w io.Writer, heading string, edges []*Edge, otherOf func(*Edge) string, limit int, site bool) {
	byKind := map[string][]*Edge{}
	var kinds []string
	for _, e := range edges {
		if _, ok := byKind[e.Kind]; !ok {
			kinds = append(kinds, e.Kind)
		}
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		list := byKind[k]
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a.Confidence != b.Confidence {
				return a.Confidence == extracted
			}
			na, nb := ix.byID[otherOf(a)].Name, ix.byID[otherOf(b)].Name
			if na != nb {
				return na < nb
			}
			return a.Line < b.Line
		})
		fmt.Fprintf(w, "%s %s (%d):\n", heading, k, len(list))
		for i, e := range list {
			if limit > 0 && i >= limit {
				fmt.Fprintf(w, "    ... %d more (--all lists them)\n", len(list)-limit)
				break
			}
			fmt.Fprintf(w, "    %s\n", ix.edgeLine(e, otherOf(e), site))
		}
	}
}

func fromOf(e *Edge) string { return e.From }
func toOf(e *Edge) string   { return e.To }

// printQuery düğümü ve komşularını yazar: önce çıkanlar (bu düğüm → öteki), sonra gelenler.
func (ix *index) printQuery(w io.Writer, n *Node, limit int) {
	fmt.Fprintln(w, describe(n))
	fmt.Fprintf(w, "  id: %s\n", n.ID)
	if n.Detail != "" {
		fmt.Fprintf(w, "  %s\n", n.Detail)
	}
	if len(n.Params) > 0 {
		fmt.Fprintf(w, "  params: %s\n", strings.Join(n.Params, ", "))
	}
	ix.printGroups(w, "  ->", ix.out[n.ID], toOf, limit, false)
	ix.printGroups(w, "  <-", ix.in[n.ID], fromOf, limit, true)
	if n.Kind == kindName {
		ix.printNameChain(w, n, limit)
	}
}

// printWhoUses bir düğümü kullananları yazar: gelen kenarlar (contains hariç), kullanım yeriyle.
func (ix *index) printWhoUses(w io.Writer, n *Node, limit int) {
	uses := ix.whoUsesHeader(w, n)
	if len(uses) == 0 {
		fmt.Fprintln(w, "  no uses")
		return
	}
	ix.printGroups(w, "  <-", uses, fromOf, limit, true)
}

// whoUsesHeader düğümü ve (kod ise) Go sabitini yazar, kullanım kenarlarını verir.
func (ix *index) whoUsesHeader(w io.Writer, n *Node) []*Edge {
	fmt.Fprintln(w, describe(n))
	if n.Detail != "" {
		fmt.Fprintf(w, "  %s\n", n.Detail)
	}
	var uses []*Edge
	for _, e := range ix.in[n.ID] {
		switch e.Kind {
		case edgeContains:
		case edgeDenotes: // kodun Go sabiti: kullanım değil, ama "kim kullanıyor"un sonraki sorusu
			fmt.Fprintf(w, "  Go constant: %s (who-uses it for its users)\n", describe(ix.byID[e.From]))
		default:
			uses = append(uses, e)
		}
	}
	return uses
}

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
	note func(*Node) string // verilirse edgeLine her düğümün altına bir not satırı ekler (test_notes.go)
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
			return ix.withParams(q, found, kinds)
		}
	}
	return nil
}

// withParams çıplak ad sorgusuna aynı adı taşıyan TR parametrelerini ekler (kinds boşsa ya da tr_param içeriyorsa):
// aynı adın farklı anlamlarda kullanıldığı yerler birlikte görünür. Sorgu ad kodu ya da işlev adı olsa da
// eklenir; parametre yoksa found olduğu gibi döner.
func (ix *index) withParams(q string, found []*Node, kinds []string) []*Node {
	if strings.ContainsAny(q, "./:") || len(kinds) > 0 && !contains(kinds, kindTRParam) {
		return found
	}
	for i := range ix.g.Nodes {
		if n := &ix.g.Nodes[i]; n.Kind == kindTRParam && strings.HasSuffix(n.Name, "."+q) && !containsNode(found, n) {
			found = append(found, n)
		}
	}
	return found
}

func containsNode(list []*Node, n *Node) bool {
	for _, m := range list {
		if m == n {
			return true
		}
	}
	return false
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
	forEachLimited(w, sorted, limit, "  ", "", func(n *Node) string { return fmt.Sprintf("%s  (id %s)", describe(n), n.ID) })
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
	if ix.note != nil {
		if t := ix.note(n); t != "" {
			s += "\n      " + t
		}
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
		forEachLimited(w, list, limit, "    ", moreHint, func(e *Edge) string { return ix.edgeLine(e, otherOf(e), site) })
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
	if n.Doc != "" {
		fmt.Fprintf(w, "  doc: %s\n", n.Doc)
	}
	if len(n.Params) > 0 {
		fmt.Fprintf(w, "  params: %s\n", strings.Join(n.Params, ", "))
	}
	out := ix.printFields(w, n)
	ix.printGroups(w, "  ->", out, toOf, limit, false)
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

// printFields bir türün alanlarını (type → field `contains`) metotlardan ayrı ve sınırsız yazar: alanlar azdır ve yapının
// biçimi bilinecek ilk şeydir; 34 metodun altında kalan iki alan ("compiler.source") yanlış bir tasarım sonucuna götürmüştü.
// Alan olmayan çıkan kenarları döner (metotlar vb., sınırlı gruplara gider).
func (ix *index) printFields(w io.Writer, n *Node) []*Edge {
	var fields []*Node
	var rest []*Edge
	for _, e := range ix.out[n.ID] {
		if t := ix.byID[e.To]; e.Kind == edgeContains && t != nil && t.Kind == kindField {
			fields = append(fields, t)
		} else {
			rest = append(rest, e)
		}
	}
	if len(fields) == 0 {
		return rest
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Line < fields[j].Line })
	fmt.Fprintf(w, "  -> fields (%d, always shown):\n", len(fields))
	for _, f := range fields {
		line := fmt.Sprintf("    %s", f.Name)
		if f.Detail != "" {
			line += "  " + f.Detail
		}
		if f.Doc != "" {
			line += "  // " + truncate(f.Doc, 100)
		}
		fmt.Fprintln(w, line)
	}
	return rest
}

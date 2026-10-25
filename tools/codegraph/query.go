package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// index okunmuş grafiğin arama tablolarıdır.
type index struct {
	root       string  // depo kökü (sorgu anında dosya okuyan yardımcılar için; boşsa okunmaz)
	fnMessages []*Node // işlev adı taşıyan iletiler (functionMessages; tembel)
	g          *Graph
	byID       map[string]*Node
	out        map[string][]*Edge
	in         map[string][]*Edge
	note       func(*Node) string // verilirse edgeLine her düğümün altına bir not satırı ekler (test_notes.go)
	trNames    []trName           // çıplak adla çözülen TR düğümleri (trNameTable; tembel)
	folded     []foldedNode       // düğümlerin katlanmış adları (nodeFolds; tembel)
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

// lookup bir sorgu metnini düğümlere çözer (kinds doluysa yalnızca o türlerde arar); ilk eşleşme veren basamaktaki bütün
// düğümleri döner (birden çoksa belirsizdir). Çıplak ad (`.`, `/`, `:` içermeyen) TR adlarında (modül, TR bildirimi,
// test, ad kodu) tr dizin'in kuralıyla çözülür (trByName); Go ve belge düğümlerinde basamaklar: kimlik, kimliğin "tür:"
// önekleri atılmış hali, takma ad, tam ad, sınırda biten ad ("New" → "messages.New", yalnızca '.' ve '/' sınırında),
// büyük-küçük harfsiz tam ad ve ad sonu; hiçbiri tutmazsa metin aramaları: adın başı, adın alt dizesi, açıklama metninin
// alt dizesi (en az minTextQuery karakter).
func (ix *index) lookup(q string, kinds ...string) []*Node {
	if n := ix.byID[q]; n != nil && (len(kinds) == 0 || contains(kinds, n.Kind)) {
		return []*Node{n}
	}
	bare := !strings.ContainsAny(q, "./:")
	var tr []*Node
	trFolded := false // TR adayları büyük harfli sorgunun harfsiz geçişinden (Go'nun harfsiz basamağıyla aynı sırada)
	if bare {
		tr, trFolded = ix.trByName(q, kinds)
		if !trFolded && slices.ContainsFunc(tr, func(n *Node) bool { return !strings.HasPrefix(n.ID, "tr:") || n.Kind == kindFile }) {
			return tr // modül ya da ad kodu: takma ad basamağıdır, Go adlarından önce gelir
		}
	}
	foldTiers := 5   // basamak listesinde harfsiz basamakların başladığı yer
	var qf [2]string // sorgunun iki katlaması (folds sırasıyla)
	for k, f := range folds {
		qf[k] = f(q)
	}
	nf := ix.nodeFolds()
	anyFold := func(match func(k int) bool) bool { return match(0) || match(1) }
	var cur *foldedNode // basamak döngüsünün o anki düğümünün katlanmışları
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
		// Büyük-küçük harf yok sayılarak tam ad, sonra ad sonu (foldEqual: Türkçe ya da Go kuralı).
		func(n *Node) bool { return anyFold(func(k int) bool { return cur.name[k] == qf[k] }) },
		func(n *Node) bool {
			return anyFold(func(k int) bool {
				return strings.HasSuffix(cur.name[k], "."+qf[k]) || strings.HasSuffix(cur.name[k], "/"+qf[k])
			})
		},
	}
	nameTiers := len(tiers)
	// Metin aramaları: ad ya da (ileti, kural, başlık) açıklama metninin başı/alt dizesi.
	if len([]rune(q)) >= minTextQuery {
		tiers = append(tiers,
			// Önce sözcüğün kendisi (harf katlamasıyla, sözcük sınırında): "ölü" sorusu "bölüm"ü değil, "ölü kod"u bulur.
			func(n *Node) bool { return anyFold(func(k int) bool { return hasWordFolded(cur.name[k], qf[k]) }) },
			func(n *Node) bool {
				return anyFold(func(k int) bool { return hasWordFolded(cur.detail[k], qf[k]) || hasWordFolded(cur.doc[k], qf[k]) })
			},
			func(n *Node) bool { return anyFold(func(k int) bool { return strings.HasPrefix(cur.name[k], qf[k]) }) },
			func(n *Node) bool { return anyFold(func(k int) bool { return strings.Contains(cur.name[k], qf[k]) }) },
			func(n *Node) bool { return anyFold(func(k int) bool { return strings.Contains(cur.detail[k], qf[k]) }) },
		)
	}
	for ti, match := range tiers {
		if len(tr) > 0 && ti >= nameTiers {
			break // TR adayı varken metin aramasına inilmez
		}
		var found []*Node
		for i := range ix.g.Nodes {
			n := &ix.g.Nodes[i]
			cur = &nf[i]
			if bare && ti < nameTiers && isTRName(n) {
				continue // çıplak TR adı trByName'in işidir
			}
			if (len(kinds) == 0 || contains(kinds, n.Kind)) && match(n) {
				found = append(found, n)
			}
		}
		if ti == foldTiers && trFolded {
			found = append(found, tr...) // TR'nin harfsiz adayları Go'nun harfsiz basamağıyla aynı sırada
		}
		if len(found) > 0 {
			if !trFolded {
				found = append(found, tr...) // TR bildirimleri Go'nun ilk uyan ad basamağıyla birlikte (`wu`: Go alanı ve TR değişkenleri)
			}
			return found
		}
	}
	return tr
}

// folds harfsiz karşılaştırmanın iki katlamasıdır: Türkçe kural (TR adları ve Türkçe metin: `I` → `ı`) ve Go'nun
// kuralı (Go adları, ASCII ile yazılmış sorgu: `I` → `i`). Biri tutunca eşleşir: `ifexpression` ast.IfExpression'ı,
// `IKINCI` ikinci'yi bulur (bağımsız denetim grup 7; yalnızca Türkçe katlama Go adlarında `I` ile başlayanları
// kaçırıyordu). Tam harfli basamaklar önce gelir: Go'da büyük-küçük harf anlamlıdır (dışa açık ad).
var folds = [2]func(string) string{codeindex.Fold, strings.ToLower}

// foldedNode bir düğümün adının, açıklamasının ve belge yorumunun iki katlamasıdır (folds sırasıyla).
type foldedNode struct {
	name, detail, doc [2]string
}

// nodeFolds bütün düğümlerin katlanmışlarıdır, ix.g.Nodes sırasıyla; ilk sorguda bir kez hesaplanır (her sorguda her
// düğümü yeniden katlamak sınavı dört kat yavaşlatıyordu).
func (ix *index) nodeFolds() []foldedNode {
	if ix.folded == nil {
		ix.folded = make([]foldedNode, len(ix.g.Nodes))
		for i := range ix.g.Nodes {
			n := &ix.g.Nodes[i]
			for k, f := range folds {
				ix.folded[i].name[k], ix.folded[i].detail[k], ix.folded[i].doc[k] = f(n.Name), f(n.Detail), f(n.Doc)
			}
		}
	}
	return ix.folded
}

// isTRName düğümün çıplak adı trByName ile mi çözülür: .tr dosyası, TR bildirimi ya da testi, ad kodu.
func isTRName(n *Node) bool {
	return strings.HasPrefix(n.ID, "tr:") || strings.HasPrefix(n.ID, "name:")
}

// trName çıplak adla çözülen bir TR düğümüdür: sınıfı ve karşılaştırılan adları (katlanmışlarıyla).
type trName struct {
	node   *Node
	class  codeindex.NameClass
	names  []string
	folded []string
}

// trNameTable grafiğin TR ad düğümlerini bir kez toplar. Adlar tr dizin'inkilerdir: modülün adı ve takma adları,
// bildirimin kısa adı (son parça), testin açıklaması, ad kodunun kendisi ve dillerdeki adları.
func (ix *index) trNameTable() []trName {
	if ix.trNames != nil {
		return ix.trNames
	}
	ix.trNames = []trName{}
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		c := trName{node: n}
		switch {
		case strings.HasPrefix(n.ID, "tr:file:"):
			c.class, c.names = codeindex.ClassModule, append([]string{n.Name}, n.Aliases...)
		case strings.HasPrefix(n.ID, "name:"):
			c.class, c.names = codeindex.ClassBuiltin, append([]string{n.Name}, n.Aliases...)
		case n.Kind == kindTRTest:
			c.class, c.names = codeindex.ClassTest, []string{n.Name[strings.Index(n.Name, ": ")+2:]} // "modül: açıklama"
		case strings.HasPrefix(n.ID, "tr:"):
			c.class, c.names = codeindex.ClassDeclaration, []string{n.Name[strings.LastIndex(n.Name, ".")+1:]}
		default:
			continue
		}
		for _, name := range c.names {
			c.folded = append(c.folded, codeindex.Fold(name))
		}
		ix.trNames = append(ix.trNames, c)
	}
	return ix.trNames
}

// trByName çıplak adın TR adaylarıdır; tr dizin'in Resolve'uyla aynı kural (codeindex.NameKeys harf kuralı,
// codeindex.PickByName sınıf sırası), böylece `query`, `tr dizin tanım` ve `tr dizin yeniden_adlandır` aynı aday kümesini
// görür (deneme 7 R1, bağımsız denetim grup 6 R1). folded adaylar büyük harfli sorgunun harfsiz geçişinden geldiyse
// true'dur. Grafikte tr dizin'in bilmediği ad kodları da vardır (anahtar sözcükler, kullanılmayan yerleşikler); onlar da
// yerleşik sınıfındadır.
func (ix *index) trByName(q string, kinds []string) (found []*Node, folded bool) {
	table := ix.trNameTable()
	for ki, k := range codeindex.NameKeys(q) {
		picked := codeindex.PickByName(len(table), func(i int) codeindex.NameClass {
			c := &table[i]
			if len(kinds) > 0 && !contains(kinds, c.node.Kind) {
				return codeindex.ClassNone
			}
			for j, name := range c.names {
				if k.Match(name, c.folded[j]) {
					return c.class
				}
			}
			return codeindex.ClassNone
		})
		for _, i := range picked {
			found = append(found, table[i].node)
		}
		if len(found) > 0 {
			return found, ki > 0
		}
	}
	return nil, false
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
	if n.Skip != "" {
		s += "  [skips when: " + n.Skip + "]" // yalnızca koşul yanlışken koşar (test_skips.go)
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
			if n.Skip != "" {
				s += "  [skips when: " + n.Skip + "]"
			}
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
	ix.printOtherConfigs(w, n, limit)
}

// printWhoUses bir düğümü kullananları yazar: gelen kenarlar (contains hariç), kullanım yeriyle.
func (ix *index) printWhoUses(w io.Writer, n *Node, limit int) {
	uses := ix.whoUsesHeader(w, n)
	if len(uses) == 0 {
		fmt.Fprintln(w, "  no uses")
		ix.printOtherConfigs(w, n, limit)
		return
	}
	ix.printGroups(w, "  <-", uses, fromOf, limit, true)
	ix.printOtherConfigs(w, n, limit)
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

// hasWord text, q'yu bütün bir sözcük olarak içeriyor mu: Türkçe harf katlamasıyla (İ/i, I/ı) karşılaştırılır, eşleşmenin iki
// yanında harf, rakam ya da alt çizgi olmamalıdır.
func hasWord(text, q string) bool {
	return hasWordFolded(folds[0](text), folds[0](q)) || hasWordFolded(folds[1](text), folds[1](q))
}

// hasWordFolded katlanmış metinde katlanmış sözcüğün sözcük sınırında geçip geçmediğidir.
func hasWordFolded(ft, fq string) bool {
	if fq == "" {
		return false
	}
	for start := 0; ; {
		i := strings.Index(ft[start:], fq)
		if i < 0 {
			return false
		}
		i += start
		before, _ := utf8.DecodeLastRuneInString(ft[:i])
		after, _ := utf8.DecodeRuneInString(ft[i+len(fq):])
		if !isWordRune(before) && !isWordRune(after) {
			return true
		}
		_, size := utf8.DecodeRuneInString(ft[i:])
		start = i + size
	}
}

func isWordRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
}

// otherConfigMention sorulan adın, grafiğin tiplerle okumadığı (bu yapılandırmada derlenmeyen) bir dosyada geçtiği yerdir.
type otherConfigMention struct {
	file string
	line int
	in   *Node // geçtiği bildirim (dosyada ondan önce başlayan son bildirim); yoksa nil
}

// otherConfigMentions n'nin kısa adının derlenmeyen dosyalarda sözcük olarak geçtiği yerlerdir (metin olarak okunur,
// tiplerle değil). Grafik oradaki çağrıları ve tür kullanımlarını görmez; cevap bu yerleri ayrıca söyler (DENETİM
// doğrulaması Y4: dataload.mmapFile'ın Linux tanımı ve onu sınayan TestMmapTruncatedDoesNotCrash görünmüyordu, "25 files"
// notu ise ilgisiz boş cevaplara da düşüyordu).
func (ix *index) otherConfigMentions(n *Node) (config string, out []otherConfigMention) {
	if !strings.HasPrefix(n.ID, "go:") || n.Kind == kindFile || n.Kind == kindPackage || ix.root == "" {
		return "", nil // dosya ve paket adı bir bildirim adı değildir ("text.go"nun son parçası "go" olurdu)
	}
	name := n.Name[strings.LastIndex(n.Name, ".")+1:]
	for i := range ix.g.Nodes {
		f := &ix.g.Nodes[i]
		if f.Kind != kindFile || !strings.HasPrefix(f.Detail, otherConfigDetail) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ix.root, filepath.FromSlash(f.Name)))
		if err != nil || !strings.Contains(string(data), name) {
			continue
		}
		config = strings.TrimPrefix(f.Detail, otherConfigDetail)
		type decl struct {
			line int
			node *Node
		}
		var decls []decl
		for _, e := range ix.out[f.ID] {
			if e.Kind == edgeContains && ix.byID[e.To] != nil {
				decls = append(decls, decl{e.Line, ix.byID[e.To]})
			}
		}
		sort.Slice(decls, func(a, b int) bool { return decls[a].line < decls[b].line })
		for no, line := range strings.Split(string(data), "\n") {
			if !hasWord(line, name) || strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue // yorum satırı kullanım değildir
			}
			m := otherConfigMention{file: f.Name, line: no + 1}
			for _, d := range decls {
				if d.line <= no+1 {
					m.in = d.node
				}
			}
			out = append(out, m)
		}
	}
	return config, out
}

// printOtherConfigs sorulan adın derlenmeyen dosyalarda geçtiği yerleri bildirime göre tek satırda yazar (testler
// işaretli); geçmiyorsa bir şey yazmaz.
func (ix *index) printOtherConfigs(w io.Writer, n *Node, limit int) {
	ix.printOtherConfigsExcept(w, n, limit, nil)
}

// printOtherConfigsExcept printOtherConfigs'tir; listed'deki testler (tests-for'un gruplarında yazılanlar) burada bir
// kez daha yazılmaz (bağımsız denetim grup 7: derlenmeyen dosyadaki test iki grupta çıkıyordu).
func (ix *index) printOtherConfigsExcept(w io.Writer, n *Node, limit int, listed []*Node) {
	config, ms := ix.otherConfigMentions(n)
	if len(ms) == 0 {
		return
	}
	var lines []string
	seen := map[string]bool{}
	for _, m := range ms {
		where := fmt.Sprintf("%s:%d", m.file, m.line)
		if m.in != nil && slices.Contains(listed, m.in) {
			continue
		}
		if m.in != nil {
			if seen[m.in.ID+m.file] {
				continue
			}
			seen[m.in.ID+m.file] = true
			where = fmt.Sprintf("%s [%s] at %s", m.in.Name, m.in.Kind, where)
			if isTestNode(&Node{Kind: m.in.Kind, Name: m.in.Name, File: m.file}) {
				where += "  [test]"
			}
		}
		lines = append(lines, where)
	}
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- in other build configurations (%d; files not built for %s, read as text, not type-checked):\n", len(lines), config)
	printLimited(w, lines, limit, "    ")
}

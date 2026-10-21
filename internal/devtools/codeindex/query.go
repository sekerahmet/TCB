package codeindex

import (
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// QueryOptions sorgunun ayarlarıdır.
type QueryOptions struct {
	Limit             int    // listelerdeki en çok satır (0: sınırsız)
	Kind              string // sorgu adını yalnızca bu türdeki düğümlere çöz (boş: hepsi)
	IncludeDefinition bool   // References: tanımın kendisi de listelensin (LSP includeDeclaration)
}

// maxTestSteps dolaylı sınamanın izlendiği en çok çağrı adımıdır.
const maxTestSteps = 4

// prepare bir sorguyu çözer. Belirsiz ya da bulunamayan adda hazır sonucu (Status ayarlı) ve nil
// verir; çözülürse sonucun iskeletini ve düğümü verir.
func (r *Reader) prepare(command, q string, o QueryOptions) (*Result, *Node) {
	res := NewResult(command, Query{Text: q, Language: codes.Display().Code})
	found := r.Resolve(q, o.Kind)
	switch len(found) {
	case 1:
		n := r.display(*found[0])
		res.Target = &n
		return res, found[0]
	case 0:
		res.Status = StatusNotFound
		for _, n := range r.Suggest(q, 6) {
			res.DidYouMean = append(res.DidYouMean, r.display(*n))
		}
		return res, nil
	}
	res.Status = StatusAmbiguous
	sorted := append([]*Node(nil), found...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, n := range sorted {
		res.Candidates = append(res.Candidates, r.display(*n))
	}
	return res, nil
}

// display düğümü çıktıya hazırlar: yerleşik adın okuyanın dilindeki adı (Display) eklenir.
func (r *Reader) display(n Node) Node {
	n.Imports = nil // dizin içi ayrıntı; sonuçta yer tutmaz
	if n.Kind == KindBuiltin {
		if k, err := codes.Parse(n.Name); err == nil {
			n.Display = codes.DisplayName(k)
		}
	}
	return n
}

// Definition bir adın tanımını verir: düğüm, belge yorumu ve (yapı ya da modülse) üyeleri.
func (r *Reader) Definition(q string, o QueryOptions) *Result {
	res, n := r.prepare(CommandDefinition, q, o)
	if n == nil {
		return res
	}
	res.Docs = n.Doc
	for _, e := range r.Index.Edges {
		if e.Kind == EdgeDefines && e.From == n.ID {
			res.Members = append(res.Members, r.display(*r.byID[e.To]))
		}
	}
	sort.SliceStable(res.Members, func(i, j int) bool { return posLess(res.Members[i].Pos, res.Members[j].Pos) })
	return res
}

// References düğüme başvuranları verir: rol, güven, konum ve başvuruyu içeren bildirim.
func (r *Reader) References(q string, o QueryOptions) *Result {
	res, n := r.prepare(CommandReferences, q, o)
	if n == nil {
		return res
	}
	uses := r.usesOf([]string{n.ID}, o.IncludeDefinition, "")
	r.fillUses(res, uses, o)
	return res
}

// usesOf düğümlere yapılan başvuruları (tanım dahil değilse tanımsız) dosya ve satır sırasıyla verir;
// outside doluysa o modülün içindeki başvurular atılır.
func (r *Reader) usesOf(ids []string, withDefinition bool, outside string) []Use {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var uses []Use
	for _, oc := range r.Index.Occurrences {
		if !want[oc.Node] || (oc.Role == RoleDefinition && !withDefinition) || (outside != "" && oc.Pos.File == outside) {
			continue
		}
		u := Use{Role: oc.Role, Confidence: oc.Confidence, Pos: oc.Pos}
		if in := r.byID[oc.In]; in != nil {
			d := r.display(*in)
			u.In = &d
		}
		uses = append(uses, u)
	}
	sort.SliceStable(uses, func(i, j int) bool { return posLess(&uses[i].Pos, &uses[j].Pos) })
	return uses
}

func (r *Reader) fillUses(res *Result, uses []Use, o QueryOptions) {
	counts := &Counts{Total: len(uses), ByRole: map[Role]int{}}
	for _, u := range uses {
		counts.ByRole[u.Role]++
	}
	res.Counts = counts
	res.Users, res.Truncated = limit(uses, o.Limit)
}

func limit[T any](list []T, n int) ([]T, bool) {
	if n > 0 && len(list) > n {
		return list[:n], true
	}
	return list, false
}

func posLess(a, b *Pos) bool {
	if a == nil || b == nil {
		return a != nil
	}
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

// TestedBy düğümü sınayan test bloklarını verir: önce doğrudan olanlar (testin kendisi çağırır),
// sonra çağrı zinciriyle dolaylı olanlar (en çok maxTestSteps adım; INFERRED, Steps adım sayısı),
// yakından uzağa.
func (r *Reader) TestedBy(q string, o QueryOptions) *Result {
	res, n := r.prepare(CommandTestedBy, q, o)
	if n == nil {
		return res
	}
	tests := r.testsOf([]string{n.ID})
	res.Counts = &Counts{Total: len(tests)}
	res.Tests, res.Truncated = limit(tests, o.Limit)
	return res
}

// testsOf düğümlerin doğrudan ve dolaylı testlerini verir (her test bir kez, en yakın haliyle).
func (r *Reader) testsOf(ids []string) []Use {
	var out []Use
	seenTest := map[string]bool{}
	for _, id := range ids {
		for _, oc := range r.Index.Occurrences {
			if oc.Node == id && oc.Role == RoleTest && !seenTest[oc.In] {
				seenTest[oc.In] = true
				u := Use{Role: RoleTest, Confidence: oc.Confidence, Pos: oc.Pos}
				if in := r.byID[oc.In]; in != nil {
					d := r.display(*in)
					u.In = &d
				}
				out = append(out, u)
			}
		}
	}
	// Dolaylı: hedeften çağıranları geriye doğru izle; ilk teste kadar.
	seen := map[string]bool{}
	frontier := append([]string(nil), ids...)
	for _, id := range ids {
		seen[id] = true
	}
	for step := 1; step <= maxTestSteps && len(frontier) > 0; step++ {
		var next []string
		for _, id := range frontier {
			for _, from := range r.callers[id] {
				if seen[from] {
					continue
				}
				seen[from] = true
				c := r.byID[from]
				if c == nil {
					continue
				}
				if c.Kind == KindTest {
					if !seenTest[from] {
						seenTest[from] = true
						d := r.display(*c)
						out = append(out, Use{Role: RoleTest, Confidence: Inferred, Pos: *c.Pos, In: &d, Steps: step})
					}
					continue
				}
				next = append(next, from)
			}
		}
		frontier = next
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Steps != out[j].Steps {
			return out[i].Steps < out[j].Steps
		}
		return posLess(&out[i].Pos, &out[j].Pos)
	})
	return out
}

// Impact bir ad ya da dosya değişirse etkilenenleri verir: adı (modülse içindeki bütün bildirimleri)
// dışarıdan kullananlar, modülü (dolaylı da) aktaran modüller ve bunları sınayan testler.
func (r *Reader) Impact(q string, o QueryOptions) *Result {
	res, n := r.prepare(CommandImpact, q, o)
	if n == nil {
		return res
	}
	mod := n.Pos.File
	ids := []string{n.ID}
	if n.Kind == KindModule {
		ids = r.declaredIn(n.Pos.File)
	}
	if n.Kind == KindBuiltin {
		mod = ""
	}
	aff := &Affected{}
	uses := r.usesOf(ids, false, mod)
	tests := r.testsOf(ids)
	if mod != "" {
		for _, m := range r.importClosure(NodeID(KindModule, mod, "")) {
			aff.Modules = append(aff.Modules, r.display(*r.byID[m]))
		}
	}
	for _, u := range tests {
		aff.Tests = append(aff.Tests, *u.In)
	}
	res.Counts = &Counts{Total: len(uses), ByRole: map[Role]int{}}
	for _, u := range uses {
		res.Counts.ByRole[u.Role]++
	}
	aff.Users, res.Truncated = limit(uses, o.Limit)
	aff.Modules, _ = limit(aff.Modules, o.Limit)
	aff.Tests, _ = limit(aff.Tests, o.Limit)
	res.Affected = aff
	return res
}

// declaredIn bir dosyadaki bütün bildirimlerin (modülün kendisi hariç) kimlikleridir.
func (r *Reader) declaredIn(file string) []string {
	var ids []string
	for _, nd := range r.Index.Nodes {
		if nd.Kind != KindModule && nd.Kind != KindBuiltin && nd.Pos != nil && nd.Pos.File == file {
			ids = append(ids, nd.ID)
		}
	}
	return ids
}

// importClosure modülü doğrudan ve dolaylı aktaran modüllerdir (kendisi hariç), kimliğe göre sıralı.
func (r *Reader) importClosure(module string) []string {
	seen := map[string]bool{module: true}
	queue := []string{module}
	var out []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, from := range r.importers[cur] {
			if !seen[from] {
				seen[from] = true
				out = append(out, from)
				queue = append(queue, from)
			}
		}
	}
	sort.Strings(out)
	return out
}

package codeindex

import (
	"slices"
	"sort"
)

// Reader bir dizini sorgulamak için hazırlar: kimlik tablosu ve dosya başına sıralı başvurular.
// Dizin değişmez; Reader onun üzerinde salt okunur bir görünümdür (dil sunucusu her istekte
// aynı Reader'ı kullanır, dosya değişince yeni Index'ten yenisini kurar).
type Reader struct {
	Index  *Index
	byID   map[string]*Node
	byFile map[string][]int // dosya -> Index.Occurrences sıraları (satır, sütun sırasıyla)
	// builtins yerleşik adı (her gömülü dilde) -> düğüm kimlikleri.
	builtins       map[string][]string
	builtinsFolded map[string][]string // aynısı, adlar Türkçe katlanmış (Fold)
	folded         []foldedNames       // Index.Nodes sırasıyla
	callers        map[string][]string // düğüm -> onu çağıran düğümler (calls kenarları, ters)
	importers      map[string][]string // modül -> onu aktaran modüller (imports kenarları, ters)
	// codeNames kodlu düğümlerin (Node.Code) kodu ve her yazım dilindeki adı -> düğüm kimlikleri; codeNamesFolded
	// aynısı Türkçe katlanmış.
	codeNames, codeNamesFolded map[string][]string
	parent                     map[string]string   // düğüm -> onu tanımlayan (defines kenarları, ters)
	ignored                    map[string]bool     // git'in yok saydığı dosyalar (Index.Ignored)
	children                   map[string][]string // düğüm -> tanımladıkları
}

// NewReader dizinden bir Reader kurar.
func NewReader(idx *Index) *Reader {
	r := &Reader{Index: idx, byID: make(map[string]*Node, len(idx.Nodes)), byFile: map[string][]int{}, parent: map[string]string{},
		children: map[string][]string{}, ignored: map[string]bool{}}
	for _, rel := range idx.Ignored {
		r.ignored[rel] = true
	}
	for i := range idx.Nodes {
		r.byID[idx.Nodes[i].ID] = &idx.Nodes[i]
	}
	for _, e := range idx.Edges {
		if e.Kind == EdgeDefines {
			r.parent[e.To] = e.From
			r.children[e.From] = append(r.children[e.From], e.To)
		}
	}

	r.builtins = builtinNames(idx)
	r.builtinsFolded = foldNames(r.builtins)
	r.codeNames = codeNames(idx)
	r.codeNamesFolded = foldNames(r.codeNames)
	r.folded = make([]foldedNames, len(idx.Nodes))
	for i := range idx.Nodes {
		n := &idx.Nodes[i]
		f := foldedNames{short: Fold(shortName(n)), qualified: Fold(n.Qualified)}
		for _, a := range n.Aliases {
			f.aliases = append(f.aliases, Fold(a))
		}
		f.qualifiedIn = r.qualifiedSpellings(n)
		r.folded[i] = f
	}
	r.callers, r.importers = map[string][]string{}, map[string][]string{}
	for _, e := range idx.Edges {
		switch e.Kind {
		case EdgeCalls, EdgeUses:
			// Zincire giren bağlar codegraph'ınkiyle aynıdır (test_reach.go reachEdge: TR tanımına çağrı ve kullanım;
			// işlev değer olarak geçip başka yerde çağrılabilir; yerleşik ada kullanım grafikte uses_name'dir, zincire
			// girmez, yerleşiği yalnızca çağıran test sınar). İki aracın sınayan kümesi aynı olur (DENETİM doğrulaması T3).
			if e.Kind == EdgeUses && r.byID[e.To] != nil && r.byID[e.To].Kind == KindBuiltin {
				continue
			}
			r.callers[e.To] = append(r.callers[e.To], e.From)
		case EdgeImports:
			r.importers[e.To] = append(r.importers[e.To], e.From)
		}
	}
	for i, o := range idx.Occurrences {
		r.byFile[o.Pos.File] = append(r.byFile[o.Pos.File], i)
	}
	for _, list := range r.byFile {
		sort.SliceStable(list, func(a, b int) bool {
			pa, pb := idx.Occurrences[list[a]].Pos, idx.Occurrences[list[b]].Pos
			if pa.Line != pb.Line {
				return pa.Line < pb.Line
			}
			return pa.Col < pb.Col
		})
	}
	return r
}

// foldNames ad -> kimlikler eşlemesinin adları Türkçe katlanmış (Fold) karşılığıdır.
func foldNames(names map[string][]string) map[string][]string {
	folded := map[string][]string{}
	for name, ids := range names {
		f := Fold(name)
		for _, id := range ids {
			if !slices.Contains(folded[f], id) {
				folded[f] = append(folded[f], id)
			}
		}
	}
	return folded
}

// Node kimliği verilen düğümdür (yoksa nil).
func (r *Reader) Node(id string) *Node { return r.byID[id] }

// At dosyada satır ve sütundaki başvuruyu ve işaret ettiği düğümü verir: "imlecin altındaki ad
// neye başvuruyor". Konum adın kapsadığı aralıktaysa (sütun ≥ başlangıç ve < bitiş) eşleşir;
// bulunamazsa nil döner. Dil sunucusunun tanıma git, kullanımları bul ve yeniden adlandırma
// isteklerinin başlangıcıdır.
func (r *Reader) At(file string, line, col int) (*Occurrence, *Node) {
	for _, i := range r.byFile[file] {
		o := &r.Index.Occurrences[i]
		if o.Pos.Line != line || o.Basis == BasisNameOnly {
			continue
		}
		if col >= o.Pos.Col && (o.Pos.EndCol == 0 || col < o.Pos.EndCol) {
			return o, r.byID[o.Node]
		}
	}
	return nil, nil
}

// Occurrences bir düğüme yapılan bütün başvurulardır (tanım dahil), dosya ve satır sırasıyla. Alan ve metotta alıcısı
// bilinmeyen aynı adlı erişimler (MemberGuess) burada "olası" başvuruya (BasisNameOnly) çevrilir.
func (r *Reader) Occurrences(id string) []Occurrence {
	var out []Occurrence
	for _, o := range r.Index.Occurrences {
		if o.Node == id {
			out = append(out, o)
		}
	}
	if guesses := r.memberGuesses(id); len(guesses) > 0 {
		out = append(out, guesses...)
		sort.SliceStable(out, func(i, j int) bool { return comparePos(out[i].Pos, out[j].Pos) < 0 })
	}
	return out
}

// memberGuesses alanın ya da metodun ad-yalnız başvurularıdır: aynı adlı, alanın (metodun) dosyasını gören dosyadaki
// alıcısı bilinmeyen erişimler; metot için yalnız aday sınırının aşıldığı erişimler (sınır içindekilerin kenarı ve
// BasisNameMatch başvurusu dizindedir). Görünürlük tanımın dosyasından ters aktarma bağlarında tek gezintiyle bulunur
// (sorgu başına O(dosya + aktarma)); dizine dosya başına görünürlük listesi yazılmaz.
func (r *Reader) memberGuesses(id string) []Occurrence {
	n := r.byID[id]
	if n == nil || n.Pos == nil || n.Kind != KindField && n.Kind != KindMethod {
		return nil
	}
	name := shortName(n)
	var seers map[string]bool
	var out []Occurrence
	for _, g := range r.Index.MemberGuesses {
		if g.Member != name || n.Kind == KindMethod && !g.MethodsCapped {
			continue
		}
		if seers == nil {
			seers = r.seers(n.Pos.File)
		}
		if !seers[g.Pos.File] {
			continue
		}
		role := RoleRead
		switch {
		case n.Kind == KindMethod && g.Called:
			role = RoleCall
		case g.Written:
			role = RoleWrite
		}
		out = append(out, Occurrence{Node: id, Role: role, Confidence: Inferred, Pos: g.Pos, In: g.In, Basis: BasisNameOnly})
	}
	return out
}

// seers file'ı gören dosyalardır: kendisi ve onu (dolaylı da) aktaranlar.
func (r *Reader) seers(file string) map[string]bool {
	out := map[string]bool{file: true}
	queue := []string{NodeID(KindModule, file, "")}
	seen := map[string]bool{queue[0]: true}
	for len(queue) > 0 {
		mod := queue[0]
		queue = queue[1:]
		for _, from := range r.importers[mod] {
			if !seen[from] {
				seen[from] = true
				queue = append(queue, from)
				if m := r.byID[from]; m != nil && m.Pos != nil {
					out[m.Pos.File] = true
				}
			}
		}
	}
	return out
}

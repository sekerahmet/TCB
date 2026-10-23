package codeindex

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// refresh önbellekteki dizini güncel dosyalara göre tazeler. Dosya kümesi aynı kalmışsa çalışır
// (içerik değişikliği); aksi halde ya da artımlı yol güvenle kurulamazsa ok=false döner ve çağıran
// baştan kurar. Değişmeyen dosyaların düğümleri, bağları, başvuruları önbellekten alınır.
func refresh(root string, opts Options, prev *cached, cur map[string]manifestFile) (*Index, Stats, bool) {
	prevProject := 0
	for rel := range prev.man.Files {
		if !strings.HasPrefix(rel, libraryPrefix) {
			prevProject++
		}
	}
	if prevProject != len(cur) {
		return nil, Stats{}, false
	}
	var changed []string
	for rel, c := range cur {
		p, ok := prev.man.Files[rel]
		if !ok {
			return nil, Stats{}, false
		}
		if p.Hash != c.Hash {
			changed = append(changed, rel)
		}
	}
	man := manifest{IndexVersion: IndexVersion, Schema: SchemaVersion, Signature: opts.signature(), Files: map[string]manifestFile{}}
	for rel, c := range cur {
		man.Files[rel] = c
	}
	for rel, p := range prev.man.Files {
		if !strings.HasPrefix(rel, libraryPrefix) {
			continue
		}
		h := libraryHash(opts, rel)
		if h == "" {
			return nil, Stats{}, false // kitaplıktan kalkmış modül
		}
		man.Files[rel] = manifestFile{Hash: h}
		if h != p.Hash {
			changed = append(changed, rel)
		}
	}
	sort.Strings(changed)
	if len(changed) == 0 {
		_ = saveCache(root, man, nil) // yalnızca manifest: değişme zamanları yenilensin (içerik aynı)
		return prev.index, Stats{Files: len(prev.index.Files), FromCache: true}, true
	}
	idx, parsed, ok := incrementalBuild(root, opts, prev.index, changed)
	if !ok {
		return nil, Stats{}, false
	}
	_ = saveCache(root, man, idx)
	return idx, Stats{Files: len(idx.Files), Parsed: parsed}, true
}

// nodeFile bir düğümün dosyasıdır (yerleşik adın dosyası yoktur).
func nodeFile(n *Node) string {
	if n.Pos == nil {
		return ""
	}
	return n.Pos.File
}

// incrementalBuild changed dosyalarını ve onları (dolaylı da) aktaran dosyaları yeniden ayrıştırıp
// çözer, öbür dosyaların sonuçlarını önceki dizinden alır. Sonuç sıfırdan kurulanla aynıdır;
// bunu güvenle sağlayamazsa (gömülü kitaplık modül kümesi değişti) ok=false döner.
func incrementalBuild(root string, opts Options, prev *Index, changed []string) (*Index, int, bool) {
	prevNodes := make(map[string]*Node, len(prev.Nodes))
	for i := range prev.Nodes {
		prevNodes[prev.Nodes[i].ID] = &prev.Nodes[i]
	}
	dirty := dirtyClosure(prev, changed)
	x := &indexer{root: root, lang: source.Turkish(), library: opts.Library, files: map[string]*file{},
		nodes: map[string]*Node{}, edges: map[edgeKey]*Edge{}, constructSeen: map[string]bool{}}
	for _, n := range prev.Nodes {
		if !dirty[nodeFile(&n)] {
			c := n
			x.nodes[n.ID] = &c
		}
	}
	for _, e := range prev.Edges {
		if from := prevNodes[e.From]; from != nil && !dirty[nodeFile(from)] {
			c := e
			x.edges[edgeKey{e.From, e.To, e.Kind}] = &c
		}
	}
	for _, o := range prev.Occurrences {
		if !dirty[o.Pos.File] {
			x.occ = append(x.occ, o)
		}
	}
	for _, c := range prev.Constructs {
		if !dirty[c.Pos.File] {
			x.constructs = append(x.constructs, c)
			x.constructSeen[c.In+"|"+c.Kind+"|"+c.Op+"|"+c.Operand] = true
		}
	}
	for _, u := range prev.Unresolved {
		if !dirty[u.Pos.File] {
			x.unresolved = append(x.unresolved, u)
		}
	}
	for _, s := range prev.Skipped {
		if !dirty[s.File] {
			x.skipped = append(x.skipped, s)
		}
	}
	for _, rel := range prev.Files {
		if !dirty[rel] {
			x.files[rel] = stubFile(rel, prev)
		}
	}
	order := sortedKeys(dirty)
	for _, rel := range order {
		data, err := readInput(root, opts, rel)
		if err != nil && strings.HasPrefix(rel, libraryPrefix) {
			return nil, 0, false
		}
		x.addFile(rel, data, err)
	}
	dirtyOrder := append([]string(nil), x.order...)
	x.addLibrary()
	for _, rel := range x.order {
		if !dirty[rel] {
			return nil, 0, false // yeni bir kitaplık modülü girdi: baştan kur
		}
	}
	for _, rel := range dirtyOrder {
		x.declare(x.files[rel])
	}
	for _, rel := range dirtyOrder {
		x.resolveImports(x.files[rel])
	}
	x.inferReturns(dirtyOrder)
	for _, rel := range dirtyOrder {
		x.resolve(x.files[rel])
	}
	x.order = finalOrder(prev, x)
	if !sameLibrary(prev, x) {
		return nil, 0, false
	}
	pruneBuiltins(x)
	return x.finish(), len(dirtyOrder), true
}

// dirtyClosure değişen dosyalarla onları doğrudan ya da dolaylı aktaran dosyalardır.
func dirtyClosure(prev *Index, changed []string) map[string]bool {
	importers := map[string][]string{}
	for _, n := range prev.Nodes {
		if n.Kind != KindModule {
			continue
		}
		for _, imp := range n.Imports {
			if imp.Target != "" {
				importers[imp.Target] = append(importers[imp.Target], n.Pos.File)
			}
		}
	}
	dirty := map[string]bool{}
	queue := append([]string(nil), changed...)
	for len(queue) > 0 {
		rel := queue[0]
		queue = queue[1:]
		if dirty[rel] {
			continue
		}
		dirty[rel] = true
		queue = append(queue, importers[rel]...)
	}
	return dirty
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readInput bir girdi dosyasının içeriğidir: diskten ya da (sanal yolsa) gömülü kitaplıktan.
func readInput(root string, opts Options, rel string) ([]byte, error) {
	if strings.HasPrefix(rel, libraryPrefix) {
		return libraryBytes(opts, rel)
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
}

// stubFile değişmeyen bir dosyayı önceki dizinden kurar: ayrıştırma ağacı yoktur, yalnızca
// diğer dosyaların ad çözümlemesi için gereken tanımlar, metotlar ve aktarmalar vardır.
func stubFile(rel string, prev *Index) *file {
	f := &file{rel: rel, base: strings.TrimSuffix(filepath.Base(rel), ".tr"), defs: map[string]string{},
		methods: map[string][]string{}, fields: map[string][]string{}, aliases: map[string]*file{}}
	var top []Node
	for _, n := range prev.Nodes {
		if nodeFile(&n) != rel {
			continue
		}
		switch n.Kind {
		case KindModule:
			for _, imp := range n.Imports {
				f.imports = append(f.imports, importRef{file: imp.Target, alias: imp.Alias, pos: imp.Pos, nested: imp.Nested})
			}
		case KindFunction, KindStruct, KindConstant, KindVariable:
			top = append(top, n)
		case KindField:
			name := n.Name[strings.LastIndex(n.Name, ".")+1:]
			f.fields[name] = append(f.fields[name], n.ID)
		case KindMethod:
			name := n.Name[strings.LastIndex(n.Name, ".")+1:]
			f.methods[name] = append(f.methods[name], n.ID)
		}
	}
	sort.SliceStable(top, func(i, j int) bool { return top[i].Pos.Line < top[j].Pos.Line })
	for _, n := range top {
		f.defs[n.Name] = n.ID
	}
	return f
}

// finalOrder dizinin dosya listesidir: projedeki ayrıştırılabilen dosyalar yol sırasıyla, sonra
// sanal kitaplık modülleri önceki sırayla (sıfırdan kurulumdaki sıra).
func finalOrder(prev *Index, x *indexer) []string {
	var project, virtual []string
	seen := map[string]bool{}
	add := func(rel string) {
		if seen[rel] {
			return
		}
		seen[rel] = true
		if strings.HasPrefix(rel, libraryPrefix) {
			virtual = append(virtual, rel)
		} else {
			project = append(project, rel)
		}
	}
	for _, rel := range prev.Files {
		add(rel)
	}
	for _, s := range prev.Skipped {
		add(s.File)
	}
	sort.Strings(project)
	var out []string
	for _, rel := range project {
		if x.files[rel] != nil {
			out = append(out, rel)
		}
	}
	for _, rel := range virtual {
		if x.files[rel] != nil {
			out = append(out, rel)
		}
	}
	return out
}

// sameLibrary dizinin aktarma bağlarının gösterdiği sanal modül kümesi önceki dizindekiyle aynı mı:
// değilse artımlı sonuç sıfırdan kurulandan ayrışırdı.
func sameLibrary(prev *Index, x *indexer) bool {
	now := map[string]bool{}
	for k := range x.edges {
		if k.kind == EdgeImports && strings.HasPrefix(k.to, "module:"+libraryPrefix) {
			now[k.to] = true
		}
	}
	before := map[string]bool{}
	for _, e := range prev.Edges {
		if e.Kind == EdgeImports && strings.HasPrefix(e.To, "module:"+libraryPrefix) {
			before[e.To] = true
		}
	}
	if len(now) != len(before) {
		return false
	}
	for id := range now {
		if !before[id] {
			return false
		}
	}
	return true
}

// pruneBuiltins artık hiçbir bağın hedefi olmayan yerleşik düğümleri atar (sıfırdan kurulumda
// yerleşik düğümü yalnızca başvuru varsa vardır).
func pruneBuiltins(x *indexer) {
	used := map[string]bool{}
	for k := range x.edges {
		used[k.to] = true
	}
	for id, n := range x.nodes {
		if n.Kind == KindBuiltin && !used[id] {
			delete(x.nodes, id)
		}
	}
}

package codeindex

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Options dizinin nereden kurulacağını söyler.
type Options struct {
	// Dirs .tr dosyalarının aranacağı klasörlerdir (köke göre, "/" ile). Boşsa kökün kendisi.
	Dirs []string
	// SkipDirs gezilmeyen klasör adlarıdır (nil ise DefaultSkipDirs).
	SkipDirs map[string]bool
}

// DefaultSkipDirs sürüm denetimi, araç ve örnek verisi klasörleridir.
var DefaultSkipDirs = map[string]bool{
	".git": true, ".claude": true, ".codegraph": true, "node_modules": true, "testdata": true,
}

// file ayrıştırılmış bir .tr dosyasıdır.
type file struct {
	rel, base string
	res       *parsed
	lines     []string          // gösterim metninin satırları (satır numaraları bununla uyumlu)
	defs      map[string]string // üst düzey ad -> düğüm kimliği (işlev, yapı, sabit, değişken)
	methods   map[string][]string
	imports   []importRef
	visible   map[string]string // ad -> kimlik: kendi tanımları ve takma adsız aktarılan modüllerinki
	aliases   map[string]*file
}

// importRef bir `aktar` ifadesidir; file çözülemezse "".
type importRef struct {
	file, alias string
	pos         Pos
}

type edgeKey struct {
	from, to string
	kind     EdgeKind
}

// indexer dizini kurarken biriken durumdur.
type indexer struct {
	root    string
	lang    *codes.Language
	files   map[string]*file
	order   []string
	nodes   map[string]*Node
	edges   map[edgeKey]*Edge
	occ     []Occurrence
	skipped []Skipped
}

// Build root altındaki .tr dosyalarından dizini kurar. Okunamayan ya da ayrıştırılamayan dosya
// Index.Skipped'a yazılır (modül düğümü kalır, bildirimleri yoktur); hata değildir.
func Build(root string, opts Options) (*Index, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	x := &indexer{root: abs, lang: source.Turkish(), files: map[string]*file{},
		nodes: map[string]*Node{}, edges: map[edgeKey]*Edge{}}
	skip := opts.SkipDirs
	if skip == nil {
		skip = DefaultSkipDirs
	}
	for _, rel := range x.discover(opts.Dirs, skip) {
		x.read(rel)
	}
	for _, rel := range x.order {
		x.declare(x.files[rel])
	}
	for _, rel := range x.order {
		x.resolveImports(x.files[rel])
	}
	for _, rel := range x.order {
		x.resolve(x.files[rel])
	}
	return x.finish(), nil
}

// discover .tr dosyalarının köke göre yollarını sıralı verir.
func (x *indexer) discover(dirs []string, skip map[string]bool) []string {
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	var rels []string
	for _, top := range dirs {
		dir := filepath.Join(x.root, filepath.FromSlash(top))
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != x.root && skip[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".tr") {
				r, _ := filepath.Rel(x.root, p)
				rels = append(rels, filepath.ToSlash(r))
			}
			return nil
		})
	}
	sort.Strings(rels)
	return dedupe(rels)
}

func dedupe(sorted []string) []string {
	var out []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// read bir dosyayı okur ve modül düğümünü ekler; ayrıştırılamıyorsa atlananlara yazar.
func (x *indexer) read(rel string) {
	base := strings.TrimSuffix(path.Base(rel), ".tr")
	aliases := []string{base}
	if rest, ok := strings.CutPrefix(rel, "lib/"); ok {
		aliases = append(aliases, "tr/"+strings.TrimSuffix(rest, ".tr"))
	}
	mod := &Node{ID: NodeID(KindModule, rel, ""), Kind: KindModule, Name: rel, Pos: &Pos{File: rel, Line: 1}, Aliases: aliases}
	x.nodes[mod.ID] = mod
	data, err := os.ReadFile(filepath.Join(x.root, filepath.FromSlash(rel)))
	if err != nil {
		x.skipped = append(x.skipped, Skipped{File: rel, Reason: err.Error()})
		return
	}
	res := readFile(string(data))
	mod.Coded = res.coded
	if len(res.errs) > 0 {
		x.skipped = append(x.skipped, Skipped{File: rel, Reason: firstLine(res.errs[0])})
		return
	}
	x.files[rel] = &file{rel: rel, base: base, res: res, lines: strings.Split(res.text, "\n"),
		defs: map[string]string{}, methods: map[string][]string{}, aliases: map[string]*file{}}
	x.order = append(x.order, rel)
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// addEdge bağı ekler; aynı (from, to, kind) tekrar gelirse sayaç artar ve güven EXTRACTED'a
// yükselir (bir yerde kesin okunan bağ, başka yerdeki tahminden güçlüdür).
func (x *indexer) addEdge(from, to string, kind EdgeKind, conf Confidence, pos Pos) {
	k := edgeKey{from, to, kind}
	if e, ok := x.edges[k]; ok {
		e.Count++
		if conf == Extracted {
			e.Confidence = Extracted
		}
		return
	}
	p := pos
	x.edges[k] = &Edge{From: from, To: to, Kind: kind, Confidence: conf, Pos: &p, Count: 1}
}

// finish sıralı, tutarlı dizini kurar: uç düğümü olmayan kenarlar atılır.
func (x *indexer) finish() *Index {
	idx := &Index{Schema: SchemaVersion, Version: IndexVersion, Files: []string{}, Nodes: []Node{},
		Edges: []Edge{}, Occurrences: []Occurrence{}, Skipped: x.skipped}
	idx.Files = append(idx.Files, x.order...)
	for _, n := range x.nodes {
		idx.Nodes = append(idx.Nodes, *n)
	}
	sort.Slice(idx.Nodes, func(i, j int) bool { return idx.Nodes[i].ID < idx.Nodes[j].ID })
	for k, e := range x.edges {
		if x.nodes[k.from] != nil && x.nodes[k.to] != nil {
			idx.Edges = append(idx.Edges, *e)
		}
	}
	sort.Slice(idx.Edges, func(i, j int) bool {
		a, b := idx.Edges[i], idx.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	for _, o := range x.occ {
		if x.nodes[o.Node] != nil {
			idx.Occurrences = append(idx.Occurrences, o)
		}
	}
	sort.SliceStable(idx.Occurrences, func(i, j int) bool {
		a, b := idx.Occurrences[i].Pos, idx.Occurrences[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return idx
}

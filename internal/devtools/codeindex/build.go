package codeindex

import (
	"cmp"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Options dizinin nereden kurulacağını söyler.
type Options struct {
	// Dirs .tr dosyalarının aranacağı klasörlerdir (köke göre, "/" ile). Boşsa kökün kendisi.
	Dirs []string
	// SkipDirs gezilmeyen klasör adlarıdır (nil ise DefaultSkipDirs).
	SkipDirs map[string]bool
	// Tracked doğruysa yalnızca git'in izlediği .tr dosyaları dizine girer (depo aracı: codegraph); yanlışsa (tr dizin)
	// kökün altındaki her .tr, .gitignore'a bakılmadan (ListFiles kipleri).
	Tracked bool
	// Library gömülü TR kitaplığıdır (kökünde katman.tr gibi dosyalar): projede lib/ad.tr yoksa
	// `aktar "tr/ad"` buraya çözülür ve modül "tr:lib/ad.tr" yoluyla dizine girer. Boşsa yok.
	Library fs.FS
}

// DefaultSkipDirs sürüm denetimi, araç çıktısı, bağımlılık ve sınama verisi klasörleridir; depo araçlarının tek atlama
// listesi (ListFiles).
var DefaultSkipDirs = map[string]bool{
	".git": true, ".claude": true, ".codegraph": true, CacheDir: true, "node_modules": true, "testdata": true, "vendor": true,
}

// file ayrıştırılmış bir .tr dosyasıdır.
type file struct {
	rel, base string
	res       *parsed
	lines     []string          // diskteki satırlar (parsed.lines; satır numaraları dosyanınki)
	defs      map[string]string // üst düzey ad -> düğüm kimliği (işlev, yapı, sabit, değişken)
	methods   map[string][]string
	fields    map[string][]string // alan adı -> alan düğümlerinin kimlikleri (alıcı tipi bilinmeyen erişimlerin adayları)
	imports   []importRef
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
	root          string
	lang          *codes.Language
	files         map[string]*file
	order         []string
	nodes         map[string]*Node
	edges         map[edgeKey]*Edge
	occ           []Occurrence
	guesses       []MemberGuess   // alıcısı bilinmeyen üye erişimleri (bind.go: noteMemberGuess)
	declared      map[string]bool // projede bir dosyanın üst düzeyde bildirdiği adlar (lookupVisible; tembel)
	methodNames   map[string]bool // projede bir yapının metot adları (methodCandidates; tembel)
	skipped       []Skipped
	unresolved    []Unresolved
	library       fs.FS
	bodies        map[string]fnBody // dönüş türü çıkarımı için işlev gövdeleri (infer.go)
	constructs    []Construct       // bildirimlerin kullandığı dil yapıları (constructs.go)
	constructSeen map[string]bool
	retDone       map[string]bool
	paramOrder    map[string][]string // işlev ya da metot -> parametreleri başlıktaki sırayla (paramsInOrder; tembel)
	snippet       bool                // metin başlıksız bir program gövdesidir (Snippet: Go testinin dizesi), kodlu okunur
}

// Build root altındaki .tr dosyalarından dizini kurar. Okunamayan ya da ayrıştırılamayan dosya
// Index.Skipped'a yazılır (modül düğümü kalır, bildirimleri yoktur); hata değildir.
func Build(root string, opts Options) (*Index, error) {
	if err := languageReady(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	x := &indexer{root: abs, lang: identities(), library: opts.Library, files: map[string]*file{},
		nodes: map[string]*Node{}, edges: map[edgeKey]*Edge{}, constructSeen: map[string]bool{}}
	skip := opts.SkipDirs
	if skip == nil {
		skip = DefaultSkipDirs
	}
	for _, rel := range x.discover(opts, skip) {
		x.read(rel)
	}
	x.addLibrary()
	for _, rel := range x.order {
		x.declare(x.files[rel])
	}
	for _, rel := range x.order {
		x.resolveImports(x.files[rel])
	}
	x.inferReturns(x.order)
	for _, rel := range x.order {
		x.resolve(x.files[rel])
	}
	idx := x.finish()
	markIgnored(abs, idx, opts)
	return idx, nil
}

// discover .tr dosyalarının köke göre yollarını sıralı verir (ListFiles; opts.Tracked kipi seçer).
func (x *indexer) discover(opts Options, skip map[string]bool) []string {
	set := AllFiles
	if opts.Tracked {
		set = TrackedFiles
	}
	return ListFiles(x.root, opts.Dirs, skip, set, func(rel string) bool { return strings.HasSuffix(rel, ".tr") })
}

// read diskteki bir dosyayı okur ve modül düğümünü ekler.
func (x *indexer) read(rel string) {
	data, err := os.ReadFile(filepath.Join(x.root, filepath.FromSlash(rel)))
	x.addFile(rel, data, err)
}

// addFile bir dosyanın metninden modül düğümünü ve (ayrıştırılabiliyorsa) dosya kaydını kurar;
// ayrıştırılamıyorsa atlananlara yazar.
func (x *indexer) addFile(rel string, data []byte, readErr error) {
	base := strings.TrimSuffix(path.Base(rel), ".tr")
	aliases := []string{base}
	if rest, ok := strings.CutPrefix(rel, "lib/"); ok {
		aliases = append(aliases, "tr/"+strings.TrimSuffix(rest, ".tr"))
	} else if rest, ok := strings.CutPrefix(rel, libraryPrefix+"lib/"); ok {
		aliases = append(aliases, "tr/"+strings.TrimSuffix(rest, ".tr"))
	}
	mod := &Node{ID: NodeID(KindModule, rel, ""), Kind: KindModule, Name: rel, Pos: &Pos{File: rel, Line: 1}, Aliases: aliases}
	x.nodes[mod.ID] = mod
	if readErr != nil {
		x.skipped = append(x.skipped, Skipped{File: rel, Reason: readErr.Error()})
		return
	}
	res := readFile(string(data), x.snippet)
	mod.Coded = res.coded
	if len(res.errs) > 0 {
		x.skipped = append(x.skipped, Skipped{File: rel, Reason: firstLine(res.errs[0])})
		return
	}
	x.files[rel] = &file{rel: rel, base: base, res: res, lines: res.lines,
		defs: map[string]string{}, methods: map[string][]string{}, fields: map[string][]string{}, aliases: map[string]*file{}}
	x.order = append(x.order, rel)
}

// libraryPrefix gömülü kitaplık modüllerinin yol önekidir ("tr:lib/katman.tr").
const libraryPrefix = "tr:"

// addLibrary projenin `aktar "tr/ad"` ile aldığı ama kendi lib/ klasöründe bulunmayan modülleri
// gömülü kitaplıktan okur (kitaplığın kendi aktardıkları da, durağan noktaya kadar).
func (x *indexer) addLibrary() {
	if x.library == nil {
		return
	}
	for changed := true; changed; {
		changed = false
		for _, rel := range append([]string(nil), x.order...) {
			for _, s := range x.files[rel].res.program.Statements {
				imp, ok := s.(*ast.ImportStatement) // aktarma yalnız dosyanın başındadır (2.1)
				if !ok {
					continue
				}
				name, ok := strings.CutPrefix(strings.TrimSuffix(imp.Path, ".tr"), "tr/")
				if !ok || x.nodes[NodeID(KindModule, "lib/"+name+".tr", "")] != nil {
					continue
				}
				virtual := libraryPrefix + "lib/" + name + ".tr"
				if x.nodes[NodeID(KindModule, virtual, "")] != nil {
					continue
				}
				if data, err := fs.ReadFile(x.library, name+".tr"); err == nil {
					x.addFile(virtual, data, nil)
					changed = true
				}
			}
		}
	}
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
		Edges: []Edge{}, Occurrences: []Occurrence{}, Skipped: x.skipped, Unresolved: x.unresolved}
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
	slices.SortFunc(idx.Edges, func(a, b Edge) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To), cmp.Compare(a.Kind, b.Kind))
	})
	for _, o := range x.occ {
		if x.nodes[o.Node] != nil {
			idx.Occurrences = append(idx.Occurrences, o)
		}
	}
	sort.SliceStable(idx.Occurrences, func(i, j int) bool {
		a, b := idx.Occurrences[i], idx.Occurrences[j]
		if c := comparePos(a.Pos, b.Pos); c != 0 {
			return c < 0
		}
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		return a.Role < b.Role
	})
	idx.MemberGuesses = append(idx.MemberGuesses, x.guesses...)
	sort.SliceStable(idx.MemberGuesses, func(i, j int) bool {
		a, b := idx.MemberGuesses[i], idx.MemberGuesses[j]
		if c := comparePos(a.Pos, b.Pos); c != 0 {
			return c < 0
		}
		return a.Member < b.Member
	})
	sort.SliceStable(idx.Unresolved, func(i, j int) bool {
		a, b := idx.Unresolved[i], idx.Unresolved[j]
		if c := comparePos(a.Pos, b.Pos); c != 0 {
			return c < 0
		}
		return a.Name < b.Name
	})
	sort.SliceStable(idx.Skipped, func(i, j int) bool { return idx.Skipped[i].File < idx.Skipped[j].File })
	for _, c := range x.constructs {
		if x.nodes[c.In] != nil {
			idx.Constructs = append(idx.Constructs, c)
		}
	}
	sort.SliceStable(idx.Constructs, func(i, j int) bool {
		a, b := idx.Constructs[i], idx.Constructs[j]
		if c := comparePos(a.Pos, b.Pos); c != 0 {
			return c < 0
		}
		return a.In+a.Kind+a.Op+a.Operand < b.In+b.Kind+b.Op+b.Operand
	})
	return idx
}

// languageReady gömülü Türkçe liste kurulamadıysa (bozuk ya da yarım değişmiş tr.json) nedenini
// verir: dizin kimliklere düşmüş adlarla kurulup yanlış cevap vermesin, çağıran Go tarafıyla
// sürer (codegraph) ya da durur (tr dizin).
func languageReady() error {
	_, err := codes.LoadLanguage(codes.SourceLanguage)
	return err
}

// FindRoot başlangıç klasöründen yukarı doğru proje kökünü arar: `.git` (klasör ya da dosya)
// bulunan ilk klasör; yoksa başlangıç klasörünün kendisi. `--kök` verilmediğinde kullanılır.
func FindRoot(start string) string {
	abs, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for dir := abs; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		dir = parent
	}
}

// CacheDir dizin önbelleğinin proje köküne göre klasörüdür (git'e girmez).
const CacheDir = ".tr-dizin"

// ClearCache önbellek klasörünü siler; vardıysa true döner.
func ClearCache(root string) (bool, error) {
	dir := filepath.Join(root, CacheDir)
	if _, err := os.Stat(dir); err != nil {
		return false, nil
	}
	return true, os.RemoveAll(dir)
}

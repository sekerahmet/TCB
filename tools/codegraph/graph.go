package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// schemaVersion graph.json biçimi değişince artar; okuyucular bilmedikleri sürümü reddeder.
const schemaVersion = 1

// extractorRevision çıkarıcıların ürettiği içerik değişince artar (yeni alan, yeni kenar, farklı
// eşleme). Eski bir sürümün kurduğu grafik, imzası aynı olsa da bir sorguda yeniden kurulur.
// 2: düğümlere son satır (end) eklendi. 3: paket düzeyi const/var düğümleri ve `uses`, `denotes` kenarları. 4: ad kodu düğümlerinde dildeki adlar (aliases). 5: .tr kaynakları (tr_* düğümleri, TR çağrı, kullanım ve ad kodu kenarları).
// 6: struct alanı düğümleri ve okuma/yazma erişim yerleri, func literal düğümleri ve yakaladıkları değişkenler.
// 7: .tr kaynakları etkin yazım dilinden bağımsız okunur (TR_DIL=en grafiği eksik bırakıyordu).
// 8: sabit, değişken ve alan düğümlerinin bitiş satırı kendi değeriyle biter (öncekiler sonraki bloğun sonuna uzanıyordu).
// 9: aynı modül hem takma adsız hem takma adla aktarılınca takma ad tanınır (yutuluyordu).
const extractorRevision = 30

// Düğüm türleri.
const (
	kindPackage = "package"
	kindFile    = "file"
	kindFunc    = "func"
	kindMethod  = "method"
	kindType    = "type"
	kindConst   = "const"
	kindField   = "field"
	kindClosure = "closure"
	kindVar     = "var"
	kindMessage = "message"
	kindName    = "name"
	kindRule    = "rule"
	kindDoc     = "doc"
	kindHeading = "heading"
)

// Kenar türleri.
const (
	edgeContains    = "contains"
	edgeImports     = "imports"
	edgeCalls       = "calls"
	edgeImplements  = "implements"
	edgeTests       = "tests"
	edgeUsesMessage = "uses_message"
	edgeUsesName    = "uses_name"
	edgeUsesEnv     = "uses_env"
	edgeReadsFile   = "reads_file"
	edgeEmbeds      = "embeds"
	edgeWalks       = "walks"
	edgeDispatches  = "dispatches_to"
	edgeReadsVar    = "reads_var"
	edgeWritesVar   = "writes_var"
	edgeInitsVar    = "inits_var"
	edgeUses        = "uses"
	edgeReadsField  = "reads_field"
	edgeWritesField = "writes_field"
	edgeDenotes     = "denotes"
	edgeGuardedBy   = "guarded_by"
	edgeLinksTo     = "links_to"
	edgeBuilds      = "builds"
)

// Güven etiketleri: EXTRACTED kaynaktan ya da tip bilgisinden kesin okunan, INFERRED addan ya da
// değer kullanımından çıkarılan bağdır.
const (
	extracted = "EXTRACTED"
	inferred  = "INFERRED"
)

// Node grafiğin bir düğümüdür. Name sorguda yazılan kısa addır (messages.New, H2100); ID tektir.
type Node struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	File   string   `json:"file,omitempty"`
	Line   int      `json:"line,omitempty"`
	End    int      `json:"end,omitempty"` // son satır (işlev/tip/başlık bölümü): hunk'ları düğüme eşlemek için
	Detail string   `json:"detail,omitempty"`
	Doc    string   `json:"doc,omitempty"` // bildirimin belge yorumunun ilk satırları (go_docs.go)
	Params []string `json:"params,omitempty"`
	// Aliases düğümün başka adlarıdır (ad kodunun dildeki adları: sorguda ad gibi çözülür).
	Aliases []string `json:"aliases,omitempty"`
	// Captured bir func literal'in dış kapsamdan yakaladığı değişkenlerdir ("ad tip").
	Captured []string `json:"captured,omitempty"`
}

// Edge yönlü bir bağdır; File ve Line bağın ilk görüldüğü yerdir, Count aynı bağın kaç kez
// görüldüğü.
type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Confidence string `json:"confidence"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Count      int    `json:"count,omitempty"`
	Sites      []Site `json:"sites,omitempty"` // alan erişim kenarlarında her erişim yeri
}

// Site bir alan erişiminin yeridir: dosya, satır, kip (read, write, read+write, elem-write, addr,
// literal) ve erişimin içinde olduğu en iç func literal'in düğüm kimliği (yoksa "").
type Site struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Mode    string `json:"mode"`
	Closure string `json:"closure,omitempty"`
}

// Graph graph.json'ın içeriğidir. Sıralıdır ve zaman damgası taşımaz: aynı depo aynı dosyayı verir.
type Graph struct {
	SchemaVersion int      `json:"schema_version"`
	Extractor     int      `json:"extractor"`
	Module        string   `json:"module"`
	Stamp         string   `json:"stamp,omitempty"` // kurulduğu andaki kaynak imzası (fresh.go); yoksa denetlenmez
	Nodes         []Node   `json:"nodes"`
	Edges         []Edge   `json:"edges"`
	Warnings      []string `json:"warnings,omitempty"`
}

type edgeKey struct{ from, to, kind string }

// builder grafiği çıkarıcılardan toplar; uç düğümü olmayan kenarlar kurulurken atılır.
type builder struct {
	module   string
	nodes    map[string]*Node
	edges    map[edgeKey]*Edge
	warnings []string
}

func newBuilder(module string) *builder {
	return &builder{module: module, nodes: map[string]*Node{}, edges: map[edgeKey]*Edge{}}
}

// addNode düğümü ekler; aynı kimlik varsa ilki kalır.
func (b *builder) addNode(n Node) {
	if _, ok := b.nodes[n.ID]; !ok {
		b.nodes[n.ID] = &n
	}
}

// addEdge kenarı ekler. Aynı (from, to, kind) tekrar gelirse sayaç artar ve güven EXTRACTED'a
// yükselir (bir yerde kesin okunan bağ, başka yerdeki çıkarımdan güçlüdür).
func (b *builder) addEdge(from, to, kind, conf, file string, line int) {
	k := edgeKey{from, to, kind}
	if e, ok := b.edges[k]; ok {
		e.Count++
		if conf == extracted {
			e.Confidence = extracted
		}
		return
	}
	b.edges[k] = &Edge{From: from, To: to, Kind: kind, Confidence: conf, File: file, Line: line, Count: 1}
}

// maxSites bir kenarda saklanan en çok erişim yeridir.
const maxSites = 500

// addSite alan erişim kenarını (yoksa) ekler ve erişim yerini kenara yazar.
func (b *builder) addSite(from, to, kind string, s Site) {
	b.addEdge(from, to, kind, extracted, s.File, s.Line)
	if e := b.edges[edgeKey{from, to, kind}]; len(e.Sites) < maxSites {
		e.Sites = append(e.Sites, s)
	}
}

func (b *builder) warn(msg string) { b.warnings = append(b.warnings, msg) }

// graph sıralı ve tutarlı Graph'ı kurar: uç düğümü bulunmayan kenarlar atılır (örneğin
// standart kitaplığa giden çağrılar).
func (b *builder) graph() *Graph {
	g := &Graph{SchemaVersion: schemaVersion, Extractor: extractorRevision, Module: b.module, Nodes: []Node{}, Edges: []Edge{}}
	for _, n := range b.nodes {
		g.Nodes = append(g.Nodes, *n)
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	for k, e := range b.edges {
		if b.nodes[k.from] == nil || b.nodes[k.to] == nil {
			continue
		}
		g.Edges = append(g.Edges, *e)
	}
	slices.SortFunc(g.Edges, func(a, c Edge) int {
		return cmp.Or(cmp.Compare(a.From, c.From), cmp.Compare(a.To, c.To), cmp.Compare(a.Kind, c.Kind))
	})
	g.Warnings = append(g.Warnings, b.warnings...)
	sort.Strings(g.Warnings)
	return g
}

// writeGraph grafiği girintili JSON olarak yazar (klasör yoksa açılır).
func writeGraph(path string, g *Graph) error {
	data, err := json.MarshalIndent(g, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return codeindex.WriteFileAtomic(path, append(data, byte(10))) // aynı anda çalışan iki sorgu yarım dosya okumaz
}

// readGraph graph.json'ı okur ve sürümünü denetler.
func readGraph(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g := new(Graph)
	if err := json.Unmarshal(data, g); err != nil {
		return nil, err
	}
	if g.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("graph.json schema_version %d is not supported (this tool reads %d); run build again", g.SchemaVersion, schemaVersion)
	}
	return g, nil
}

package main

import (
	"go/ast"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Bu dosya Go testlerinin depo dosyalarına olan bağını kurar: bir test `.tr` dosyasını çalıştırıyor
// (`reporoot.Path("tools/gpu/gpt_training.tr")`), `lib/*.tr`'yi tarıyor (`reporoot.Path("lib", "*.tr")`) ya da
// bir belgeyi okuyor ise o dosyayı korur. Test işlevinden var olan dosya (ya da belge) düğümüne INFERRED bir
// `tests` kenarı çekilir; `tests-for <dosya>` böylece dosyayı sınayan Go testlerini de verir. Yol, test
// dosyasındaki dize sabitlerinden gelir (değişkenle kurulan yollar görülmez); eşleşme göreli yoldadır ve
// yalnızca "/" içeren yollar sayılır (çıplak "README.md" geçici klasörlerde çok yazılır).

// pathRef test dosyasında geçen, dosya yoluna benzeyen bir dize sabitidir.
type pathRef struct {
	owner string // sabiti içeren işlevin kimliği
	dir   string // test dosyasının klasörü (kökten göreli): "./x" yolları için
	file  string
	line  int
	text  string // yol ya da glob kalıbı
	prod  bool   // sabit test olmayan kodda: sahibinden dosyaya reads_file/embeds kenarı da çekilir
	embed bool   // //go:embed kalıbı: paketin klasörüne göredir, "/" aranmaz
}

// pathCallNames yol kuran çağrılardır (reporoot.Path, filepath.Join, path.Join).
var pathCallNames = map[string]bool{"Path": true, "Join": true, "Glob": true, "GlobFiles": true}

// globList her dize sabiti bağımsız değişkeninin ayrı bir kalıp olduğu çağrılardır.
var globList = map[string]bool{"GlobFiles": true}

// pathCall çağrının bütün argümanları dize sabitiyse onları birleştirip tek yol olarak kaydeder ve argüman
// sabitlerini tek başına yol saymaz.
func (x *goExtractor) pathCall(fc *fileCtx, owner string, call *ast.CallExpr) {
	id := nameIdent(call.Fun)
	if id == nil || !pathCallNames[id.Name] || len(call.Args) == 0 {
		return
	}
	if globList[id.Name] { // testkit.GlobFiles(t, "a/*.tr", "b/*.tr"): her sabit ayrı bir kalıptır
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok {
				if s, ok := stringLit(lit); ok && strings.Contains(s, "/") {
					x.consumed[lit] = true
					x.addPathRef(fc, owner, lit.Pos(), s)
				}
			}
		}
		return
	}
	// Sondaki dize sabiti dizisi yolun kendisidir; öndeki sabit olmayan bağımsız değişkenler (bir kök ya da geçici
	// klasör, `filepath.Join(root, "examples", "tr", "*.tr")`) atılır: kök depo kökü ya da onun kopyasıdır.
	// Tek parçalı ve "/" içermeyen kuyruk ("a.json") yol sayılmaz.
	if _, ok := call.Args[0].(*ast.BasicLit); ok && len(call.Args) > 1 {
		// Kök sabitse ama ortada sabit olmayan parça varsa (reporoot.Path("examples", lang, "*.tr")) o parça "*" olur:
		// kalıp kökten göreli bir yoldur, eşleşmesi var olan dosyalarla sınanır.
		if parts, ok := wildcardParts(call.Args); ok {
			x.addPathRef(fc, owner, call.Pos(), path.Join(parts...))
			for _, arg := range call.Args {
				if lit, isLit := arg.(*ast.BasicLit); isLit {
					x.consumed[lit] = true
				}
			}
			return
		}
	}
	start := len(call.Args)
	for start > 0 {
		lit, ok := call.Args[start-1].(*ast.BasicLit)
		if !ok {
			break
		}
		if _, ok := stringLit(lit); !ok {
			break
		}
		start--
	}
	parts := make([]string, 0, len(call.Args)-start)
	for _, a := range call.Args[start:] {
		lit := a.(*ast.BasicLit)
		s, _ := stringLit(lit)
		parts = append(parts, s)
		x.consumed[lit] = true
	}
	if len(parts) == 0 || (len(parts) == 1 && !strings.Contains(parts[0], "/")) {
		return
	}
	x.addPathRef(fc, owner, call.Pos(), path.Join(parts...))
}

// pathLit tek başına bir dize sabitini, yola benziyorsa kaydeder.
func (x *goExtractor) pathLit(fc *fileCtx, owner string, lit *ast.BasicLit) {
	if x.consumed[lit] {
		return
	}
	if s, ok := stringLit(lit); ok && !strings.ContainsAny(s, "*?[") {
		x.addPathRef(fc, owner, lit.Pos(), s) // tek başına joker kalıbı bir sınama verisi olabilir (FuzzPattern)
	}
}

func stringLit(lit *ast.BasicLit) (string, bool) {
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// addPathRef yola benzeyen metni kaydeder: boşluksuz, kısa ve "/" içeren.
func (x *goExtractor) addPathRef(fc *fileCtx, owner string, pos token.Pos, text string) {
	if len(text) < 3 || len(text) > 200 || !strings.Contains(text, "/") || strings.ContainsAny(text, " \t\n\r{}$%") {
		return
	}
	x.idx.pathRefs = append(x.idx.pathRefs, pathRef{owner: owner, dir: path.Dir(fc.rel), file: fc.rel, line: x.line(pos), text: text, prod: !fc.isTest})
}

// linkTestPaths yol sabitlerini var olan dosya ve belge düğümlerine çözer ve test işlevinden `tests` kenarı çeker.
// Yalnızca TestX/BenchmarkX/FuzzX/ExampleX işlevlerinin sabitleri sayılır. Çözüm, düğümlerin eklendiği
// sıradan bağımsız olsun diye bütün çıkarıcılardan sonra çalışır.
func linkTestPaths(b *builder, idx *goIndex, reach *testReach) {
	isTest, callers, pkgOf := reach.isTest, reach.callers, reach.pkgOf
	byFile := map[string][]*Node{}
	var targets []*Node // glob kalıpları bunlara karşı denenir
	for _, n := range b.nodes {
		if (n.Kind == kindFile || n.Kind == kindDoc) && n.File != "" {
			byFile[n.File] = append(byFile[n.File], n)
			targets = append(targets, n)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	for _, r := range idx.pathRefs {
		tests := reach.testsReaching(r.owner)
		pats := []string{path.Join(r.dir, r.text)}
		if !r.embed {
			pats = append([]string{path.Clean(r.text)}, pats...)
		}
		for _, pat := range pats {
			for _, n := range resolvePattern(pat, byFile, targets) {
				if n.File == r.file {
					continue
				}
				if r.prod && !isTest[r.owner] {
					kind := edgeReadsFile
					if r.embed {
						kind = edgeEmbeds
					}
					b.addEdge(r.owner, n.ID, kind, inferred, r.file, r.line)
				}
				for _, t := range tests {
					b.addEdge(t, n.ID, edgeTests, inferred, r.file, r.line)
				}
				if r.prod && !isTest[r.owner] {
					for _, rg := range walkersOf(r.owner, idx.ranges, callers, pkgOf, b) {
						for _, t := range reach.testsReaching(rg.owner) {
							b.addEdge(t, n.ID, edgeWalks, inferred, rg.file, rg.line)
						}
					}
				}
			}
		}
	}
}

// resolvePattern yolu ya da glob kalıbını var olan dosya/belge düğümlerine çözer.
func resolvePattern(pat string, byFile map[string][]*Node, targets []*Node) []*Node {
	if !strings.ContainsAny(pat, "*?[") {
		return byFile[pat]
	}
	var hit []*Node
	for _, n := range targets {
		if matchGlob(pat, n.File) {
			hit = append(hit, n)
		}
	}
	return hit
}

// Bir yol sabitinden (ya da gömülen dosyadan) onu sınayan teste giden yol, çağıran/kullanan zinciridir. Zincir iki
// ölçüyle sınırlanır: paket sınırını en çok testMaxHops kez geçer (veriyi yükleyen paketin kendi içi serbesttir;
// `codes.Codes()` çağıran bir test bir sınır geçer, onu çağıranın çağıranı iki geçer ve konunun dışına çıkar) ve
// en çok testMaxSteps adımdır. Yalnızca adım sayısıyla sınırlansaydı, bir yükleyiciden her yerde kullanılan
// ortak bir işleve kadar yüzlerce test "korur" görünürdü.
const (
	testMaxHops  = 1
	testMaxSteps = 8
)

// testsReaching owner'ın kendisi test işleviyse onu, değilse onu çağıran (ya da kullanan) testleri sınırlar
// içinde verir; bir testin ötesine geçilmez. pkgOf düğümün paket klasörünü verir.
func testsReaching(owner string, isTest map[string]bool, callers map[string][]string, pkgOf func(string) string) []string {
	if isTest[owner] {
		return []string{owner}
	}
	type state struct {
		id   string
		hops int
	}
	seen := map[string]bool{owner: true}
	frontier := []state{{owner, 0}}
	var out []string
	for steps := 0; steps < testMaxSteps && len(frontier) > 0; steps++ {
		var next []state
		for _, st := range frontier {
			for _, c := range callers[st.id] {
				hops := st.hops
				if pkgOf(c) != pkgOf(st.id) {
					hops++
				}
				if hops > testMaxHops || seen[c] {
					continue
				}
				seen[c] = true
				if isTest[c] {
					out = append(out, c)
				} else {
					next = append(next, state{c, hops})
				}
			}
		}
		frontier = next
	}
	sort.Strings(out)
	return out
}

// embedRefs `//go:embed kalıp...` yönergesi taşıyan değişkenden gömülen dosyalara başvuru kaydeder (kalıplar
// değişkenin paket klasörüne görelidir). Yönerge bildirimin ya da belirtimin doc yorumundadır.
func (x *goExtractor) embedRefs(fc *fileCtx, d *ast.GenDecl) {
	if d.Tok != token.VAR {
		return
	}
	add := func(doc *ast.CommentGroup, names []*ast.Ident) {
		if doc == nil || len(names) == 0 {
			return
		}
		obj := fc.u.Info.Defs[names[0]]
		if obj == nil || obj.Pkg() == nil {
			return
		}
		owner := valueID(kindVar, obj.Pkg().Path(), names[0].Name)
		for _, c := range doc.List {
			rest, ok := strings.CutPrefix(c.Text, "//go:embed ")
			if !ok {
				continue
			}
			for _, pat := range strings.Fields(rest) {
				pat = strings.Trim(pat, "\"`")
				x.idx.pathRefs = append(x.idx.pathRefs, pathRef{owner: owner, dir: path.Dir(fc.rel), file: fc.rel,
					line: x.line(c.Pos()), text: pat, prod: !fc.isTest, embed: true})
			}
		}
	}
	for _, spec := range d.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok {
			doc := vs.Doc
			if doc == nil && len(d.Specs) == 1 {
				doc = d.Doc
			}
			add(doc, vs.Names)
		}
	}
}

// wildcardParts bağımsız değişkenleri yol parçalarına çevirir: sabitler kendisi, sabit olmayanlar "*". En az bir
// sabit olmayan ve en az iki sabit parça varsa ok (hepsi sabitse ya da tek sabit varsa çağıran başka kol işler).
func wildcardParts(args []ast.Expr) ([]string, bool) {
	var parts []string
	lits, dyn := 0, 0
	for _, a := range args {
		if lit, ok := a.(*ast.BasicLit); ok {
			if s, ok := stringLit(lit); ok {
				parts = append(parts, s)
				lits++
				continue
			}
		}
		parts = append(parts, "*")
		dyn++
	}
	return parts, lits >= 2 && dyn > 0
}

// testReach bir işlevden onu çağıran testlere giden zincirin ortak durumudur (yol sabitleri ve TR metinleri aynı kuralı
// kullanır).
type testReach struct {
	isTest  map[string]bool
	callers map[string][]string // işlev ya da değişken -> onu çağıran ya da kullananlar
	pkgOf   func(string) string
}

func newTestReach(b *builder, idx *goIndex) *testReach {
	r := &testReach{isTest: map[string]bool{}, callers: map[string][]string{}}
	for _, t := range idx.tests {
		r.isTest[t.id] = true
	}
	for k := range b.edges {
		if k.kind == edgeCalls || k.kind == edgeUses || k.kind == edgeReadsVar || k.kind == edgeWritesVar {
			r.callers[k.to] = append(r.callers[k.to], k.from)
		}
	}
	for _, list := range r.callers {
		sort.Strings(list)
	}
	r.pkgOf = func(id string) string {
		if n := b.nodes[id]; n != nil {
			return path.Dir(n.File)
		}
		return ""
	}
	return r
}

func (r *testReach) testsReaching(owner string) []string {
	return testsReaching(owner, r.isTest, r.callers, r.pkgOf)
}

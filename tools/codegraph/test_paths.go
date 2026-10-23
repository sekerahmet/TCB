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
}

// pathCallNames yol kuran çağrılardır (reporoot.Path, filepath.Join, path.Join).
var pathCallNames = map[string]bool{"Path": true, "Join": true, "Glob": true}

// pathCall çağrının bütün argümanları dize sabitiyse onları birleştirip tek yol olarak kaydeder ve argüman
// sabitlerini tek başına yol saymaz.
func (x *goExtractor) pathCall(fc *fileCtx, owner string, call *ast.CallExpr) {
	id := nameIdent(call.Fun)
	if !fc.isTest || id == nil || !pathCallNames[id.Name] || len(call.Args) == 0 {
		return
	}
	parts := make([]string, 0, len(call.Args))
	for _, a := range call.Args {
		lit, ok := a.(*ast.BasicLit)
		if !ok {
			return
		}
		s, ok := stringLit(lit)
		if !ok {
			return
		}
		parts = append(parts, s)
	}
	for _, a := range call.Args {
		x.consumed[a.(*ast.BasicLit)] = true
	}
	x.addPathRef(fc, owner, call.Pos(), path.Join(parts...))
}

// pathLit tek başına bir dize sabitini, yola benziyorsa kaydeder.
func (x *goExtractor) pathLit(fc *fileCtx, owner string, lit *ast.BasicLit) {
	if !fc.isTest || x.consumed[lit] {
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
	x.idx.pathRefs = append(x.idx.pathRefs, pathRef{owner: owner, dir: path.Dir(fc.rel), file: fc.rel, line: x.line(pos), text: text})
}

// linkTestPaths yol sabitlerini var olan dosya ve belge düğümlerine çözer ve test işlevinden `tests` kenarı çeker.
// Yalnızca TestX/BenchmarkX/FuzzX/ExampleX işlevlerinin sabitleri sayılır. Çözüm, düğümlerin eklendiği
// sıradan bağımsız olsun diye bütün çıkarıcılardan sonra çalışır.
func linkTestPaths(b *builder, idx *goIndex) {
	isTest := map[string]bool{}
	for _, t := range idx.tests {
		isTest[t.id] = true
	}
	byFile := map[string][]*Node{}
	var targets []*Node // glob kalıpları bunlara karşı denenir
	for _, n := range b.nodes {
		if (n.Kind == kindFile || n.Kind == kindDoc) && n.File != "" {
			byFile[n.File] = append(byFile[n.File], n)
			targets = append(targets, n)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	callers := map[string][]string{} // işlev ya da değişken -> onu çağıran ya da kullananlar
	for k := range b.edges {
		if k.kind == edgeCalls || k.kind == edgeUses {
			callers[k.to] = append(callers[k.to], k.from)
		}
	}
	for _, list := range callers {
		sort.Strings(list)
	}
	for _, r := range idx.pathRefs {
		tests := testsReaching(r.owner, isTest, callers)
		if len(tests) == 0 {
			continue
		}
		for _, pat := range []string{path.Clean(r.text), path.Join(r.dir, r.text)} {
			var hit []*Node
			if !strings.ContainsAny(pat, "*?[") {
				hit = byFile[pat]
			} else {
				for _, n := range targets {
					if matchGlob(pat, n.File) {
						hit = append(hit, n)
					}
				}
			}
			for _, n := range hit {
				for _, t := range tests {
					if n.File != r.file {
						b.addEdge(t, n.ID, edgeTests, inferred, r.file, r.line)
					}
				}
			}
		}
	}
}

// testPathDepth yol sabitini taşıyan yardımcıdan testine kadar izlenen en çok çağrı adımıdır.
const testPathDepth = 3

// testsReaching owner'ın kendisi test işleviyse onu, değilse onu en çok testPathDepth adımda çağıran (ya da kullanan)
// testleri verir; bir testin ötesine geçilmez.
func testsReaching(owner string, isTest map[string]bool, callers map[string][]string) []string {
	if isTest[owner] {
		return []string{owner}
	}
	seen := map[string]bool{owner: true}
	frontier := []string{owner}
	var out []string
	for depth := 0; depth < testPathDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, id := range frontier {
			for _, c := range callers[id] {
				if seen[c] {
					continue
				}
				seen[c] = true
				if isTest[c] {
					out = append(out, c)
				} else {
					next = append(next, c)
				}
			}
		}
		frontier = next
	}
	sort.Strings(out)
	return out
}

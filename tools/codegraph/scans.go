package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Bu dosya depo genelini tarayan testleri bağlar. Bir kural testi dosyaları tek tek adlandırmaz, bir klasörü gezer ve
// uzantıyla süzer (`reporoot.WalkDir(filepath.Join(root, top), ...)` + `!strings.HasSuffix(path, ".go") ||
// strings.HasSuffix(path, "_test.go")`, ya da `git ls-files` çıktısı). Böyle bir test kalıbın kapsadığı her dosyanın
// testidir: internal.TestNoTurkishLiterals her test dışı .go dosyasını denetler. Bağ olmadan `tests-for <dosya>` yalnızca
// paketin testlerini veriyordu; tam denetim bir dosyada bu testle kaldığında cevap onu göstermemişti (TCB 2026-10-25).
//
// Her tarama yeri bir `scan` düğümüdür: tabanları (depo köküne göre klasörler), dahil ve hariç uzantıları. Taban yalnızca
// depo kökünden çözülebiliyorsa kabul edilir: reporoot.Root(), reporoot.Path(...), Join(kök, parça...), parça bir dize,
// sabit listede dönen değişken ya da paket düzeyi liste, kökü tutan yerel değişken ya da (bir adım) çağıranların verdiği
// parametre. Geçici kopya (t.TempDir()) ya da veriden okunan klasör çözülmez ve tarama sayılmaz. Süzgeç atlama
// koşullarından okunur: `return`/`continue` ile biten bir `if`in koşulu dosyayı dışarıda bırakır (koşuldaki olumsuz
// uzantı dahil, olumlu uzantı hariçtir), başka bir `if` ya da `switch filepath.Ext(...)` dahil eder; klasör atlama
// (testdata, negative) ve başka koşullar okunmaz, kalıp bu yüzden bir üst kümedir. Testten tarama düğümüne `scans`
// kenarı çekilir (zincir kuralı); tests-for kalıba uyan dosyada bunu kalıp başına tek satır yazar (query_scans.go).

// scanSite bir test dosyasında depo dosyalarını listeleyen çağrıdır.
type scanSite struct {
	owner    string
	fc       *fileCtx
	fn       *ast.FuncDecl
	base     ast.Expr // nil: `git ls-files` -C'siz, testin klasörü
	dirs     ast.Expr // ListFiles'ın alt klasör listesi (nil ya da []string)
	filters  ast.Node // WalkDir/ListFiles geri çağırması; nil ise işlevin gövdesi ve (liste dönen yardımcıda) çağıranlarınki
	line     int
	fromPkg  bool // taban testin klasörüdür
	listOnly bool // çağrı listeyi döndürür (git ls-files, GitTracked): süzgeç çağıranda da olabilir
}

// assignRef bir değişkene ya da yapı alanına yapılan atamadır (kök çözümü için).
type assignRef struct {
	fc  *fileCtx
	fn  *ast.FuncDecl
	rhs ast.Expr
	idx int // çoklu dönüşte sol taraf indisi (a, b := f() -> 0, 1); tekli atamada -1
}

// scanScope bir taramanın kapsamıdır.
type scanScope struct {
	bases    []string // depo köküne göre klasörler ("." kök)
	inc, exc []string // dosya adı ekleri (".go", "_test.go")
}

// scanFunc bir test dosyası işlevinin bildirimi ve bağlamıdır (parametre ve çağıran çözümü için).
type scanFunc struct {
	fc *fileCtx
	fn *ast.FuncDecl
	id string
}

// scanState tarama çözümünün durumudur: kayıtlar dosyalar gezilirken toplanır, çözüm bütün modül okununca yapılır.
type scanState struct {
	sites    []scanSite
	funcs    map[types.Object]scanFunc    // test dosyalarındaki işlevler
	allFuncs map[types.Object]scanFunc    // modüldeki bütün işlevler (yardımcı kök çözümü için)
	callers  map[string][]scanFunc        // işlev kimliği -> onu doğrudan çağıran test işlevleri
	assigns  map[types.Object][]assignRef // paket değişkeni / yapı alanı atamaları
	lists    map[types.Object][]string    // paket düzeyi dize listeleri (`var layoutRoots = []string{...}`)
	ranges   map[types.Object]rangeOver   // döngü değişkeni -> üzerinde döndüğü ifade
	done     []resolvedScan               // çözülen taramalar (linkScans düğüme çevirir)
}

// rangeOver bir döngünün üzerinde döndüğü ifade ve dosyası.
type rangeOver struct {
	fc *fileCtx
	x  ast.Expr
}

// resolvedScan bir taramanın çözülmüş hali ve testlere ulaştığı işlev.
type resolvedScan struct {
	id, from string // düğüm kimliği; testlerine bakılan işlev
	file     string
	line     int
	scope    scanScope
}

func newScanState() *scanState {
	return &scanState{
		funcs:    map[types.Object]scanFunc{},
		allFuncs: map[types.Object]scanFunc{},
		assigns:  map[types.Object][]assignRef{},
		lists:    map[types.Object][]string{},
		ranges:   map[types.Object]rangeOver{},
	}
}

// collectScanDefs modüldeki bütün işlevleri ve paket/yapı alanı atamalarını kaydeder (`repoState.root = resolveRoot("")` gibi).
func (x *goExtractor) collectScanDefs(fc *fileCtx, f *ast.File) {
	st := x.idx.scans
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok || d.Body == nil {
			continue
		}
		obj := fc.u.Info.Defs[d.Name]
		if obj == nil {
			continue
		}
		st.allFuncs[obj] = scanFunc{fc, d, x.objectID(obj)}
		ast.Inspect(d.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, l := range as.Lhs {
				var target types.Object
				switch lhs := l.(type) {
				case *ast.SelectorExpr:
					target = fc.u.Info.Uses[lhs.Sel]
				case *ast.Ident:
					if u := fc.u.Info.Uses[lhs]; u != nil && u.Parent() == fc.u.Pkg.Scope() {
						target = u
					}
				}
				if target == nil {
					continue
				}
				switch {
				case len(as.Lhs) == len(as.Rhs):
					st.assigns[target] = append(st.assigns[target], assignRef{fc, d, as.Rhs[i], -1})
				case len(as.Rhs) == 1 && len(as.Lhs) > 1:
					st.assigns[target] = append(st.assigns[target], assignRef{fc, d, as.Rhs[0], i})
				}
			}
			return true
		})
	}
}

// collectScans bir test dosyasının işlevlerini, paket düzeyi dize listelerini, döngü değişkenlerini ve listeleme
// çağrılarını kaydeder.
func (x *goExtractor) collectScans(fc *fileCtx, f *ast.File) {
	st := x.idx.scans
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
					if vals, ok := stringList(vs.Values[0]); ok {
						st.lists[fc.u.Info.Defs[vs.Names[0]]] = vals
					}
				}
			}
		case *ast.FuncDecl:
			obj := fc.u.Info.Defs[d.Name]
			if obj == nil || d.Body == nil {
				continue
			}
			id := x.objectID(obj)
			st.funcs[obj] = scanFunc{fc, d, id}
			ast.Inspect(d.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.RangeStmt:
					if v, ok := n.Value.(*ast.Ident); ok && fc.u.Info.Defs[v] != nil {
						st.ranges[fc.u.Info.Defs[v]] = rangeOver{fc, n.X}
					}
				case *ast.CallExpr:
					if site, ok := x.scanCall(fc, d, id, n); ok {
						st.sites = append(st.sites, site)
					}
				}
				return true
			})
		}
	}
}

// scanCall depo dosyalarını listeleyen çağrıyı tanır: reporoot.WalkDir, codeindex.ListFiles, codeindex.GitTracked ve `git ... ls-files`.
func (x *goExtractor) scanCall(fc *fileCtx, fn *ast.FuncDecl, owner string, call *ast.CallExpr) (scanSite, bool) {
	site := scanSite{owner: owner, fc: fc, fn: fn, line: x.line(call.Pos())}
	if f, ok := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func); ok && f.Pkg() != nil {
		pkg := f.Pkg().Path()
		switch {
		case strings.HasSuffix(pkg, "internal/reporoot") && f.Name() == "WalkDir" && len(call.Args) == 2:
			site.base = call.Args[0]
			if lit, ok := call.Args[1].(*ast.FuncLit); ok {
				site.filters = lit.Body
			}
			return site, true
		case strings.HasSuffix(pkg, "internal/devtools/codeindex") && f.Name() == "ListFiles" && len(call.Args) == 5:
			site.base, site.dirs = call.Args[0], call.Args[1]
			if lit, ok := call.Args[4].(*ast.FuncLit); ok {
				site.filters = lit.Body
			} else if id, ok := call.Args[4].(*ast.Ident); ok && id.Name == "nil" {
				site.listOnly = true
			}
			return site, true
		case strings.HasSuffix(pkg, "internal/devtools/codeindex") && f.Name() == "GitTracked" && len(call.Args) == 1:
			site.base, site.listOnly = call.Args[0], true
			return site, true
		}
	}
	var words []string
	for _, a := range call.Args {
		if lit, ok := a.(*ast.BasicLit); ok {
			s, _ := strconv.Unquote(lit.Value)
			words = append(words, s)
		} else {
			words = append(words, "")
		}
	}
	if len(words) < 2 || words[0] != "git" || !containsString(words, "ls-files") {
		return site, false
	}
	site.listOnly = true
	site.fromPkg = true
	for i, w := range words[:len(words)-1] {
		if w == "-C" {
			site.base, site.fromPkg = call.Args[i+1], false
		}
	}
	return site, true
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// stringList bir dize sabiti listesinin değerleridir.
func stringList(e ast.Expr) ([]string, bool) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	var vals []string
	for _, el := range lit.Elts {
		b, ok := el.(*ast.BasicLit)
		if !ok || b.Kind != token.STRING {
			return nil, false
		}
		s, _ := strconv.Unquote(b.Value)
		vals = append(vals, s)
	}
	return vals, true
}

// evalSubdirs ListFiles'ın ikinci bağımsız değişkenini (nil ya da alt klasör listesi) çözer.
func (x *goExtractor) evalSubdirs(fc *fileCtx, e ast.Expr) ([]string, bool) {
	if id, ok := e.(*ast.Ident); ok {
		if id.Name == "nil" {
			return nil, true
		}
		vals, ok := x.idx.scans.lists[fc.u.Info.Uses[id]]
		return vals, ok
	}
	return stringList(e)
}

// resolveScans kaydedilen taramaların tabanını ve süzgecini çözer (bütün modül okunduktan sonra).
func (x *goExtractor) resolveScans() {
	st := x.idx.scans
	x.buildScanCallers()
	for _, s := range st.sites {
		var bases []string
		if s.fromPkg {
			bases = []string{path.Dir(s.fc.rel)}
		} else {
			vals, rooted := x.evalPath(s.fc, s.fn, s.base, 6)
			if !rooted || len(vals) == 0 {
				continue // taban depo kökünden çözülmüyor: geçici kopya ya da veriden okunan klasör
			}
			bases = vals
		}
		if s.dirs != nil {
			sub, ok := x.evalSubdirs(s.fc, s.dirs)
			if !ok {
				continue
			}
			if len(sub) > 0 {
				bases = joinAll(bases, [][]string{sub})
			}
		}
		filters := s.filters
		if filters == nil {
			filters = s.fn.Body
		}
		inc, exc, dirs := scanFilters(filters, x.suffixEval(s.fc, s.fn, nil))
		if dirs {
			continue // yalnızca klasörleri gezer (paket başına build.ImportDir): dosya kalıbı yok
		}
		id := "scan:" + s.fc.rel + ":" + strconv.Itoa(s.line)
		callers := x.directCallers(s.owner)
		if isTestName(s.fn.Name.Name) || len(callers) == 0 || !s.listOnly && !usesStringParam(filters, s.fc, s.fn) {
			st.done = append(st.done, resolvedScan{id, s.owner, s.fc.rel, s.line, newScope(bases, inc, exc)})
			continue
		}
		// Süzgeci çağırana bağlı yardımcı: liste döndürür (`git ls-files` çıktısı, süzgeç çağıranda) ya da eki parametreyle
		// alır (`repoFiles(t, ".tr")`). Her çağıranın kendi kapsamı vardır.
		for _, c := range callers {
			bind := x.bindParams(c, s.fc, s.fn)
			ci, ce, _ := scanFilters(filters, x.suffixEval(s.fc, s.fn, bind))
			if s.listOnly {
				li, le, _ := scanFilters(c.fn.Body, x.suffixEval(c.fc, c.fn, nil))
				ci, ce = append(ci, li...), append(ce, le...)
			}
			st.done = append(st.done, resolvedScan{id + ":" + c.id, c.id, s.fc.rel, s.line, newScope(bases, ci, ce)})
		}
	}
}

// bindParams çağıranın fn'ye verdiği dize bağımsız değişkenlerini fn'nin parametrelerine bağlar (birden çok çağrıda
// değerler birleşir).
func (x *goExtractor) bindParams(c scanFunc, fc *fileCtx, fn *ast.FuncDecl) map[types.Object][]string {
	obj := fc.u.Info.Defs[fn.Name]
	var params []types.Object
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			params = append(params, fc.u.Info.Defs[name])
		}
	}
	bind := map[types.Object][]string{}
	ev := x.suffixEval(c.fc, c.fn, nil)
	ast.Inspect(c.fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && c.fc.u.Info.Uses[nameIdent(call.Fun)] == obj {
			for i, a := range call.Args {
				if i < len(params) && params[i] != nil {
					bind[params[i]] = append(bind[params[i]], ev(a)...)
				}
			}
		}
		return true
	})
	return bind
}

// usesStringParam süzgeç fn'nin bir dize parametresini kullanıyor mu (eki çağıran verir).
func usesStringParam(filters ast.Node, fc *fileCtx, fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(filters, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && !found {
			if obj := fc.u.Info.Uses[id]; obj != nil && paramIndex(fc, fn, obj) >= 0 {
				if b, ok := obj.Type().(*types.Basic); ok && b.Info()&types.IsString != 0 {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func newScope(bases, inc, exc []string) scanScope {
	sortUnique := func(list []string) []string {
		sort.Strings(list)
		return compactStrings(list)
	}
	return scanScope{sortUnique(bases), sortUnique(inc), sortUnique(exc)}
}

func compactStrings(list []string) []string {
	var out []string
	for i, s := range list {
		if i == 0 || s != list[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// buildScanCallers test dosyası işlevlerinin doğrudan çağıranlarını bir kez indeksler.
func (x *goExtractor) buildScanCallers() {
	st := x.idx.scans
	st.callers = map[string][]scanFunc{}
	for _, f := range st.funcs {
		seen := map[string]bool{}
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if obj := f.fc.u.Info.Uses[nameIdent(call.Fun)]; obj != nil {
					if id := x.objectID(obj); id != "" && !seen[id] {
						seen[id] = true
						st.callers[id] = append(st.callers[id], f)
					}
				}
			}
			return true
		})
	}
	for id, list := range st.callers {
		sort.Slice(list, func(i, j int) bool { return list[i].id < list[j].id })
		st.callers[id] = list
	}
}

// directCallers bir test dosyası işlevini doğrudan çağıran test dosyası işlevleridir.
func (x *goExtractor) directCallers(id string) []scanFunc {
	return x.idx.scans.callers[id]
}

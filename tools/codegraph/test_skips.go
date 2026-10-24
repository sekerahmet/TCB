package main

import (
	"bytes"
	"go/ast"
	"go/build/constraint"
	"go/printer"
	"go/token"
	"go/types"
	"path"
	"slices"
	"strings"
)

// Bu dosya yalnızca belli bir ortamda koşan testleri işaretler (deneme 9 H2, bağımsız denetim grup 6 H2). Testin
// gövdesi deyim deyim yürünür (denetim akışı; iç içe işlev değişmezlerine, `t.Run` alt testlerine girilmez) ve
// `t.Skip` çağrısına varan her yolun koşulu testin `Skip` alanına kaynak metniyle yazılır: `g.lt == nil`,
// `testing.Short()`, `runtime.GOOS != "linux"`. Yolun koşulu, girilen `if` dallarının koşulları ile ondan önce sessizce
// dönülen yerlerin koşullarıdır (`no earlier return (C)`: `if err := os.Symlink(...); err == nil { return }` sonrası
// atlama ancak bağlantı kurulamazsa olur); `t.Fatal` ile biten dal koşula girmez (o bir hatadır, ortam değil).
// Başında koşulsuz atlayan test "always" alır. Bir yardımcının (aynı paketteki test dosyası işlevi) etiketi onu
// çağıran teste o noktanın koşuluyla geçer, yardımcının yardımcısı da. Dosyanın build kısıtı (`//go:build unix`,
// `_amd64_test.go`) da yazılır: `not unix (build constraint)`. Koşul değerlendirilmez; okuyan "CI bunu koşuyor mu"
// sorusunu kendisi cevaplar. tests-for her testin yanına yazar ve listelenen testlerin hepsi ortama bağlıysa bunu tek
// satırla söyler (CI'ın bir kipinde zaten koşan `testing.Short()` ve `raceEnabled` ortam sayılmaz).

// skipDecl bir test dosyası işlevidir.
type skipDecl struct {
	id   string
	decl *ast.FuncDecl
	info *types.Info
	file string // köke göre yol
	cons string // dosyanın build kısıtı ("" yoksa)
}

// testSkips modüldeki test dosyalarının işlevlerini tarar ve testlerin atlanma koşullarını düğümlerine yazar.
func (x *goExtractor) testSkips() {
	var decls []skipDecl
	for _, u := range x.m.Units {
		for _, f := range u.Files {
			rel := x.m.Rel(x.m.Fset.Position(f.Pos()).Filename)
			if !isTestFile(rel) {
				continue
			}
			cons := fileConstraint(f, rel)
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if fn, _ := u.Info.Defs[fd.Name].(*types.Func); fn != nil {
					decls = append(decls, skipDecl{funcRef(fn), fd, u.Info, rel, cons})
				}
			}
		}
	}
	// Yardımcıların etiketleri birbirine dayanır: değişmeyene dek (en çok birkaç tur) yeniden yürünür.
	labels := map[string]string{}
	for round := 0; round < 4; round++ {
		changed := false
		for _, d := range decls {
			if l := skipLabel(d, labels, x.source); l != labels[d.id] {
				labels[d.id], changed = l, true
			}
		}
		if !changed {
			break
		}
	}
	for _, d := range decls {
		if d.decl.Recv != nil || !isTestName(d.decl.Name.Name) {
			continue
		}
		label := labels[d.id]
		if d.cons != "" {
			label = joinOr(notBuilt(d.cons), label)
		}
		if n := x.b.nodes[d.id]; n != nil && label != "" {
			n.Skip = label
		}
	}
}

// skipLabel işlevin atlama koşuludur: atlamaya varan yolların koşulları `||` ile ("" atlamıyorsa).
func skipLabel(d skipDecl, helpers map[string]string, source func(ast.Expr) string) string {
	w := &skipWalk{info: d.info, helpers: helpers, source: source}
	w.block(d.decl.Body.List, nil)
	return pathsLabel(w.found)
}

// pathsLabel atlama yollarının etiketidir: koşulsuz yol varsa "always"; bütün yolların ortak baş koşulları bir kez
// yazılır (`X && (A || B)`).
func pathsLabel(paths [][]string) string {
	if len(paths) == 0 {
		return ""
	}
	k := 0
	for ; len(paths) > 1 && k < len(paths[0]); k++ {
		if slices.ContainsFunc(paths[1:], func(p []string) bool { return k >= len(p) || p[k] != paths[0][k] }) {
			break
		}
	}
	var rest []string
	for _, p := range paths {
		if len(p) == 0 {
			return "always"
		}
		if len(p) == k {
			return joinAnd(p...) // bir yol yalnızca ortak koşullardan oluşur: öbürleri onu genişletmez
		}
		if r := joinAnd(p[k:]...); !slices.Contains(rest, r) {
			rest = append(rest, r)
		}
	}
	if k == 0 {
		return strings.Join(rest, " || ")
	}
	return joinAnd(append(slices.Clone(paths[0][:k]), strings.Join(rest, " || "))...)
}

// skipWalk bir gövdenin yürüyüşüdür.
type skipWalk struct {
	info    *types.Info
	helpers map[string]string
	source  func(ast.Expr) string
	found   [][]string // atlamaya varan yolların koşulları
}

// record bir atlama yolunu yazar, aynı yol bir kez.
func (w *skipWalk) record(path []string, extra string) {
	path = slices.Clone(path)
	if extra != "" {
		path = append(path, extra)
	}
	if !slices.ContainsFunc(w.found, func(p []string) bool { return slices.Equal(p, path) }) {
		w.found = append(w.found, path)
	}
}

// block bir deyim listesini sırayla yürür; abs listeye varmanın koşullarıdır. ends liste her yolda bitiyorsa (dönüş,
// atlama, düşme) true'dur; rets listenin içinde sessizce dönülen yerlerin listeye göre koşullarıdır ("" koşulsuz).
func (w *skipWalk) block(list []ast.Stmt, abs []string) (ends bool, rets []string) {
	var local []string // listenin içinde eklenen koşullar (önceki dönüşlerin olmaması)
	for _, s := range list {
		cur := append(slices.Clone(abs), local...)
		e, r := w.stmt(s, cur)
		for _, c := range r {
			rets = append(rets, joinAnd(append(slices.Clone(local), c)...))
			if c != "" {
				local = append(local, "no earlier return "+paren(c))
			}
		}
		if e {
			return true, rets
		}
	}
	return false, rets
}

// stmt bir deyimi yürür (block'un sözleşmesiyle; rets deyime göre).
func (w *skipWalk) stmt(s ast.Stmt, abs []string) (ends bool, rets []string) {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		if w.calls(s, abs) {
			return true, nil
		}
		return true, []string{""}
	case *ast.ExprStmt, *ast.AssignStmt, *ast.DeclStmt, *ast.IncDecStmt, *ast.SendStmt:
		return w.calls(s, abs), nil
	case *ast.BlockStmt:
		return w.block(s.List, abs)
	case *ast.LabeledStmt:
		return w.stmt(s.Stmt, abs)
	case *ast.IfStmt:
		if s.Init != nil && w.calls(s.Init, abs) || w.calls(s.Cond, abs) {
			return true, nil
		}
		cond, neg := w.condition(s)
		thenEnds, thenRets := w.block(s.Body.List, append(slices.Clone(abs), cond))
		for _, r := range thenRets {
			rets = append(rets, joinAnd(cond, r))
		}
		if s.Else == nil {
			return false, rets
		}
		elseEnds, elseRets := w.stmt(s.Else, append(slices.Clone(abs), neg))
		for _, r := range elseRets {
			rets = append(rets, joinAnd(neg, r))
		}
		return thenEnds && elseEnds, rets
	case *ast.ForStmt:
		_, rets = w.block(s.Body.List, abs)
	case *ast.RangeStmt:
		if w.calls(s.X, abs) {
			return true, nil
		}
		_, rets = w.block(s.Body.List, abs)
	case *ast.SwitchStmt:
		for _, c := range s.Body.List {
			cc := c.(*ast.CaseClause)
			_, r := w.block(cc.Body, append(slices.Clone(abs), w.caseCondition(s.Tag, cc)))
			rets = append(rets, r...)
		}
	}
	return false, rets
}

// calls bir deyimin ya da ifadenin içindeki çağrıları yürür (iç içe işlev değişmezlerine girmez): `x := f(helper(t))`
// içindeki yardımcı da sayılır. Atlama ya da düşme çağrısı deyimi bitirir.
func (w *skipWalk) calls(n ast.Node, abs []string) (ends bool) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if w.call(n, abs) {
				ends = true
			}
		}
		return !ends
	})
	return ends
}

// call bir çağrıyı yürür: atlama yolu yazar, düşme ya da atlama deyimi bitirir; yardımcının etiketi o noktanın
// koşuluyla yazılır.
func (w *skipWalk) call(c *ast.CallExpr, abs []string) (ends bool) {
	switch {
	case w.testingCall(c, "Skip", "Skipf", "SkipNow"):
		w.record(abs, "")
		return true
	case w.testingCall(c, "Fatal", "Fatalf", "FailNow") || w.funcCall(c, "os", "Exit") || w.builtin(c, "panic"):
		return true
	}
	if id := nameIdent(c.Fun); id != nil {
		if fn, ok := w.info.Uses[id].(*types.Func); ok {
			switch l := w.helpers[funcRef(fn)]; l {
			case "":
			case "always":
				w.record(abs, "")
				return true // her zaman atlayan yardımcıdan sonrası koşmaz
			default:
				w.record(abs, l)
			}
		}
	}
	return false
}

// testingCall c, testing paketinin T, B, F ya da TB türünde bir alıcıyla names'ten bir metodun çağrısı mı.
func (w *skipWalk) testingCall(c *ast.CallExpr, names ...string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || !slices.Contains(names, sel.Sel.Name) {
		return false
	}
	t := w.info.TypeOf(sel.X)
	return t != nil && strings.HasPrefix(strings.TrimPrefix(t.String(), "*"), "testing.")
}

// funcCall c, pkg paketinin name işlevinin çağrısı mı.
func (w *skipWalk) funcCall(c *ast.CallExpr, pkg, name string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	fn, ok := w.info.Uses[sel.Sel].(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == pkg
}

// builtin c, Go'nun name yerleşiğinin çağrısı mı.
func (w *skipWalk) builtin(c *ast.CallExpr, name string) bool {
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != name {
		return false
	}
	_, ok = w.info.Uses[id].(*types.Builtin)
	return ok
}

// condition if deyiminin koşulu ve değili, kaynak metniyle; ön deyim varsa `(ön; koşul)` biçiminde.
func (w *skipWalk) condition(s *ast.IfStmt) (cond, neg string) {
	cond, neg = w.source(s.Cond), w.negation(s.Cond)
	if s.Init != nil {
		var b bytes.Buffer
		_ = printer.Fprint(&b, token.NewFileSet(), s.Init)
		init := strings.Join(strings.Fields(b.String()), " ")
		cond, neg = "("+init+"; "+cond+")", "("+init+"; "+neg+")"
	}
	return cond, neg
}

// negation ifadenin değilidir: karşılaştırma ters çevrilir, `!x` x olur, ad ve çağrı `!f()`, öbürü `!(x)`.
func (w *skipWalk) negation(e ast.Expr) string {
	flip := map[token.Token]token.Token{token.EQL: token.NEQ, token.NEQ: token.EQL, token.LSS: token.GEQ, token.GEQ: token.LSS, token.GTR: token.LEQ, token.LEQ: token.GTR}
	switch e := ast.Unparen(e).(type) {
	case *ast.BinaryExpr:
		if op, ok := flip[e.Op]; ok {
			return w.source(e.X) + " " + op.String() + " " + w.source(e.Y)
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return w.source(e.X)
		}
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr:
		return "!" + w.source(e)
	}
	return "!(" + w.source(e) + ")"
}

// caseCondition bir switch kolunun koşuludur (`tag == a || tag == b`; varsayılan kol "default").
func (w *skipWalk) caseCondition(tag ast.Expr, cc *ast.CaseClause) string {
	if cc.List == nil {
		return "default"
	}
	var parts []string
	for _, e := range cc.List {
		if tag != nil {
			parts = append(parts, w.source(tag)+" == "+w.source(e))
		} else {
			parts = append(parts, w.source(e))
		}
	}
	return wrapOr(strings.Join(parts, " || "))
}

// joinAnd boş olmayan koşulları `&&` ile birleştirir (tek koşul olduğu gibi kalır).
func joinAnd(conds ...string) string {
	var parts []string
	for _, c := range conds {
		if c != "" {
			parts = append(parts, c)
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	for i, c := range parts {
		parts[i] = wrapOr(c)
	}
	return strings.Join(parts, " && ")
}

// joinOr iki etiketi `||` ile birleştirir (boş olan atlanır).
func joinOr(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " || " + b
}

// wrapOr parantez dışında `||` içeren bir koşulu `&&` ile birleşmeden önce paranteze alır.
func wrapOr(c string) string {
	depth := 0
	for i, r := range c {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 && strings.HasPrefix(c[i:], "|| ") {
				return "(" + c + ")"
			}
		}
	}
	return c
}

// paren koşulu paranteze alır; zaten baştan sona bir parantez içindeyse olduğu gibi bırakır.
func paren(c string) string {
	depth := 0
	for i, r := range c {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i < len(c)-1 {
				return "(" + c + ")"
			}
		}
		if depth == 0 && i == 0 {
			return "(" + c + ")"
		}
	}
	if depth != 0 || c == "" {
		return "(" + c + ")"
	}
	return c
}

// notBuilt build kısıtının değilidir, etiket metni olarak.
func notBuilt(cons string) string {
	if strings.ContainsAny(cons, " &|!") {
		cons = "(" + cons + ")"
	}
	return "not " + cons + " (build constraint)"
}

// fileConstraint dosyanın build kısıtıdır: `//go:build` satırı ve adındaki GOOS/GOARCH eki (`_unix` bir ek değildir,
// `go/build` kuralı); ikisi varsa `&&` ile.
func fileConstraint(f *ast.File, rel string) string {
	var parts []string
	for _, cg := range f.Comments {
		if cg.Pos() >= f.Package {
			break
		}
		for _, c := range cg.List {
			if constraint.IsGoBuild(c.Text) {
				if e, err := constraint.Parse(c.Text); err == nil {
					parts = append(parts, e.String())
				}
			}
		}
	}
	name := strings.TrimSuffix(strings.TrimSuffix(path.Base(rel), ".go"), "_test")
	words := strings.Split(name, "_")
	for i := max(1, len(words)-2); i < len(words); i++ {
		if knownOS[words[i]] || knownArch[words[i]] {
			parts = append(parts, words[i])
		}
	}
	return strings.Join(parts, " && ")
}

// knownOS ve knownArch dosya adı eki olabilen GOOS ve GOARCH değerleridir (`go tool dist list`, Go 1.26).
var (
	knownOS = map[string]bool{"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
		"illumos": true, "ios": true, "js": true, "linux": true, "netbsd": true, "openbsd": true, "plan9": true,
		"solaris": true, "wasip1": true, "windows": true}
	knownArch = map[string]bool{"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true, "mips": true,
		"mipsle": true, "mips64": true, "mips64le": true, "ppc64": true, "ppc64le": true, "riscv64": true, "s390x": true,
		"wasm": true}
)

// ciRunsIt etiket yalnızca CI'ın bir kipinde zaten koşan koşullardan mı oluşur: `testing.Short()` (uzun kip -short'suz
// koşar) ve `raceEnabled` (yarış dedektörsüz kip). Böyle bir test ortama bağlı sayılmaz.
func ciRunsIt(label string) bool {
	for _, or := range strings.Split(label, " || ") {
		for _, and := range strings.Split(or, " && ") {
			switch strings.Trim(and, "() ") {
			case "testing.Short()", "raceEnabled", "!raceEnabled":
			default:
				return false
			}
		}
	}
	return true
}

// source ifadenin kaynak metnidir (tek satır).
func (x *goExtractor) source(e ast.Expr) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, token.NewFileSet(), e); err != nil {
		return "?"
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

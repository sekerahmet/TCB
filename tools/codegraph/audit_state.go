package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// `package-state`: süreç genelinde değişebilir durum. `init` ve değişken bildirimi dışında yazılan
// paket düzeyi değişkenler (atama, ++/--, harita ya da dilim öğesi ya da alan yazımı, adresinin
// alınması, delete/clear): aynı süreçte koşan programlar ya da görevler bunu paylaşır. Yayın
// uyarıları bir zamanlar böyleydi (paket değişkenleri `givenWarning`, `warningSilenced`,
// `warningOutput`; aynı süreçteki programlar birbirinin uyarısını bastırıyordu); kural: programa ya
// da göreve ait durum `VM.ProgramValue`/`VM.TaskValue`'dadır (internal/README.md "Testler kararlıdır").
// Sınıflar:
//   - mutable: üretim kodunda `init` dışında yazılan değişken: aday;
//   - test-hook: yalnızca test dosyaları yazıyor (kancayı değiştirip geri koyan testler): testler
//     paralel koşarsa yarış kaynağı, ama üretimde sabit;
//   - synchronized: sync/atomic tipli değişken (Once, Mutex, atomic sayaç): paylaşım bilerek, ama
//     paylaşılan durumun varlığı görünsün diye ayrıca listelenir.
// Yalnızca okunan tablolar (haritalar, listeler) listelenmez, sayısı yazılır. Bir yerel değişken
// üzerinden (takma ad) yapılan yazım ve işaretçi yöntemleriyle yapılan değişiklik görülmez.

// stateWrite bir yazım yeridir.
type stateWrite struct {
	Func string `json:"func"`
	File string `json:"file"`
	Line int    `json:"line"`
	How  string `json:"how"`
	Test bool   `json:"test,omitempty"`
}

// stateVar bir paket değişkenidir.
type stateVar struct {
	Package string       `json:"package"`
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	File    string       `json:"file"`
	Line    int          `json:"line"`
	Kind    string       `json:"kind"`
	Writers []stateWrite `json:"writers,omitempty"`
}

// stateAudit denetimin ara tabloları.
type stateAudit struct {
	m          *goload.Module
	vars       map[string]*stateVar // konum anahtarı -> değişken
	order      []string
	atomicAddr map[*ast.UnaryExpr]bool // sync/atomic işlevine verilen &değişken ifadeleri
}

// vkey bir değişkenin tip denetimleri arasında değişmeyen anahtarıdır (bildirim konumu).
func (a *stateAudit) vkey(v *types.Var) string {
	p := a.m.Fset.Position(v.Pos())
	return fmt.Sprintf("%s:%d", p.Filename, p.Offset)
}

// findPackageState denetimi çalıştırır; sonuç paket ve ada göre sıralıdır, readOnly yalnızca okunan
// değişken sayısıdır.
func findPackageState(m *goload.Module) (vars []*stateVar, readOnly int) {
	a := &stateAudit{m: m, vars: map[string]*stateVar{}, atomicAddr: map[*ast.UnaryExpr]bool{}}
	a.collectVars()
	a.collectWrites()
	for _, k := range a.order {
		v := a.vars[k]
		switch {
		case isSyncType(v.Type) || allWriters(v.Writers, "atomic"):
			v.Kind = "synchronized"
		case len(v.Writers) == 0:
			readOnly++
			continue
		case onlyTests(v.Writers):
			v.Kind = "test-hook"
		case allWriters(v.Writers, "address taken"):
			v.Kind = "address-taken"
		default:
			v.Kind = "mutable"
		}
		vars = append(vars, v)
	}
	sort.SliceStable(vars, func(i, j int) bool {
		if vars[i].Kind != vars[j].Kind {
			return vars[i].Kind < vars[j].Kind
		}
		if vars[i].Package != vars[j].Package {
			return vars[i].Package < vars[j].Package
		}
		return vars[i].Name < vars[j].Name
	})
	return vars, readOnly
}

// isSyncType tip metni (bir alanı da dahil) sync ya da sync/atomic paketinden bir tip içeriyor mu: kendi kilidiyle
// korunan ya da atomik durum.
func isSyncType(t string) bool {
	return strings.Contains(t, "sync.") || strings.Contains(t, "sync/atomic.")
}

// allWriters bütün yazanlar verilen biçimde mi.
func allWriters(ws []stateWrite, how string) bool {
	for _, w := range ws {
		if w.How != how {
			return false
		}
	}
	return len(ws) > 0
}

func onlyTests(ws []stateWrite) bool {
	for _, w := range ws {
		if !w.Test {
			return false
		}
	}
	return true
}

// collectVars modülün paket düzeyi değişkenlerini testsiz paketlerden toplar.
func (a *stateAudit) collectVars() {
	paths := make([]string, 0, len(a.m.Base))
	for p := range a.m.Base {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		scope := a.m.Base[p].Scope()
		for _, name := range scope.Names() {
			v, ok := scope.Lookup(name).(*types.Var)
			if !ok || name == "_" || v.IsField() {
				continue
			}
			file, line := position(a.m, v.Pos())
			if isTestFile(file) {
				continue
			}
			sv := &stateVar{Package: shortPkg(a.m, p), Name: name, File: file, Line: line,
				Type: strings.ReplaceAll(types.TypeString(v.Type(), func(pk *types.Package) string { return pk.Path() }), a.m.Path+"/", "")}
			k := a.vkey(v)
			a.vars[k] = sv
			a.order = append(a.order, k)
		}
	}
}

// collectWrites bütün dosyalardaki yazımları gezer.
func (a *stateAudit) collectWrites() {
	forEachFile(a.m, func(u *goload.Unit, f *ast.File, rel string) {
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.VAR {
				a.noteVarFuncLits(u, gd, rel) // değişken başlatıcılarındaki işlev değerleri sonra çalışır
			}
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || (fn.Recv == nil && fn.Name.Name == "init") {
				continue // değişken bildirimleri ve init: başlatma anı
			}
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				name = recvTypeName(fn.Recv.List[0].Type) + "." + name
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				a.noteWrites(u, n, name, rel)
				return true
			})
		}
	})
}

// noteVarFuncLits `var f = func() { ... }` başlatıcılarındaki işlev gövdelerini tarar: değişkenin değeri
// başlatma anında atanır ama gövde sonra, çağrıldıkça çalışır.
func (a *stateAudit) noteVarFuncLits(u *goload.Unit, gd *ast.GenDecl, rel string) {
	ast.Inspect(gd, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		ast.Inspect(lit.Body, func(m ast.Node) bool {
			a.noteWrites(u, m, "(var initializer func)", rel)
			return true
		})
		return false
	})
}

// noteAtomicCall `atomic.AddInt64(&v, 1)` gibi bir sync/atomic çağrısındaki &değişken bağımsız değişkenlerini
// "atomic" yazım olarak kaydeder (ayrıca "address taken" sayılmaz).
func (a *stateAudit) noteAtomicCall(u *goload.Unit, c *ast.CallExpr, record func(ast.Expr, string)) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	fn, ok := u.Info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "sync/atomic" {
		return
	}
	for _, arg := range c.Args {
		if un, ok := arg.(*ast.UnaryExpr); ok && un.Op == token.AND {
			a.atomicAddr[un] = true
			record(un.X, "atomic")
		}
	}
}

// noteWrites bir düğümün yazdığı paket değişkenlerini kaydeder.
func (a *stateAudit) noteWrites(u *goload.Unit, n ast.Node, fn, rel string) {
	record := func(target ast.Expr, how string) {
		if v := a.packageVar(u, target); v != nil {
			if sv := a.vars[a.vkey(v)]; sv != nil {
				if _, bare := stripParens(target).(*ast.Ident); !bare && how == "assign" {
					how = "element/field write"
				}
				file, line := position(a.m, target.Pos())
				sv.Writers = append(sv.Writers, stateWrite{Func: fn, File: file, Line: line, How: how, Test: isTestFile(rel)})
			}
		}
	}
	switch s := n.(type) {
	case *ast.AssignStmt:
		if s.Tok != token.DEFINE {
			for _, l := range s.Lhs {
				record(l, "assign")
			}
		}
	case *ast.IncDecStmt:
		record(s.X, "inc/dec")
	case *ast.UnaryExpr:
		if s.Op == token.AND && !a.atomicAddr[s] {
			record(s.X, "address taken")
		}
	case *ast.RangeStmt:
		if s.Tok == token.ASSIGN {
			if s.Key != nil {
				record(s.Key, "assign")
			}
			if s.Value != nil {
				record(s.Value, "assign")
			}
		}
	case *ast.CallExpr:
		a.noteAtomicCall(u, s, record)
		if id, ok := s.Fun.(*ast.Ident); ok && len(s.Args) > 0 && (id.Name == "delete" || id.Name == "clear") {
			if _, isBuiltin := u.Info.Uses[id].(*types.Builtin); isBuiltin {
				record(s.Args[0], "delete/clear")
			}
		}
	}
}

func stripParens(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// packageVar bir yazım hedefinin köküne inip modülün paket düzeyi değişkenini verir; değilse nil.
func (a *stateAudit) packageVar(u *goload.Unit, e ast.Expr) *types.Var {
	for {
		switch v := e.(type) {
		case *ast.ParenExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.IndexListExpr:
			e = v.X
		case *ast.SliceExpr:
			e = v.X
		case *ast.SelectorExpr:
			if x, ok := v.X.(*ast.Ident); ok {
				if _, isPkg := u.Info.Uses[x].(*types.PkgName); isPkg {
					return pkgLevel(u.Info.Uses[v.Sel])
				}
			}
			e = v.X
		case *ast.Ident:
			return pkgLevel(u.Info.Uses[v])
		default:
			return nil
		}
	}
}

// pkgLevel nesne bir paket düzeyi değişken ise onu verir.
func pkgLevel(o types.Object) *types.Var {
	v, ok := o.(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		return nil
	}
	return v
}

// runPackageState `package-state` komutudur.
func runPackageState(args []string, stdout, stderr io.Writer) int {
	return runGoAudit("package-state", args, false, stdout, stderr, func(m *goload.Module, o *auditOpts) (map[string]any, func(io.Writer)) {
		all, readOnly := findPackageState(m)
		var vars []*stateVar
		for _, v := range all {
			if underIn(v.File, o.in) {
				vars = append(vars, v)
			}
		}
		return map[string]any{"variables": vars, "read_only": readOnly}, func(w io.Writer) { printPackageState(w, vars, readOnly, o.limit) }
	})
}

func printPackageState(w io.Writer, vars []*stateVar, readOnly, limit int) {
	fmt.Fprintf(w, "%d package variable(s) with process-wide state (%d more are only read after init)\n", len(vars), readOnly)
	for _, kind := range []string{"mutable", "test-hook", "address-taken", "synchronized"} {
		var of []*stateVar
		for _, v := range vars {
			if v.Kind == kind {
				of = append(of, v)
			}
		}
		if len(of) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s (%d):\n", kind, len(of))
		forEachLimited(w, of, limit, "  ", moreHint, func(v *stateVar) string {
			s := fmt.Sprintf("%s.%s %s  %s:%d", v.Package, v.Name, v.Type, v.File, v.Line)
			if len(v.Writers) > 0 {
				s += "  written by " + describeWriters(v.Writers)
			}
			return s
		})
	}
}

// describeWriters yazanları "işlev (nasıl)" olarak sıralar; en çok üç tanesini gösterir.
func describeWriters(ws []stateWrite) string {
	seen := map[string]bool{}
	var parts []string
	for _, w := range ws {
		label := w.Func + " (" + w.How + ")"
		if w.Test {
			label += " [test]"
		}
		if !seen[label] {
			seen[label] = true
			parts = append(parts, label)
		}
	}
	if len(parts) > 3 {
		return strings.Join(parts[:3], ", ") + fmt.Sprintf(", +%d more", len(parts)-3)
	}
	return strings.Join(parts, ", ")
}

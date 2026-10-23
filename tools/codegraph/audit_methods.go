package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// `dead-methods`: çağıranı olmayan ya da yalnızca derleme anı arayüz denetimiyle (`var _ I = (*T)(nil)`)
// canlı görünen metotlar. deadcode bunları görmez: tip bir arayüze atandığında metot "erişilebilir"
// sayılır. Sınıflar:
//   - unreferenced: hiçbir kullanımı yok ve hiçbir modül arayüzünün metodu değil;
//   - assertion-only: bir modül arayüzünü sağlıyor ama o arayüzün bu adlı metodu hiçbir yerde
//     çağrılmıyor: metot yalnızca arayüz denetimiyle tutuluyor;
//   - shadowed: arayüz üzerinden çağrılıyor ama çağrı yeri, aynı işlevde daha önce yapılan ve
//     dönen başka bir arayüz denetimiyle gölgeleniyor (`if k, ok := x.(TaskOperatorHook); ok {
//     return ... }` sonra `x.(OperatorHook)`: ikisini de sağlayan tip için ikincisi hiç çalışmaz);
//   - tests-only: yalnızca test dosyalarından kullanılıyor.
// Dış sözleşme adları (String, Error, Write ...) başka paketlerin çağırdığı metotlardır, listelenmez.
// Reflect ve kod üretimiyle çağrılan metotlar görülmez: çıktı karardır değil, adaydır.

// deadMethod bir adaydır.
type deadMethod struct {
	Method string `json:"method"` // paket.Tür.metot
	File   string `json:"file"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// externalContract yaygın standart kitaplık arayüzlerinin metot adlarıdır: çağıranları başka
// paketlerdedir (fmt, sort, io, encoding/json ...) ve burada görülmez.
var externalContract = map[string]bool{
	"String": true, "Error": true, "Unwrap": true, "Is": true, "As": true, "Format": true, "GoString": true,
	"Write": true, "WriteString": true, "WriteByte": true, "WriteRune": true, "WriteTo": true,
	"Read": true, "ReadFrom": true, "ReadByte": true, "ReadRune": true, "Close": true, "Seek": true, "Flush": true,
	"Len": true, "Less": true, "Swap": true, "Push": true, "Pop": true,
	"MarshalJSON": true, "UnmarshalJSON": true, "MarshalText": true, "UnmarshalText": true,
	"Set": true, "Type": true, "Lock": true, "Unlock": true, "Next": true, "Done": true, "Err": true,
	"Deadline": true, "Value": true, "Import": true, "Scan": true, "Run": true, "Visit": true, "Handle": true, "ServeHTTP": true,
}

// Not: goload her paketi iki kez denetler (testsiz ve testli) ve içe aktaran paketler testsiz olanı
// görür; bu yüzden aynı yöntem için farklı *types.Func nesneleri vardır. Bildirimler ve kullanımlar
// bu nedenle nesneyle değil bildirim konumuyla (dosya:ofset) eşlenir.

// methodDecl bir yöntem bildirimidir.
type methodDecl struct {
	fn   *types.Func
	recv *types.Named
	file string
	line int
}

// methodAudit denetimin ara tabloları.
type methodAudit struct {
	m         *goload.Module
	decls     []methodDecl
	prodUses  map[string]int // konum anahtarı -> üretim dosyalarındaki kullanım sayısı
	testUses  map[string]int
	ifaces    []*types.Named            // modülün adlı arayüzleri
	ifaceByID map[string]*types.Named   // tip anahtarı -> arayüz
	concrete  []*types.Named            // modülün adlı somut tipleri
	implement map[string][]*types.Named // somut tip anahtarı -> sağladığı modül arayüzleri
	satisfies map[string][]ifaceSlot    // yöntem anahtarı -> karşıladığı arayüz yöntemleri (gömülü alandan yükseltilen dahil)
	shadow    map[string]string         // "tip.metot" -> gölgeleme açıklaması
}

// fkey bir işlev nesnesinin tip denetimleri arasında değişmeyen anahtarıdır (bildirim konumu).
func (a *methodAudit) fkey(fn *types.Func) string {
	p := a.m.Fset.Position(fn.Origin().Pos())
	return fmt.Sprintf("%s:%d", p.Filename, p.Offset)
}

// typeKey bir adlı tipin paket yoluyla nitelenmiş adıdır.
func typeKey(t *types.Named) string { return t.Obj().Pkg().Path() + "." + t.Obj().Name() }

// findDeadMethods denetimi çalıştırır.
func findDeadMethods(m *goload.Module) []deadMethod {
	a := &methodAudit{m: m, prodUses: map[string]int{}, testUses: map[string]int{}, ifaceByID: map[string]*types.Named{}}
	a.collectTypes()
	a.collectDecls()
	a.collectUses()
	a.collectShadows()
	var out []deadMethod
	for _, d := range a.decls {
		if dm, ok := a.classify(d); ok {
			out = append(out, dm)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// collectTypes modülün adlı arayüzlerini ve somut tiplerini, hangi tipin hangi arayüzü sağladığını çıkarır
// (testsiz paketlerden: bildirimler ve içe aktaranların gördüğü tipler).
func (a *methodAudit) collectTypes() {
	var pairs []implPair
	a.concrete, a.ifaces, pairs = moduleImplementations(a.m)
	for _, i := range a.ifaces {
		a.ifaceByID[typeKey(i)] = i
	}
	a.implement = map[string][]*types.Named{}
	a.satisfies = map[string][]ifaceSlot{}
	for _, p := range pairs {
		a.implement[typeKey(p.concrete)] = append(a.implement[typeKey(p.concrete)], p.iface)
		// dispatches_to ile aynı kural (implementingMethods, types.NewMethodSet): bir yöntem, gömüldüğü tipin
		// sağladığı arayüzü de karşılar (tensor.VersionCounter.Hold, cpuStorage ve gpu.storage üzerinden Storage.Hold).
		for _, pair := range implementingMethods(p) {
			k := a.fkey(pair[1])
			a.satisfies[k] = append(a.satisfies[k], ifaceSlot{p.iface, pair[0]})
		}
	}
}

// ifaceSlot bir arayüzün bir yöntemidir.
type ifaceSlot struct {
	iface  *types.Named
	method *types.Func
}

// collectDecls somut tiplerin bildirilmiş yöntemlerini toplar.
func (a *methodAudit) collectDecls() {
	for _, t := range a.concrete {
		for i := 0; i < t.NumMethods(); i++ {
			fn := t.Method(i)
			file, line := position(a.m, fn.Pos())
			if isTestFile(file) {
				continue
			}
			a.decls = append(a.decls, methodDecl{fn: fn, recv: t, file: file, line: line})
		}
	}
}

// collectUses her işlev ve arayüz metodunun kullanım sayısını (üretim / test) çıkarır; bir işlevin kendi
// gövdesindeki (özyinelemeli) kullanım sayılmaz.
func (a *methodAudit) collectUses() {
	forEachFile(a.m, func(u *goload.Unit, f *ast.File, rel string) {
		counts := a.prodUses
		if isTestFile(rel) {
			counts = a.testUses
		}
		for _, d := range f.Decls {
			self := ""
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, _ := u.Info.Defs[fd.Name].(*types.Func); fn != nil {
					self = a.fkey(fn)
				}
			}
			ast.Inspect(d, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if fn, ok := u.Info.Uses[id].(*types.Func); ok {
					if k := a.fkey(fn); k != self {
						counts[k]++
					}
				}
				return true
			})
		}
	})
}

// ifaceMethod arayüzün adlı metodunu verir (gömülü arayüzlerinki dahil); yoksa nil.
func ifaceMethod(i *types.Named, name string) *types.Func {
	it := i.Underlying().(*types.Interface)
	for k := 0; k < it.NumMethods(); k++ {
		if it.Method(k).Name() == name {
			return it.Method(k)
		}
	}
	return nil
}

// classify bir yöntemin sınıfını verir; canlıysa ok=false.
func (a *methodAudit) classify(d methodDecl) (deadMethod, bool) {
	name := d.fn.Name()
	key := a.fkey(d.fn)
	dm := deadMethod{Method: shortPkg(a.m, d.recv.Obj().Pkg().Path()) + "." + d.recv.Obj().Name() + "." + name, File: d.file, Line: d.line}
	if a.prodUses[key] > 0 || externalContract[name] || name == "init" {
		return dm, false
	}
	var satisfied []string
	called := false
	for _, s := range a.satisfies[key] {
		if !slices.Contains(satisfied, s.iface.Obj().Name()) {
			satisfied = append(satisfied, s.iface.Obj().Name())
		}
		called = called || a.prodUses[a.fkey(s.method)] > 0
	}
	switch {
	case called:
		if why, ok := a.shadow[typeKey(d.recv)+"."+name]; ok {
			dm.Kind, dm.Detail = "shadowed", why
			return dm, true
		}
		return dm, false
	case a.testUses[key] > 0:
		dm.Kind = "tests-only"
	case len(satisfied) > 0 && isSealedMarker(d.fn):
		return dm, false
	case len(satisfied) > 0:
		dm.Kind, dm.Detail = "assertion-only", "satisfies "+strings.Join(satisfied, ", ")+"; that interface method is never called"
	default:
		dm.Kind = "unreferenced"
	}
	return dm, true
}

// isSealedMarker kapalı arayüz işaretçisi mi: parametresiz ve sonuçsuz, dışa kapalı adlı metot
// (`func (X) statementNode()`). Hiç çağrılmaması beklenir; yalnızca tipin arayüze ait olduğunu söyler.
func isSealedMarker(fn *types.Func) bool {
	sig := fn.Type().(*types.Signature)
	return !fn.Exported() && sig.Params().Len() == 0 && sig.Results().Len() == 0
}

// guard bir işlevdeki arayüz denetimidir (x.(I) ya da type switch kolu). returns: denetim tutunca işlev
// biter ya da kol dışa kapalıdır, yani sonraki denetimler o tip için erişilemez.
type guard struct {
	iface   *types.Named
	returns bool
	file    string
	line    int
	pos     token.Pos    // denetimin başladığı yer
	end     token.Pos    // if deyiminin bittiği yer
	else_   [2]token.Pos // else kolunun aralığı (yoksa sıfır)
	sw      ast.Node     // type switch ise deyimin kendisi: aynı switch'in kolları birbirini dışlar
	idx     int          // switch içindeki sıra
}

// collectShadows her işlevdeki arayüz denetimlerini sırayla gezer; iki modül arayüzünü birden sağlayan
// her tip için, sonraki arayüzün önceki arayüzde olmayan metotlarını gölgeli sayar.
func (a *methodAudit) collectShadows() {
	a.shadow = map[string]string{}
	forEachFile(a.m, func(u *goload.Unit, f *ast.File, rel string) {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			gs := a.guardsOf(u, fd.Body)
			for x := 0; x < len(gs); x++ {
				for y := x + 1; y < len(gs); y++ {
					if excludes(gs[x], gs[y]) {
						a.markShadowed(gs[x], gs[y])
					}
				}
			}
		}
	})
}

// excludes g1 tutunca g2'ye hiç gelinmez mi: aynı type switch'in sonraki kolu; ya da dönen bir if'in
// ardındaki ya da else kolundaki denetim. g1'in kendi gövdesine yerleşmiş bir denetim (iç içe) gölgelenmez.
func excludes(g1, g2 guard) bool {
	if g1.sw != nil {
		return g2.sw == g1.sw && g2.idx > g1.idx
	}
	if !g1.returns {
		return false
	}
	return g2.pos >= g1.end || g2.pos >= g1.else_[0] && g2.pos < g1.else_[1]
}

// guardsOf bir gövdedeki arayüz denetimlerini kaynak sırasıyla verir.
func (a *methodAudit) guardsOf(u *goload.Unit, body *ast.BlockStmt) []guard {
	var out []guard
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt:
			if i := a.assertedIface(u, s.Init); i != nil {
				file, line := position(a.m, s.Pos())
				g := guard{iface: i, returns: endsWithReturn(s.Body), file: file, line: line, pos: s.Pos(), end: s.End()}
				if s.Else != nil {
					g.else_ = [2]token.Pos{s.Else.Pos(), s.Else.End()}
				}
				out = append(out, g)
			}
		case *ast.TypeSwitchStmt:
			idx := 0
			for _, c := range s.Body.List {
				cc := c.(*ast.CaseClause)
				for _, e := range cc.List {
					if i := a.ifaceOfType(u, e); i != nil {
						file, line := position(a.m, cc.Pos())
						out = append(out, guard{iface: i, returns: true, file: file, line: line, pos: cc.Pos(), end: cc.End(), sw: s, idx: idx})
						idx++
					}
				}
			}
		}
		return true
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].pos < out[j].pos })
	return out
}

// assertedIface `k, ok := x.(I)` biçimindeki başlatıcının I'sini (modül arayüzüyse) verir.
func (a *methodAudit) assertedIface(u *goload.Unit, init ast.Stmt) *types.Named {
	as, ok := init.(*ast.AssignStmt)
	if !ok || len(as.Rhs) != 1 {
		return nil
	}
	ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || ta.Type == nil {
		return nil
	}
	return a.ifaceOfType(u, ta.Type)
}

// ifaceOfType bir tip ifadesinin modül arayüzü olan adlı tipini (testsiz paketteki karşılığıyla) verir.
func (a *methodAudit) ifaceOfType(u *goload.Unit, e ast.Expr) *types.Named {
	tv, ok := u.Info.Types[e]
	if !ok {
		return nil
	}
	n, _ := tv.Type.(*types.Named)
	if n == nil || !types.IsInterface(n) || n.Obj().Pkg() == nil {
		return nil
	}
	return a.ifaceByID[typeKey(n)]
}

// endsWithReturn bloğun son deyimi return mü.
func endsWithReturn(b *ast.BlockStmt) bool {
	if b == nil || len(b.List) == 0 {
		return false
	}
	_, ok := b.List[len(b.List)-1].(*ast.ReturnStmt)
	return ok
}

// markShadowed first'ü sağlayan ve later'i de sağlayan her tip için later'in first'te olmayan
// metotlarını gölgeli işaretler.
func (a *methodAudit) markShadowed(first, later guard) {
	for _, t := range a.concrete {
		both := 0
		for _, i := range a.implement[typeKey(t)] {
			if i == first.iface || i == later.iface {
				both++
			}
		}
		if both < 2 {
			continue
		}
		it := later.iface.Underlying().(*types.Interface)
		for k := 0; k < it.NumMethods(); k++ {
			name := it.Method(k).Name()
			if ifaceMethod(first.iface, name) == nil {
				a.shadow[typeKey(t)+"."+name] = fmt.Sprintf("%s check at %s:%d never runs for %s: the earlier %s check at %s:%d returns first",
					later.iface.Obj().Name(), later.file, later.line, t.Obj().Name(), first.iface.Obj().Name(), first.file, first.line)
			}
		}
	}
}

// runDeadMethods `dead-methods` komutudur.
func runDeadMethods(args []string, stdout, stderr io.Writer) int {
	return runGoAudit("dead-methods", args, true, stdout, stderr, func(m *goload.Module, o *auditOpts) (map[string]any, func(io.Writer)) {
		var list []deadMethod
		for _, d := range findDeadMethods(m) {
			if underIn(d.File, o.in) {
				list = append(list, d)
			}
		}
		return map[string]any{"methods": list}, func(w io.Writer) { printDeadMethods(w, list, o.limit) }
	})
}

func printDeadMethods(w io.Writer, list []deadMethod, limit int) {
	fmt.Fprintf(w, "%d method candidate(s)\n", len(list))
	for _, kind := range []string{"unreferenced", "assertion-only", "shadowed", "tests-only"} {
		var of []deadMethod
		for _, d := range list {
			if d.Kind == kind {
				of = append(of, d)
			}
		}
		if len(of) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s (%d):\n", kind, len(of))
		forEachLimited(w, of, limit, "  ", moreHint, func(d deadMethod) string {
			s := fmt.Sprintf("%s  %s:%d", d.Method, d.File, d.Line)
			if d.Detail != "" {
				s += "  — " + d.Detail
			}
			return s
		})
	}
}

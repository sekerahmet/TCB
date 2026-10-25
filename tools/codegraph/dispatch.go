package main

import (
	"go/ast"
	"go/constant"
	"go/types"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Bu dosya TR dil yapılarını onları çalıştıran Go işlevlerine bağlar (deneme 9 H1, deneme 10; kod-grafigi-40 tasarımı,
// TCB onaylı). Gönderim tablosu elle yazılmaz, yorumlayıcının kodundan okunur:
//   - yapı: derleyicinin (interp.compiler metotları) `switch e := expr.(type) { case *ast.CallExpression: ... }`
//     dallarında çağrılan işlevler;
//   - işleç (yorumlayıcı ve internal/stdlib kancaları; denetleyicinin tür kuralları değil): işlecin metnini
//     dallayan `switch op { case "@": ... }` (ml'deki tensör işleçleri) ya da metni sabite çeviren
//     (`case "+", "+=": return opAdd`) ve o sabiti dallayan (`case opAdd: ...`) switch'lerin dalları. Dalda çağrı yoksa
//     işlem dalın kendisindedir; uygulama o switch'i taşıyan işlevdir.
// Dal bir derleme yardımcısı çağırıyorsa (yorumlayıcının compiler metodu: `case *ast.InterpolatedString: return
// c.interpolation(...)`) yardımcının (iç kapanışları dahil) çağırdıkları da o yapının uygulamasıdır, zincir halinde
// (en çok testMaxSteps adım): kodu bir yardımcıya taşımak bağı koparmaz. Kendi AST tür switch'ini taşıyan genel derleme
// metotlarına (compiler.expression gibi, her yapıyı dağıtan) yardımcı üzerinden inilmez.
// codeindex her TR bildiriminin kullandığı yapıları kaydeder (Construct); her biri bu tablodaki işlevlere
// `implemented_by` kenarı (INFERRED) olur. İşlenen türü biliniyorsa (tensör, sayı, metin ...) yalnız o türün
// uygulamalarına gider: Tensör yorumlayıcı dışındaki kancalara (ml), yerleşik değer türleri yorumlayıcıya.

// trOperators TR'nin işleç metinleridir: işleç switch'i yalnızca bunlardan oluşan dallarıyla tanınır.
var trOperators = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true, "**": true, "@": true, "<": true, ">": true, "<=": true,
	">=": true, "==": true, "!=": true, "&&": true, "||": true, "??": true, "|>": true, "!": true,
	"+=": true, "-=": true, "*=": true, "/=": true,
}

// dispatchTable dil yapısından ve işleçten Go uygulamalarına giden tablodur.
type dispatchTable struct {
	kinds map[string][]string // AST türü ("CallExpression") -> işlev kimlikleri
	ops   map[string][]string // işleç ("@") -> işlev kimlikleri
	pkg   map[string]string   // işlev kimliği -> paket yolu (işlenen türüne göre süzmek için)
}

// readDispatch yorumlayıcının ve kancaların gönderim tablosunu modülden okur.
func readDispatch(m *module) *dispatchTable {
	t := &dispatchTable{kinds: map[string][]string{}, ops: map[string][]string{}, pkg: map[string]string{}}
	astPkg, interpPkg := m.Path+"/internal/lang/ast", m.Path+"/internal/lang/interp"
	constOps := map[types.Object][]string{} // işleç sabiti -> metinleri (opCode'un dalları)
	var switches []opSwitch
	helpers := map[string]compileHelper{} // yorumlayıcının compiler metotları (yardımcı izlemek için)
	forEachFile(m, func(u *unit, f *ast.File, rel string) {
		if isTestFile(rel) {
			return
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			pkg := strings.TrimSuffix(u.PkgPath, "_test")
			if pkg != interpPkg && !strings.HasPrefix(pkg, m.Path+"/internal/stdlib/") {
				continue // gönderim çalışma anındadır: yorumlayıcı ve kancaları; denetleyicinin işleç kuralı değil
			}
			compiler := pkg == interpPkg && fd.Recv != nil && strings.Contains(recvTypeName(fd), "compiler")
			var self string
			if fn, ok := u.Info.Defs[fd.Name].(*types.Func); ok && compiler {
				self = funcRef(fn)
				helpers[self] = compileHelper{info: u.Info, body: fd.Body.List}
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.TypeSwitchStmt:
					if compiler && t.typeSwitch(u.Info, fd, s, astPkg, pkg) {
						h := helpers[self]
						h.dispatcher = true
						helpers[self] = h
					}
				case *ast.SwitchStmt:
					switches = append(switches, opSwitch{fd, s, u.Info, pkg})
					for _, cc := range s.Body.List {
						c := cc.(*ast.CaseClause)
						if lits := operatorLits(c.List); lits != nil && len(c.Body) == 1 {
							if r, ok := c.Body[0].(*ast.ReturnStmt); ok && len(r.Results) == 1 {
								if id, ok := r.Results[0].(*ast.Ident); ok {
									if obj, ok := u.Info.Uses[id].(*types.Const); ok {
										constOps[obj] = append(constOps[obj], lits...)
									}
								}
							}
						}
					}
				}
				return true
			})
		}
	})
	t.followHelpers(helpers, m.Path, astPkg, interpPkg)
	t.operatorBranches(switches, constOps, m.Path, astPkg)
	return t
}

// opSwitch yorumlayıcıda ya da bir kancada bir switch deyimidir (işleç dalları için ikinci geçiş).
type opSwitch struct {
	fd   *ast.FuncDecl
	s    *ast.SwitchStmt
	info *types.Info
	pkg  string
}

// operatorBranches işleç sabitleri bilinince ikinci geçiştir: işleç metnini ya da sabitini dallayan switch'lerin
// dallarındaki çağrılar (çağrısız dalda switch'i taşıyan işlev) o işlecin uygulamalarıdır.
func (t *dispatchTable) operatorBranches(switches []opSwitch, constOps map[types.Object][]string, mod, astPkg string) {
	for _, s := range switches {
		for _, cc := range s.s.Body.List {
			c := cc.(*ast.CaseClause)
			ops := operatorLits(c.List)
			if ops == nil {
				ops = constLits(s.info, c.List, constOps)
			}
			if ops == nil || returnsOnlyValue(c.Body) {
				continue // işleç dalı değil ya da yalnız bir değer döndüren eşleme dalı (opCode, metin tablosu)
			}
			impl := clauseCallees(s.info, c.Body, mod, astPkg)
			if len(impl) == 0 {
				if fn, ok := s.info.Defs[s.fd.Name].(*types.Func); ok {
					impl = []string{funcRef(fn)}
				}
			}
			for _, op := range ops {
				for _, id := range impl {
					t.add(t.ops, op, id, s.pkg)
				}
			}
		}
	}
}

func (t *dispatchTable) add(m map[string][]string, key, id, pkg string) {
	for _, have := range m[key] {
		if have == id {
			return
		}
	}
	m[key] = append(m[key], id)
	t.pkg[id] = pkg
}

// compileHelper yorumlayıcının bir compiler metodudur: gövdesi ve AST türlerini dağıtıp dağıtmadığı.
type compileHelper struct {
	info       *types.Info
	body       []ast.Stmt
	dispatcher bool // kendi AST tür switch'i var: her yapıyı derleyen genel metot
}

// followHelpers her yapının uygulama listesine dalın çağırdığı derleme yardımcılarının (zincir halinde, en çok
// testMaxSteps adım) çağırdıklarını ekler; dağıtıcı metotlara yardımcı üzerinden inilmez, onlar eklenmez.
func (t *dispatchTable) followHelpers(helpers map[string]compileHelper, mod, astPkg, pkg string) {
	for kind, direct := range t.kinds {
		seen := map[string]bool{}
		frontier := append([]string(nil), direct...)
		for _, id := range direct {
			seen[id] = true
		}
		for step := 0; step < testMaxSteps && len(frontier) > 0; step++ {
			var next []string
			for _, id := range frontier {
				h, ok := helpers[id]
				if !ok || h.dispatcher {
					continue
				}
				for _, callee := range clauseCallees(h.info, h.body, mod, astPkg) {
					if seen[callee] || helpers[callee].dispatcher {
						continue
					}
					seen[callee] = true
					t.add(t.kinds, kind, callee, pkg)
					next = append(next, callee)
				}
			}
			frontier = next
		}
	}
}

// typeSwitch derleyicinin AST türü dallarını tabloya yazar; switch AST türlerini dallıyorsa true döner.
func (t *dispatchTable) typeSwitch(info *types.Info, fd *ast.FuncDecl, s *ast.TypeSwitchStmt, astPkg, pkg string) (kinds bool) {
	for _, cc := range s.Body.List {
		c := cc.(*ast.CaseClause)
		for _, e := range c.List {
			star, ok := e.(*ast.StarExpr)
			if !ok {
				continue
			}
			id := nameIdent(star.X)
			tn, ok := info.Uses[id].(*types.TypeName)
			if id == nil || !ok || tn.Pkg() == nil || tn.Pkg().Path() != astPkg {
				continue
			}
			kinds = true
			for _, f := range clauseCallees(info, c.Body, "", astPkg) {
				t.add(t.kinds, tn.Name(), f, pkg)
			}
		}
	}
	return kinds
}

// operatorLits dal ifadeleri hepsi TR işleci olan metinlerse onları verir; değilse nil.
func operatorLits(list []ast.Expr) []string {
	var out []string
	for _, e := range list {
		lit, ok := e.(*ast.BasicLit)
		if !ok {
			return nil
		}
		s, ok := stringLit(lit)
		if !ok || !trOperators[s] {
			return nil
		}
		out = append(out, s)
	}
	return out
}

// constLits dal ifadeleri hepsi işleç sabitiyse onların metinlerini verir; değilse nil.
func constLits(info *types.Info, list []ast.Expr, constOps map[types.Object][]string) []string {
	var out []string
	for _, e := range list {
		id, ok := e.(*ast.Ident)
		if !ok {
			return nil
		}
		obj, ok := info.Uses[id].(*types.Const)
		if !ok || len(constOps[obj]) == 0 || obj.Val().Kind() != constant.Int {
			return nil
		}
		out = append(out, constOps[obj]...)
	}
	return out
}

// returnsOnlyValue dal yalnızca bir değişmez ya da ad döndürüyor mu (eşleme dalı, işlem değil).
func returnsOnlyValue(body []ast.Stmt) bool {
	if len(body) != 1 {
		return false
	}
	r, ok := body[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) == 0 {
		return false
	}
	for _, e := range r.Results {
		switch e.(type) {
		case *ast.BasicLit, *ast.Ident:
		default:
			return false
		}
	}
	return true
}

// clauseCallees dalın (iç kapanışları dahil) çağırdığı modül işlevleridir; AST ve ileti yardımcıları hariç.
func clauseCallees(info *types.Info, body []ast.Stmt, mod, astPkg string) []string {
	seen := map[string]bool{}
	var out []string
	for _, st := range body {
		ast.Inspect(st, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := info.Uses[nameIdent(c.Fun)].(*types.Func)
			if !ok || fn.Pkg() == nil || fn.Pkg().Path() == astPkg || strings.HasSuffix(fn.Pkg().Path(), "/internal/lang/messages") {
				return true
			}
			if mod != "" && !strings.HasPrefix(fn.Pkg().Path(), mod) {
				return true
			}
			if id := funcRef(fn); id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
			return true
		})
	}
	sort.Strings(out)
	return out
}

// recvTypeName metodun alıcı tipinin adıdır.
func recvTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// linkConstructs TR bildirimlerinin dil yapılarını gönderim tablosundaki Go işlevlerine `implemented_by` ile bağlar.
func linkConstructs(b *builder, t *dispatchTable, idx *codeindex.Index, ids map[string]string, interpPkg string) {
	for _, c := range idx.Constructs {
		from := ids[c.In]
		if from == "" {
			continue
		}
		targets := t.kinds[c.Kind]
		if c.Op != "" {
			targets = t.ops[c.Op]
		}
		for _, to := range targets {
			if operandRules(c.Operand, t.pkg[to] == interpPkg) {
				b.addEdge(from, to, edgeImplementedBy, inferred, c.Pos.File, c.Pos.Line)
			}
		}
	}
}

// operandRules işlenen türünün bu uygulamayla çalışıp çalışmadığıdır: Tensör (A212) yorumlayıcı dışındaki kancalarla,
// yerleşik değer türleri (Tam, Ondalık, Metin, Mantık, Liste, Sözlük) yorumlayıcıyla; bilinmeyen tür ikisiyle.
func operandRules(operand string, inInterp bool) bool {
	switch operand {
	case "":
		return true
	case "type:TR_type_Tensor":
		return !inInterp
	case "type:TR_type_Int", "type:TR_type_Float", "type:TR_type_String", "type:TR_type_Bool", "type:TR_type_List", "type:TR_type_Dict":
		return inInterp
	}
	return true // yapı ya da başka tür: kancası da olabilir
}

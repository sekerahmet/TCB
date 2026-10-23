package main

import (
	"go/ast"
	"go/types"
	"regexp"
	"sort"
)

// Kod kayıtlarının paketleri (modül köküne göre) ve sabit adlarının kalıpları.
const (
	messagesDir = "internal/lang/messages"
	codesDir    = "internal/lang/codes"
)

var (
	messageConst = regexp.MustCompile(`^H\d{4}$`)
	nameConst    = regexp.MustCompile(`^[A-Z]\d{3}$`)
)

// scan root altındaki her tanımlayıcıyı çözer: işlev kullanımı `calls`, ileti kodu sabiti
// `uses_message`, ad kodu sabiti `uses_name` kenarı olur. Kenar owner'dan çıkar.
func (x *goExtractor) scan(fc *fileCtx, owner string, root ast.Node) {
	callee := map[*ast.Ident]bool{}
	modes := map[ast.Expr]string{}
	closures := map[*ast.FuncLit]string{}
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		accessModes(modes, n)
		switch n := n.(type) {
		case *ast.CallExpr:
			if id := nameIdent(n.Fun); id != nil {
				callee[id] = true
			}
			x.envUse(fc, owner, n)
		case *ast.Ident:
			x.use(fc, owner, n, callee[n])
		case *ast.FuncLit:
			closures[n] = x.closureNode(fc, owner, n, stack, closures)
		case *ast.SelectorExpr:
			x.fieldAccess(fc, owner, n, modes[n], enclosingClosure(stack, closures))
		case *ast.CompositeLit:
			x.literalFields(fc, owner, n, enclosingClosure(stack, closures))
		}
		return true
	})
}

// enclosingClosure yığındaki en iç func literal'in düğüm kimliğidir (yoksa "").
func enclosingClosure(stack []ast.Node, closures map[*ast.FuncLit]string) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if lit, ok := stack[i].(*ast.FuncLit); ok {
			return closures[lit]
		}
	}
	return ""
}

// fieldAccess alan seçicisini (x.f) tip bilgisiyle çözer ve ham erişim olarak biriktirir.
func (x *goExtractor) fieldAccess(fc *fileCtx, owner string, sel *ast.SelectorExpr, mode, closure string) {
	s := fc.u.Info.Selections[sel]
	if s == nil || s.Kind() != types.FieldVal {
		return
	}
	if mode == "" {
		mode = modeRead
	}
	x.fieldUses = append(x.fieldUses, rawFieldUse{posKey: x.posKey(s.Obj().Pos()), owner: owner,
		site: Site{File: fc.rel, Line: x.line(sel.Sel.Pos()), Mode: mode, Closure: closure}})
}

// literalFields T{f: v} bileşik değerindeki anahtarlı alanları yazma olarak biriktirir.
func (x *goExtractor) literalFields(fc *fileCtx, owner string, lit *ast.CompositeLit, closure string) {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if v, ok := fc.u.Info.Uses[key].(*types.Var); ok && v.IsField() {
			x.fieldUses = append(x.fieldUses, rawFieldUse{posKey: x.posKey(v.Pos()), owner: owner,
				site: Site{File: fc.rel, Line: x.line(key.Pos()), Mode: modeLiteral, Closure: closure}})
		}
	}
}

// nameIdent bir ifadenin adını verir: parantez, gösterici ve tip bağımsız değişkenleri atılır, `pkg.Ad`ta
// Ad alınır. Gömülü alanın tipi (*pkg.T, T[int]) ve çağrılan ifade (f, pkg.F, F[int], (f)) aynı kuralla
// çözülür.
func nameIdent(e ast.Expr) *ast.Ident {
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
		case *ast.SelectorExpr:
			return v.Sel
		case *ast.Ident:
			return v
		default:
			return nil
		}
	}
}

func (x *goExtractor) use(fc *fileCtx, owner string, id *ast.Ident, isCall bool) {
	obj := fc.u.Info.Uses[id]
	if obj == nil || obj.Pkg() == nil || !x.inModule(obj.Pkg().Path()) {
		return
	}
	line := x.line(id.Pos())
	switch o := obj.(type) {
	case *types.Func:
		ref := funcRef(o.Origin())
		if ref == "" {
			return
		}
		conf := inferred // işlev değeri olarak kullanım: çağrı sonradan olabilir
		if isCall {
			conf = extracted
		}
		x.b.addEdge(owner, ref, edgeCalls, conf, fc.rel, line)
	case *types.Const:
		// Kodun kendi paketindeki (test dışı) kullanım tanım tablolarıdır; kullanım sayılmaz.
		samePkg := o.Pkg().Path() == fc.base && !fc.isTest
		switch {
		case samePkg:
		case o.Pkg().Path() == x.m.Path+"/"+messagesDir && messageConst.MatchString(o.Name()):
			x.b.addEdge(owner, "msg:"+o.Name(), edgeUsesMessage, extracted, fc.rel, line)
		case o.Pkg().Path() == x.m.Path+"/"+codesDir && nameConst.MatchString(o.Name()):
			x.b.addEdge(owner, "name:"+o.Name(), edgeUsesName, extracted, fc.rel, line)
		}
		if o.Parent() == o.Pkg().Scope() {
			x.b.addEdge(owner, valueID(kindConst, o.Pkg().Path(), o.Name()), edgeUses, extracted, fc.rel, line)
		}
	case *types.Var:
		if !o.IsField() && o.Parent() == o.Pkg().Scope() {
			x.b.addEdge(owner, valueID(kindVar, o.Pkg().Path(), o.Name()), edgeUses, extracted, fc.rel, line)
		}
	}
}

// implements modüldeki her somut tipten, onun uyduğu modül arayüzlerine `implements` kenarı
// çeker. Yalnızca test dışı tipler taranır (testsiz paket nesneleri tek tip evreni verir).
func (x *goExtractor) implements() {
	type decl struct {
		id    string
		named *types.Named
		file  string
		line  int
	}
	var concrete, ifaces []decl
	paths := make([]string, 0, len(x.m.Base))
	for p := range x.m.Base {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		scope := x.m.Base[p].Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || named.TypeParams().Len() > 0 {
				continue
			}
			pos := x.m.Fset.Position(tn.Pos())
			d := decl{typeID(p, name), named, x.m.Rel(pos.Filename), pos.Line}
			if it, isIface := named.Underlying().(*types.Interface); isIface {
				if it.NumMethods() > 0 {
					ifaces = append(ifaces, d)
				}
			} else {
				concrete = append(concrete, d)
			}
		}
	}
	for _, t := range concrete {
		ptr := types.NewPointer(t.named)
		for _, i := range ifaces {
			it := i.named.Underlying().(*types.Interface)
			if types.Implements(t.named, it) || types.Implements(ptr, it) {
				x.b.addEdge(t.id, i.id, edgeImplements, extracted, t.file, t.line)
			}
		}
	}
}

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
	ast.Inspect(root, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if id := calleeIdent(n.Fun); id != nil {
				callee[id] = true
			}
		case *ast.Ident:
			x.use(fc, owner, n, callee[n])
		}
		return true
	})
}

// calleeIdent çağrılan ifadenin adını verir (parantez ve tip bağımsız değişkenleri atılır).
func calleeIdent(e ast.Expr) *ast.Ident {
	for {
		switch v := e.(type) {
		case *ast.ParenExpr:
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
	obj := fc.u.info.Uses[id]
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
		case o.Pkg().Path() == x.m.path+"/"+messagesDir && messageConst.MatchString(o.Name()):
			x.b.addEdge(owner, "msg:"+o.Name(), edgeUsesMessage, extracted, fc.rel, line)
		case o.Pkg().Path() == x.m.path+"/"+codesDir && nameConst.MatchString(o.Name()):
			x.b.addEdge(owner, "name:"+o.Name(), edgeUsesName, extracted, fc.rel, line)
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
	paths := make([]string, 0, len(x.m.base))
	for p := range x.m.base {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		scope := x.m.base[p].Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || named.TypeParams().Len() > 0 {
				continue
			}
			pos := x.m.fset.Position(tn.Pos())
			d := decl{typeID(p, name), named, x.rel(pos.Filename), pos.Line}
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

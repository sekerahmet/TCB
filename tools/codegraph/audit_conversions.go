package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// Bu dosya dead-methods'un dış arayüz geçişidir (K4): modülün somut bir tipinin değeri modül dışındaki adlı, boş olmayan
// bir arayüz tipine dönüşüyorsa standart kitaplık (ya da başka bir modül) o tipin metotlarını arayüz üzerinden çağırır;
// çağrı kaynakta görünmez. O tipin dışa açık bütün metotları canlı sayılır, yalnız arayüzünkiler değil: math/rand
// Uint64'e rand.Source'ta değil Source64 tip iddiasıyla ulaşıyor (pcgSource.Uint64, Int63, Seed "unreferenced"
// görünüyordu, ml.newSharedRandom onu rand.New'e veriyor). Dönüşüm yerleri, hedef tip types.Info'dan: çağrı argümanı,
// adlı değişkene atama ve var bildirimi, return, bileşik değerin alanı, öğesi, eşlem anahtarı ve değeri, açık
// dönüşüm, kanal gönderimi ve append'in eklenenleri. Boş arayüz (any) ve önceden tanımlı error sayılmaz: any'nin
// metodu yoktur, error'ın metotları (Error, Unwrap ...) dış sözleşme listesindedir; boş tanımlayıcıya verilen değer
// (`var _ rand.Source = T{}`) yalnız derleme denetimidir.

// collectExternalConversions üretim dosyalarında dış arayüze verilen modül tiplerini a.external'a yazar.
func (a *methodAudit) collectExternalConversions() {
	a.external = map[string]bool{}
	forEachFile(a.m, func(u *goload.Unit, f *ast.File, rel string) {
		if isTestFile(rel) {
			return
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				a.convertCall(u, n)
			case *ast.AssignStmt:
				if n.Tok == token.ASSIGN && len(n.Lhs) == len(n.Rhs) {
					for i, lhs := range n.Lhs {
						if id, blank := lhs.(*ast.Ident); !blank || id.Name != "_" {
							a.convert(u, u.Info.TypeOf(lhs), n.Rhs[i])
						}
					}
				}
			case *ast.ValueSpec:
				if n.Type != nil && len(n.Names) == len(n.Values) {
					for i, v := range n.Values {
						if n.Names[i].Name != "_" {
							a.convert(u, u.Info.TypeOf(n.Type), v)
						}
					}
				}
			case *ast.CompositeLit:
				a.convertElements(u, n)
			case *ast.SendStmt:
				if ch, ok := typeUnder(u.Info.TypeOf(n.Chan)).(*types.Chan); ok {
					a.convert(u, ch.Elem(), n.Value)
				}
			case *ast.FuncDecl:
				if fn, ok := u.Info.Defs[n.Name].(*types.Func); ok && n.Body != nil {
					a.convertReturns(u, n.Body, fn.Type().(*types.Signature))
				}
			case *ast.FuncLit:
				if sig, ok := u.Info.TypeOf(n).(*types.Signature); ok {
					a.convertReturns(u, n.Body, sig)
				}
			}
			return true
		})
	})
}

// convertCall çağrının argümanlarını parametre tiplerine, açık dönüşümün değerini hedef tipe, append'in eklenenlerini
// dilimin öğe tipine verir.
func (a *methodAudit) convertCall(u *goload.Unit, c *ast.CallExpr) {
	if tv, ok := u.Info.Types[c.Fun]; ok && tv.IsType() {
		if len(c.Args) == 1 {
			a.convert(u, tv.Type, c.Args[0])
		}
		return
	}
	if id, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
		if b, ok := u.Info.Uses[id].(*types.Builtin); ok {
			if b.Name() == "append" && len(c.Args) > 1 && !c.Ellipsis.IsValid() {
				if s, ok := typeUnder(u.Info.TypeOf(c.Args[0])).(*types.Slice); ok {
					for _, v := range c.Args[1:] {
						a.convert(u, s.Elem(), v)
					}
				}
			}
			return
		}
	}
	sig, ok := typeUnder(u.Info.TypeOf(c.Fun)).(*types.Signature)
	if !ok {
		return
	}
	for i, v := range c.Args {
		a.convert(u, paramType(sig, i, c.Ellipsis.IsValid()), v)
	}
}

// convertElements bileşik değerin öğelerini alanın, öğenin ya da eşlem anahtarının ve değerinin tipine verir.
func (a *methodAudit) convertElements(u *goload.Unit, c *ast.CompositeLit) {
	switch t := typeUnder(u.Info.TypeOf(c)).(type) {
	case *types.Struct:
		for i, e := range c.Elts {
			if kv, ok := e.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok {
					if v, ok := u.Info.Uses[id].(*types.Var); ok && v.IsField() {
						a.convert(u, v.Type(), kv.Value)
					}
				}
			} else if i < t.NumFields() {
				a.convert(u, t.Field(i).Type(), e)
			}
		}
	case *types.Slice:
		a.convertItems(u, c.Elts, nil, t.Elem())
	case *types.Array:
		a.convertItems(u, c.Elts, nil, t.Elem())
	case *types.Map:
		a.convertItems(u, c.Elts, t.Key(), t.Elem())
	}
}

// convertItems dilim, dizi ve eşlem öğelerini öğe tipine (eşlemde anahtarı anahtar tipine) verir.
func (a *methodAudit) convertItems(u *goload.Unit, elts []ast.Expr, key, elem types.Type) {
	for _, e := range elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if key != nil {
				a.convert(u, key, kv.Key)
			}
			e = kv.Value
		}
		a.convert(u, elem, e)
	}
}

// convertReturns bir gövdenin return değerlerini sonuç tiplerine verir; iç işlev değişmezlerinin return'leri onların
// kendi imzasıyla, ayrıca gezilir.
func (a *methodAudit) convertReturns(u *goload.Unit, body *ast.BlockStmt, sig *types.Signature) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(n.Results) == sig.Results().Len() {
				for i, v := range n.Results {
					a.convert(u, sig.Results().At(i).Type(), v)
				}
			}
		}
		return true
	})
}

// convert val'in tipi modülün somut bir tipiyse ve slot modül dışında adlı, boş olmayan, error olmayan bir arayüzse
// tipi dış arayüze verilmiş sayar.
func (a *methodAudit) convert(u *goload.Unit, slot types.Type, val ast.Expr) {
	if slot == nil || val == nil {
		return
	}
	named, ok := types.Unalias(slot).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || a.inModule(named.Obj().Pkg().Path()) {
		return // modülün arayüzü satisfies yolundan, error ve adsız arayüz hiç sayılmaz
	}
	if it, isIface := named.Underlying().(*types.Interface); !isIface || it.Empty() {
		return
	}
	t, ok := types.Unalias(derefType(u.Info.TypeOf(val))).(*types.Named)
	if !ok || t.Obj().Pkg() == nil || !a.inModule(t.Obj().Pkg().Path()) || types.IsInterface(t) {
		return
	}
	a.external[typeKey(t.Origin())] = true
}

// inModule paket yolu modülün içinde mi (grafikteki goExtractor.inModule'ün denetimdeki karşılığı).
func (a *methodAudit) inModule(p string) bool {
	return p == a.m.Path || strings.HasPrefix(p, a.m.Path+"/")
}

// typeUnder t'nin alt tipidir (t nil ise nil).
func typeUnder(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return t.Underlying()
}

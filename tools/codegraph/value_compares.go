package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// Bu dosya metinle anılan sabitler: aynı değer bir yerde adlı sabitle (lexer.TokPercent = "%"), başka yerde metin
// sabitiyle anılıyor: yorumlayıcı `case "%":`, denetleyici `case "+", "-", ...:`. Sabitin `uses` listesi o yerleri
// göstermez; işlecin anlamını değiştiren biri uygulamasını görmez (eksikler listesi, kritik 1). Değerin kimlik olarak kullanıldığı
// üç yer kenar olur: switch `case`'i, `==`/`!=` karşılaştırması ve dize anahtarlı bir haritanın değişmez anahtarı. Değeri
// eşit olan her dışa açık, test dışı, modüldeki dize sabitine `compares_value` [INFERRED] kenarı gider (aynı değerli
// iki sabit varsa ikisine de: hangisi kastedildiği metinden bilinmez). Boş dize sayılmaz.

// textCompare bir dize değişmezinin kimlik olarak kullanıldığı yerdir.
type textCompare struct {
	owner, file string
	line        int
	value       string
	pkg         *types.Package // yerin paketi: sabitin paketine (geçişli) bağımlı olmalı
}

// noteStringConst dışa açık, test dışı paket düzeyi dize sabitini değerine göre kaydeder.
func (x *goExtractor) noteStringConst(fc *fileCtx, id string, c *types.Const) {
	if fc.isTest || !c.Exported() || c.Val().Kind() != constant.String {
		return
	}
	if v := constant.StringVal(c.Val()); v != "" {
		x.strConsts[v] = append(x.strConsts[v], id)
		x.strConstPkg[id] = c.Pkg().Path()
	}
}

// noteTextCompare n bir case, ==/!= ya da dize anahtarlı harita değişmeziyse içindeki dize değişmezlerini kaydeder.
func (x *goExtractor) noteTextCompare(fc *fileCtx, owner string, n ast.Node) {
	var lits []ast.Expr
	switch n := n.(type) {
	case *ast.CaseClause:
		lits = n.List
	case *ast.BinaryExpr:
		if n.Op == token.EQL || n.Op == token.NEQ {
			lits = []ast.Expr{n.X, n.Y}
		}
	case *ast.CompositeLit:
		if stringKeyedMap(fc, n.Type) {
			for _, e := range n.Elts {
				if kv, ok := e.(*ast.KeyValueExpr); ok {
					lits = append(lits, kv.Key)
				}
			}
		}
	}
	for _, e := range lits {
		if v, ok := stringLiteral(fc, e); ok {
			x.textCompares = append(x.textCompares, textCompare{owner: owner, file: fc.rel, line: x.line(e.Pos()), value: v, pkg: fc.u.Pkg})
		}
	}
}

// stringKeyedMap bileşik değişmezin yazılı tipi dize anahtarlı bir harita mı (`map[string]T`, `map[Ad]T`, harita olan adlı
// tip). Grafik ifade tiplerini yüklemez (Info.Types yok): tip adları Info.Uses'tan çözülür; tipi yazılmamış iç değişmez sayılmaz.
func stringKeyedMap(fc *fileCtx, typ ast.Expr) bool {
	named := func(e ast.Expr) types.Type {
		if id := nameIdent(e); id != nil {
			if tn, ok := fc.u.Info.Uses[id].(*types.TypeName); ok {
				return tn.Type()
			}
		}
		return nil
	}
	if mt, ok := typ.(*ast.MapType); ok {
		k := named(mt.Key)
		return k != nil && isStringType(k)
	}
	if t := named(typ); t != nil {
		m, ok := t.Underlying().(*types.Map)
		return ok && isStringType(m.Key())
	}
	return false
}

// stringLiteral e (parantezleri atılmış) boş olmayan bir dize değişmeziyse değeridir; adlı sabit sayılmaz (o `uses`tur).
func stringLiteral(fc *fileCtx, e ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil && v != ""
}

// resolveTextCompares biriken yerleri değeri eşit sabitlere bağlar (bütün paketler okunduktan sonra: sabit başka pakette).
func (x *goExtractor) resolveTextCompares() {
	deps := map[*types.Package]map[string]bool{}
	for _, c := range x.textCompares {
		for _, id := range x.strConsts[c.value] {
			if id != c.owner && dependsOn(deps, c.pkg)[x.strConstPkg[id]] {
				x.b.addEdge(c.owner, id, edgeComparesValue, inferred, c.file, c.line)
			}
		}
	}
}

// hasStringValue n değeri q olan bir dize sabiti mi (`query "%"` → lexer.TokPercent: lookup'ın son ad basamağı); değer
// düğümün açıklamasından (`const lexer.TokPercent untyped string = "%"`) okunur, kısaltılmış değer tutmaz.
func hasStringValue(n *Node, q string) bool {
	if n.Kind != kindConst {
		return false
	}
	_, val, ok := strings.Cut(n.Detail, " = ")
	v, err := strconv.Unquote(val)
	return ok && err == nil && v == q
}

// dependsOn p'nin kendisi ve geçişli olarak içe aktardığı paketlerin yollarıdır (deps'te saklanır). Değeri metinle anan yer
// sabitin paketine bağımlı değilse aynı metin başka bir anlamdadır: reporoot'un `name == "."`'ı lexer.TokDot değildir.
func dependsOn(deps map[*types.Package]map[string]bool, p *types.Package) map[string]bool {
	if d, ok := deps[p]; ok {
		return d
	}
	d := map[string]bool{p.Path(): true, strings.TrimSuffix(p.Path(), "_test"): true}
	deps[p] = d
	for _, imp := range p.Imports() {
		for path := range dependsOn(deps, imp) {
			d[path] = true
		}
	}
	return d
}

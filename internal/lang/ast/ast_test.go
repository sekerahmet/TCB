package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"
)

// parseSource ast.go'yu Go sözdizimi ağacı olarak okur (yapısal denetimler için).
func parseSource(t *testing.T, name string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// structTypes dosyadaki struct tiplerini adlarıyla verir.
func structTypes(f *ast.File) map[string]*ast.StructType {
	out := map[string]*ast.StructType{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range gd.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok {
				if st, ok := ts.Type.(*ast.StructType); ok {
					out[ts.Name.Name] = st
				}
			}
		}
	}
	return out
}

// Düğümler yalnızca veri taşır: ast.go'da serbest fonksiyon yoktur; metotlar yalnızca düğüm
// işaretleyicisi, TokenLiteral ve Line'dır (çözümleme, denetim, çalıştırma başka paketlerdedir).
func TestNodesCarryOnlyData(t *testing.T) {
	allowed := map[string]bool{"statementNode": true, "expressionNode": true, "TokenLiteral": true, "Line": true}
	for _, d := range parseSource(t, "ast.go").Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Recv == nil {
			t.Errorf("ast.go: %s serbest fonksiyon; düğümler yalnızca veri taşır", fd.Name.Name)
			continue
		}
		if !allowed[fd.Name.Name] {
			t.Errorf("ast.go: %s metodu düğüme iş ekliyor; çözümleme/denetim/çalıştırma başka pakette olmalı", fd.Name.Name)
		}
	}
}

// Her düğüm kendi sözcük birimini taşır (hata mesajı ve hata ayıklayıcı satırı oradan alınır):
// ifade ve deyim düğümlerinin ilk alanı lexer.Token tipindeki Token'dır. Bütün alanlar dışa
// açıktır; Dump yalnızca dışa açık alanları yazdığı için gizli bir alan karşılaştırmadan düşerdi.
func TestEveryNodeKnowsItsToken(t *testing.T) {
	f := parseSource(t, "ast.go")
	nodes := map[string]bool{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil && (fd.Name.Name == "statementNode" || fd.Name.Name == "expressionNode") {
			nodes[receiverName(fd)] = true
		}
	}
	if len(nodes) == 0 {
		t.Fatal("düğüm bulunamadı")
	}
	var names []string
	for n := range nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	structs := structTypes(f)
	for _, name := range names {
		st, ok := structs[name]
		if !ok || len(st.Fields.List) == 0 {
			t.Errorf("%s: struct bulunamadı", name)
			continue
		}
		first := st.Fields.List[0]
		sel, isSel := first.Type.(*ast.SelectorExpr)
		if len(first.Names) != 1 || first.Names[0].Name != "Token" || !isSel || sel.Sel.Name != "Token" {
			t.Errorf("%s: ilk alan `Token lexer.Token` olmalı (düğüm yerini bilir)", name)
		}
	}
	for name, st := range structs {
		for _, field := range st.Fields.List {
			for _, n := range field.Names {
				if !n.IsExported() {
					t.Errorf("%s.%s: gizli alan Dump'ta görünmez; alanlar dışa açık olmalı", name, n.Name)
				}
			}
		}
	}
}

// receiverName metodun alıcı tipinin adıdır (*T ya da T).
func receiverName(fd *ast.FuncDecl) string {
	t := fd.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	return t.(*ast.Ident).Name
}

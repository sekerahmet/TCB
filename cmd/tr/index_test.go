package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (17.9): alt komut sınıfının (sub) her kodu 'tr index'in alt komutudur; başka komutun alt
// komutu yoktur. H0037'nin önerisi (lexer.classedSpelling) bu yüzden 'tr index <alt komut>' yazar.
// runIndexQuery'nin her alt komutu karşılaması bunun kanıtıdır: başka bir komuta alt komut eklenirse
// ya da bir alt komut karşılanmazsa test düşer.
func TestEverySubcommandIsAnIndexSubcommand(t *testing.T) {
	handled := indexQueryCases(t)
	for _, k := range codes.Codes() {
		if k.Class() != codes.ClassSubcommand {
			continue
		}
		name := goConstName(k.String())
		if !handled[name] {
			t.Errorf("%s (%s) tr index'in alt komutu olarak runIndexQuery'de karşılanmıyor", k, name)
		}
		delete(handled, name)
	}
	for name := range handled {
		t.Errorf("runIndexQuery'deki codes.%s alt komut sınıfında değil", name)
	}
}

// indexQueryCases index.go'daki runIndexQuery'nin case etiketlerindeki codes.SubX adlarıdır.
func indexQueryCases(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(reporoot.Root(), "cmd", "tr", "index.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runIndexQuery" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			if c, ok := n.(*ast.CaseClause); ok {
				for _, e := range c.List {
					if s, ok := e.(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "Sub") {
						out[s.Sel.Name] = true
					}
				}
			}
			return true
		})
	}
	if len(out) == 0 {
		t.Fatal("runIndexQuery'de alt komut case'i bulunamadı")
	}
	return out
}

// goConstName kimliğin Go sabit adıdır (codes_generated.go'daki gibi): TR_sub_tested_by → SubTestedBy.
func goConstName(identity string) string {
	parts := strings.Split(strings.TrimPrefix(identity, codes.IdentityPrefix), "_")
	var b strings.Builder
	for _, p := range parts {
		if p != "" {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

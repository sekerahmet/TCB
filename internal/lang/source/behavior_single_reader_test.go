package source

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Dosyalar tek yerden okunur: kodlu biçim başlığının kuralları (başlık satırı, gövdeyi kodlu
// okuyan sözcük çözümleyici kurucusu) yalnızca lexer ve source paketlerindedir; öbür paketler
// başlığı kendileri tanımaz, source.Split ve source.Prepare'i kullanır. Test, test dışı Go
// kodunda başlık metnini, lexer.CodedHeader, lexer.NewBody ve source.Header kullanımlarını arar.
func TestHeaderRulesLiveOnlyInSourceAndLexer(t *testing.T) {
	root := reporoot.Root()
	checked := 0
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := reporoot.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/lang/source/") || strings.HasPrefix(rel, "internal/lang/lexer/") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			checked++
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.BasicLit:
					if n.Kind == token.STRING && strings.Contains(n.Value, "//! tr kodlu") {
						t.Errorf("%s: kodlu biçim başlığı metni yazılmış; başlık kuralı yalnızca source ve lexer paketlerindedir", rel)
					}
				case *ast.SelectorExpr:
					pkg, ok := n.X.(*ast.Ident)
					if !ok {
						return true
					}
					switch pkg.Name + "." + n.Sel.Name {
					case "lexer.CodedHeader", "lexer.NewBody", "source.Header":
						t.Errorf("%s: %s.%s kullanılıyor; dosyayı source.Split / source.Prepare ile okuyun", rel, pkg.Name, n.Sel.Name)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 50 {
		t.Fatalf("çok az dosya tarandı: %d", checked)
	}
}

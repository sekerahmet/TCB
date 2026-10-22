package hint

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

// Tek kaynak: ad ipucu iletileri (H1101-H1106: ASCII yazım, en yakın ad, tanımsız ad)
// yalnızca bu pakette kurulur; öbür dil paketleri hint işlevlerini çağırır. outsideUse, bu
// kuralın istisnalarıdır: dosya → ileti kodu → kullanım sayısı. Liste boştur ve yalnızca
// küçülebilir: yeni kayıt eklenmez, ipucu hint'e eklenir.
var outsideUse = map[string]map[string]int{}

func TestHintMessagesBuiltOnlyHere(t *testing.T) {
	root := reporoot.Root()
	found := map[string]map[string]int{}
	err := filepath.WalkDir(filepath.Join(root, "internal", "lang"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/lang/hint/") || strings.HasPrefix(rel, "internal/lang/messages/") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "messages" && isHintCode(sel.Sel.Name) {
				if found[rel] == nil {
					found[rel] = map[string]int{}
				}
				found[rel][sel.Sel.Name]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for file, codes := range found {
		for code, n := range codes {
			switch allowed := outsideUse[file][code]; {
			case n > allowed:
				t.Errorf("%s: %s hint paketi dışında kuruluyor (%d kez, izinli %d); ipucunu hint'e ekleyin", file, code, n, allowed)
			case n < allowed:
				t.Errorf("%s: %s için izin listesi eskimiş (%d kullanım kaldı, listede %d); kaydı küçültün", file, code, n, allowed)
			}
		}
	}
	for file, codes := range outsideUse {
		for code := range codes {
			if found[file][code] == 0 {
				t.Errorf("%s: %s artık kullanılmıyor; kaydı silin", file, code)
			}
		}
	}
}

// isHintCode ad ipucu iletisi kodu mu (H1100-H1106)?
func isHintCode(name string) bool {
	return len(name) == 5 && strings.HasPrefix(name, "H110") && name[4] >= '0' && name[4] <= '6'
}

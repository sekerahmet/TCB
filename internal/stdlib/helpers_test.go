package stdlib

// Motor testleri dilin tamamıyla, koddaki kesme_noktası deyiminin açtığı hata ayıklayıcı
// dahil çalışır (cmd/tr ve tr test gibi). Hata ayıklayıcı araç katmanındadır; yalnızca
// testlerde içe aktarılır.
import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// productionFiles stdlib ağacındaki test olmayan Go dosyalarını ayrıştırır (davranış testleri
// kaynağı go/parser ile tarar). Anahtar: depo köküne göre dosya yolu.
func productionFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	root := reporoot.Root()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	err := reporoot.WalkDir(reporoot.Path("internal", "stdlib"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

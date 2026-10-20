package stdlib

import (
	"go/ast"
	"path"
	"sort"
	"testing"
)

// Program bitince çağrılan Close, kaynak tutan her motorun kapatma işlevini çağırmalıdır:
// yeni bir motor CloseAll (ya da Close) yazıp Close'a eklemeyi unutursa açık okuyucular,
// günlükler ve geçici klasörler program bitince temizlenmez. Kural: stdlib altındaki bir
// paket dışa açık, alıcısız CloseAll ya da Close işlevi tanımlıyorsa engines.go'daki Close
// onu çağırır.
func TestCloseCallsEveryPackageCloser(t *testing.T) {
	closers := map[string]string{} // paket adı → işlev
	var closeBody *ast.BlockStmt
	for file, f := range productionFiles(t) {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || (fn.Name.Name != "CloseAll" && fn.Name.Name != "Close") {
				continue
			}
			if file == "internal/stdlib/engines.go" {
				closeBody = fn.Body
				continue
			}
			closers[f.Name.Name+"."+fn.Name.Name] = path.Dir(file)
		}
	}
	if closeBody == nil {
		t.Fatal("engines.go'da Close yok")
	}
	called := map[string]bool{}
	ast.Inspect(closeBody, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok {
					called[x.Name+"."+sel.Sel.Name] = true
				}
			}
		}
		return true
	})
	var missing []string
	for name := range closers {
		if !called[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("%s (%s) engines.go'daki Close'dan çağrılmıyor; program bitince kaynakları kapanmaz", name, closers[name])
	}
	if len(closers) < 4 {
		t.Errorf("kapatma işlevleri taranamadı: %v", closers)
	}
}

// Close birden çok kez çağrılabilir (cmd/tr hem defer ile hem hata yolunda çağırır).
func TestCloseCallableRepeatedly(t *testing.T) {
	for range 3 {
		Close()
	}
}

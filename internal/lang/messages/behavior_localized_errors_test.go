package messages

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Bir ileti ve içindeki her şey iletinin dilindedir; yakalanan hatada ileti yakalayan dosyanın
// dilinde kurulur (ErrorIn). Katalog bir hatayı dile göre ancak kendi türlerinden (Error,
// PosError) ya da Localized ise yazabilir; başka bir hata türü Error() metnine, yani TR_DIL'e
// düşerdi. Kural: depoda Error() string uygulayan her tür MessageText(lang string) string de
// uygular. Yeni bir hata türü bu testle kapsama girer.
//
// localizedExempt, iletinin içine hiç girmeyen türlerdir (tür → gerekçe).
var localizedExempt = map[string]string{
	"internal/lang/messages.Error":      "katalogun kendi türü; codedText doğrudan yazar",
	"internal/lang/messages.PosError":   "katalogun kendi türü; codedText doğrudan yazar",
	"internal/lang/interp.ExitRequest":  "programın bitiş isteği; metni programın kendisinindir, yakalanmaz",
	"internal/stdlib/process.notNumber": "boş metinli iç işaret; kullanıcıya ulaşmaz",
}

func TestEveryErrorTypeIsLocalized(t *testing.T) {
	root := reporoot.Root()
	errorTypes, localized := map[string]string{}, map[string]bool{}
	fset := token.NewFileSet()
	for _, top := range []string{"cmd", "internal"} {
		err := reporoot.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := goparser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
					continue
				}
				if r, ok := fd.Type.Results.List[0].Type.(*ast.Ident); !ok || r.Name != "string" {
					continue
				}
				typ := filepath.ToSlash(rel) + "." + receiverName(fd.Recv.List[0].Type)
				params := fd.Type.Params.List
				switch {
				case fd.Name.Name == "Error" && len(params) == 0:
					errorTypes[typ] = fset.Position(fd.Pos()).String()
				case fd.Name.Name == "MessageText" && len(params) == 1:
					localized[typ] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var missing []string
	for typ, pos := range errorTypes {
		if !localized[typ] && localizedExempt[typ] == "" {
			missing = append(missing, pos+": "+typ)
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("%s: hata türü Error() uyguluyor ama MessageText(lang) uygulamıyor; yakalanan hatada TR_DIL'de kalır", m)
	}
	for typ := range localizedExempt {
		if errorTypes[typ] == "" {
			t.Errorf("%s: istisna listesinde ama böyle bir hata türü yok; kaydı silin", typ)
		}
	}
}

// receiverName alıcının tür adıdır (*T ya da T).
func receiverName(e ast.Expr) string {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

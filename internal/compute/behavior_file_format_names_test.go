package compute_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Diske yazılan ya da diskten okunan JSON biçimleri (meta.json, tokenizer.json) Go alan
// adlarına bağlı değildir: encoding/json kullanan dosyalarda, en az bir alanı json etiketli
// (yani bir JSON şeması olan) her yapının bütün dışa açık alanları açık bir `json:"..."`
// adı taşır; alan adı değişince kullanıcının dosyası okunamaz olmaz.
func TestJSONFieldsHaveExplicitNames(t *testing.T) {
	checked := 0
	for _, dir := range computeDirs(t) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			path := filepath.Join(dir, n)
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if !importsJSON(file) {
				continue
			}
			ast.Inspect(file, func(node ast.Node) bool {
				st, ok := node.(*ast.StructType)
				if !ok {
					return true
				}
				if !anyJSONTag(st) {
					return true
				}
				for _, f := range st.Fields.List {
					for _, name := range fieldNames(f) {
						if !ast.IsExported(name) {
							continue
						}
						checked++
						if !testkit.HasJSONName(f) {
							t.Errorf("%s: dışa açık alan %s için json etiketi yok; dosya biçimi Go adına bağlı kalır", path, name)
						}
					}
				}
				return true
			})
		}
	}
	if checked < 30 {
		t.Fatalf("yalnızca %d JSON alanı denetlendi; tarama bozulmuş olabilir", checked)
	}
}

func anyJSONTag(st *ast.StructType) bool {
	for _, f := range st.Fields.List {
		if f.Tag != nil && strings.Contains(f.Tag.Value, `json:"`) {
			return true
		}
	}
	return false
}

func importsJSON(f *ast.File) bool {
	for _, imp := range f.Imports {
		if imp.Path.Value == `"encoding/json"` {
			return true
		}
	}
	return false
}

// fieldNames bir alan bildiriminin adlarıdır; gömülü alanda tip adı.
func fieldNames(f *ast.Field) []string {
	if len(f.Names) == 0 {
		return []string{recvName(f.Type)}
	}
	var names []string
	for _, n := range f.Names {
		names = append(names, n.Name)
	}
	return names
}

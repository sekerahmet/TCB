package debugger

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Düzenleyiciye giden ve ondan gelen JSON alan adları eklentiyle sözleşmedir: bir Go alan
// adının değişmesi protokolü sessizce bozmasın diye, en az bir alanı json etiketli her
// yapının bütün dışa açık alanları açık bir json adı taşır.
func TestDAPWireFieldsHaveExplicitNames(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range files {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok || !anyJSONTag(st) {
				return true
			}
			for _, f := range st.Fields.List {
				for _, id := range f.Names {
					if !id.IsExported() {
						continue
					}
					checked++
					if !hasJSONName(f) {
						t.Errorf("%s: dışa açık alan %s için json etiketi yok; protokol alan adı Go adına bağlı kalır", name, id.Name)
					}
				}
			}
			return true
		})
	}
	if checked < 20 {
		t.Fatalf("yalnızca %d protokol alanı denetlendi; tarama bozulmuş olabilir", checked)
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

// hasJSONName alanın boş olmayan bir json adı ya da "-" etiketi taşıyıp taşımadığını söyler.
func hasJSONName(f *ast.Field) bool {
	if f.Tag == nil {
		return false
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return false
	}
	tag, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return false
	}
	name, _, _ := strings.Cut(tag, ",")
	return name != "" || tag == "-"
}

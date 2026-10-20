package profiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// funcsOf bir dosyanın üst düzey işlevlerini ve gövdesinde sync/atomic çağıranları verir.
func funcsOf(t *testing.T, path string) (names []string, atomic map[string]bool) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	atomic = map[string]bool{}
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		names = append(names, fd.Name.Name)
		ast.Inspect(fd, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "atomic" {
					atomic[fd.Name.Name] = true
				}
			}
			return true
		})
	}
	sort.Strings(names)
	return names, atomic
}

// Örnekleyicinin paylaşılan değerleri yarış dedektörüyle derlenen sürümde atomik okunup
// yazılır: yarış denetimsiz (pos_plain.go) ve denetimli (pos_race.go) dosyaların işlev kümesi
// aynıdır ve denetimli dosyanın her depolama/okuma işlevi sync/atomic kullanır.
func TestSamplerSharedValuesAtomicUnderRace(t *testing.T) {
	plain, _ := funcsOf(t, "pos_plain.go")
	race, atomic := funcsOf(t, "pos_race.go")
	if strings.Join(plain, ",") != strings.Join(race, ",") {
		t.Fatalf("pos_plain.go ve pos_race.go aynı işlevleri tanımlamalı:\n%v\n%v", plain, race)
	}
	for _, name := range race {
		if (strings.HasPrefix(name, "store") || strings.HasPrefix(name, "load")) && !atomic[name] {
			t.Errorf("pos_race.go: %s sync/atomic kullanmıyor; yarış denetimli derlemede paylaşılan değer atomik olmalı", name)
		}
	}
}

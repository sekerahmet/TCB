package goload

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"testing"
)

// writeModule küçük bir Go modülü yazar: b, a'yı içe aktarır; a'nın iç testi, b'nin dış testi var.
func writeModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.com/m\n\ngo 1.21\n",
		"a/a.go":      "package a\n\n// Twice iki katını verir.\nfunc Twice(n int) int { return n * 2 }\n\ntype T struct{ N int }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc helper() int { return Twice(2) }\n\nfunc TestTwice(t *testing.T) { _ = helper() }\n",
		"b/b.go":      "package b\n\nimport \"example.com/m/a\"\n\nfunc Four() int { return a.Twice(2) }\n\nfunc Field() int { return a.T{}.N }\n",
		"b/b_test.go": "package b_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/b\"\n)\n\nfunc TestFour(t *testing.T) { _ = b.Four() }\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Her paket testleriyle bir ya da iki birim olur; paketler içe aktarma sırasıyla denetlenir; seçenekler
// Info ve ayrıştırma kipini belirler.
func TestLoadChecksPackagesAndTests(t *testing.T) {
	dir := writeModule(t)
	m, err := Load(dir, Options{Comments: true, Selections: true})
	if err != nil {
		t.Fatal(err)
	}
	if m.Path != "example.com/m" || m.Base["example.com/m/a"] == nil || m.Base["example.com/m/b"] == nil {
		t.Fatalf("module %q, packages %v", m.Path, m.Base)
	}
	var paths []string
	for _, u := range m.Units {
		paths = append(paths, u.PkgPath)
	}
	want := []string{"example.com/m/a", "example.com/m/b", "example.com/m/b_test"}
	if len(paths) != len(want) {
		t.Fatalf("units %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("unit %d = %s, want %s", i, paths[i], want[i])
		}
	}
	doc := func(m *Module) *ast.CommentGroup { return m.Units[0].Files[0].Decls[0].(*ast.FuncDecl).Doc }
	if m.Units[0].Info.Selections == nil || len(m.Units[0].Files) != 2 {
		t.Error("the in-package unit has both the package and its test file, and the selections map")
	}
	if doc(m) == nil {
		t.Error("Comments keeps doc comments")
	}
	plain, err := Load(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Units[0].Info.Selections != nil || doc(plain) != nil {
		t.Error("the default options read neither comments nor selections")
	}
}

func TestLoadRejectsAFolderWithoutGoPackages(t *testing.T) {
	if _, err := Load(t.TempDir(), Options{}); err == nil {
		t.Error("an empty folder is an error")
	}
}

func TestOrderPutsImportsFirst(t *testing.T) {
	a := &Listed{ImportPath: "m/a"}
	b := &Listed{ImportPath: "m/b", Imports: []string{"m/a", "fmt"}}
	c := &Listed{ImportPath: "m/c", Imports: []string{"m/b"}}
	got := order([]*Listed{c, b, a})
	if len(got) != 3 || got[0] != a || got[1] != b || got[2] != c {
		t.Errorf("order: %v %v %v", got[0].ImportPath, got[1].ImportPath, got[2].ImportPath)
	}
}

func TestRelAndRecvName(t *testing.T) {
	dir := writeModule(t)
	m, err := Load(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Rel(filepath.Join(dir, "a", "a.go")); got != "a/a.go" {
		t.Errorf("Rel = %q", got)
	}
	named := m.Base["example.com/m/a"].Scope().Lookup("T").Type()
	if RecvName(named) != "T" || RecvName(types.NewPointer(named)) != "T" || RecvName(types.Typ[types.Int]) != "" {
		t.Error("RecvName: a named type, a pointer to it, and an unnamed type")
	}
}

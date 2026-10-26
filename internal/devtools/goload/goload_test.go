package goload

import (
	"go/ast"
	"go/parser"
	"go/types"
	"os"
	"os/exec"
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

// Types seçeneği ifade tiplerini ve sabit değerleri doldurur; varsayılan doldurmaz.
func TestLoadTypesOption(t *testing.T) {
	dir := writeModule(t)
	with, err := Load(dir, Options{Types: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(with.Units[0].Info.Types) == 0 {
		t.Error("Types: true fills Info.Types")
	}
	without, err := Load(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if without.Units[0].Info.Types != nil {
		t.Error("the default leaves Info.Types nil")
	}
}

// Kural: RecvExprName yazılmış alıcı tipinin adını verir; gösterici, parantez ve tür parametreleri atılır, ad olmayan "?" olur.
func TestRecvExprNameStripsPointersParensAndTypeArguments(t *testing.T) {
	for src, want := range map[string]string{
		"T": "T", "*T": "T", "(*T)": "T", "*T[int]": "T", "T[K, V]": "T", "pkg.T": "?", "[]T": "?",
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := RecvExprName(e); got != want {
			t.Errorf("RecvExprName(%s) = %q, want %q", src, got, want)
		}
	}
}

// Kural: SkipIgnored ile git'in yok saydığı klasördeki paket (.gitignore: scratch/) yüklenmez; henüz eklenmemiş ama
// yok sayılmayan paket yüklenir. Seçeneksiz yükleme ikisini de görür (go list'in kendisi).
func TestSkipIgnoredLeavesOutPackagesGitIgnores(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	dir := writeModule(t)
	for name, body := range map[string]string{
		".gitignore":     "scratch/\n",
		"scratch/c/c.go": "package c\n\nfunc C() int { return 3 }\n",
		"yeni/yeni.go":   "package yeni\n\nfunc Y() int { return 4 }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "go.mod", "a", "b", ".gitignore"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for _, skip := range []bool{true, false} {
		m, err := Load(dir, Options{SkipIgnored: skip})
		if err != nil {
			t.Fatal(err)
		}
		if got := m.Base["example.com/m/scratch/c"] != nil; got == skip {
			t.Errorf("SkipIgnored=%v: the ignored package loaded = %v", skip, got)
		}
		if m.Base["example.com/m/yeni"] == nil || m.Base["example.com/m/a"] == nil {
			t.Errorf("SkipIgnored=%v: a tracked or a new, not ignored package is missing: %v", skip, m.Base)
		}
	}
}

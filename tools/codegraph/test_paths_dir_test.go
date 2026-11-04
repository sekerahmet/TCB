package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

var perDirDocFiles = map[string]string{
	"README.md":                  "# root\n",
	"cmd/tool/sub/README.md":     "# sub\n",
	"cmd/tool/sub/sub.go":        "package sub\n",
	"cmd/tool/sub/sub_test.go":   "package sub\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {}\n",
	"internal/core/README.md":    "# core\n",
	"internal/core/core.go":      "package core\n",
	"internal/core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestCore(t *testing.T) {}\n",
	"internal/notest/README.md":  "# notest\n",
	"internal/notest/notest.go":  "package notest\n",
	"tools/x/README.md":          "# x\n",
	"tools/x/x.go":               "package x\n",
	"tools/x/x_test.go":          "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
	"internal/reporoot/root.go": `package reporoot

import "io/fs"

func Root() string                                         { return "" }
func Path(parts ...string) string                          { return "" }
func Glob(pat string) ([]string, error)                    { return nil, nil }
func WalkDir(root string, fn fs.WalkDirFunc) error         { return nil }
`,
	"check/layout_test.go": `package check

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/audit/internal/reporoot"
)

var ruleRoots = []string{"cmd/", "internal/"}

type item struct {
	path string
	test bool
}

func allGoFiles(t *testing.T) []item {
	var out []item
	_ = reporoot.WalkDir(reporoot.Root(), func(p string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		out = append(out, item{path: p, test: strings.HasSuffix(d.Name(), "_test.go")})
		return nil
	})
	return out
}

func inScope(p string) bool {
	for _, r := range ruleRoots {
		if strings.HasPrefix(p, r) {
			return true
		}
	}
	return false
}

func pkgDirs(t *testing.T) []string {
	var dirs []string
	for _, d := range allGoFiles(t) {
		if d.test && inScope(d.path) {
			dirs = append(dirs, filepath.ToSlash(filepath.Dir(d.path)))
		}
	}
	return dirs
}

func readPkgReadme(t *testing.T, dir string) {
	_, _ = os.ReadFile(filepath.Join(reporoot.Root(), filepath.FromSlash(dir), "README.md"))
}

func TestScopedPkgReadmes(t *testing.T) {
	for _, dir := range pkgDirs(t) {
		readPkgReadme(t, dir)
	}
}

func TestPerDirPath(t *testing.T) {
	var sub string
	_, _ = os.ReadFile(reporoot.Path(sub, "README.md"))
}

func TestGlobPkgReadmes(t *testing.T) {
	_, _ = reporoot.Glob(reporoot.Path("**/README.md"))
}

func TestTempDirJoinIgnored(t *testing.T) {
	var sub string
	_, _ = os.ReadFile(filepath.Join(t.TempDir(), sub, "README.md"))
}
`,
}

// Kural: bir testin paket ya da alt klasör başına okuduğu belge (`filepath.Join(reporoot.Root(), dir, "<belge>")`,
// `reporoot.Path(dir, "<belge>")` ya da `**/<belge>` kalıbı) kapsamındaki belgelerin tests-for çıktısında görünür;
// önek ya da test dosyası süzgeciyle daraltılan klasör listeleri kapsam dışına taşmaz; geçici klasör yolu bağlanmaz.
func TestPerPackageDocumentReadsLinkToMatchingPackageDocs(t *testing.T) {
	root := auditModule(t, perDirDocFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const pkg = "go:func:example.com/audit/check."
	mustEdge(t, g, pkg+"TestScopedPkgReadmes", "doc:cmd/tool/sub/README.md", edgeTests, inferred)
	mustEdge(t, g, pkg+"TestScopedPkgReadmes", "doc:internal/core/README.md", edgeTests, inferred)
	for _, outOfScope := range []string{"doc:README.md", "doc:tools/x/README.md", "doc:internal/notest/README.md"} {
		if e := edge(g, pkg+"TestScopedPkgReadmes", outOfScope, edgeTests); e != nil {
			t.Errorf("scoped package-README reader must not link %s: %+v", outOfScope, e)
		}
	}
	mustEdge(t, g, pkg+"TestPerDirPath", "doc:cmd/tool/sub/README.md", edgeTests, inferred)
	mustEdge(t, g, pkg+"TestPerDirPath", "doc:tools/x/README.md", edgeTests, inferred)
	if e := edge(g, pkg+"TestPerDirPath", "doc:README.md", edgeTests); e != nil {
		t.Errorf("reporoot.Path(sub, README.md) must not link root README.md: %+v", e)
	}
	mustEdge(t, g, pkg+"TestGlobPkgReadmes", "doc:cmd/tool/sub/README.md", edgeTests, inferred)
	mustEdge(t, g, pkg+"TestGlobPkgReadmes", "doc:README.md", edgeTests, inferred)
	for _, e := range g.Edges {
		if e.Kind == edgeTests && e.From == pkg+"TestTempDirJoinIgnored" && strings.HasPrefix(e.To, "doc:") {
			t.Errorf("t.TempDir() join must not link repository docs: %+v", e)
		}
	}

	ix := newIndex(g)
	ix.root = root
	var out, errb strings.Builder
	n, code := ix.resolve("cmd/tool/sub/README.md", &queryOpts{stdout: &out, stderr: &errb})
	if code != exitOK {
		t.Fatalf("resolve cmd/tool/sub/README.md: %d %s", code, errb.String())
	}
	ix.printTestsFor(&out, n, 0)
	if !strings.Contains(out.String(), "check.TestScopedPkgReadmes") || strings.Contains(out.String(), "no tests read it") {
		t.Errorf("tests-for cmd/tool/sub/README.md must list check.TestScopedPkgReadmes:\n%s", out.String())
	}
}

func TestRepoPackageReadmeListsTestFileMatchesSource(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	answer := func(p string) string {
		t.Helper()
		var out, errb strings.Builder
		n, code := ix.resolve(p, &queryOpts{stdout: &out, stderr: &errb})
		if code != exitOK {
			t.Fatalf("resolve %s: %d %s", p, code, errb.String())
		}
		ix.printTestsFor(&out, n, 0)
		return out.String()
	}
	// Yol parçadan kurulur: düz yazılsa codegraph bu testi README'nin okuyucusu sayar ve aşağıdaki "no tests read it"
	// koşulu kendi kendine geçer (DENETİM istek 88 Ş1).
	e2eOut := answer(strings.Join([]string{"cmd", codes.ProgramName, "e2e", "README.md"}, "/"))
	if !strings.Contains(e2eOut, "internal_test.TestTestFileMatchesSource") || strings.Contains(e2eOut, "no tests read it") {
		t.Errorf("tests-for package README must list internal_test.TestTestFileMatchesSource:\n%s", e2eOut)
	}
	for _, e := range g.Edges {
		if e.Kind == edgeTests && strings.HasSuffix(e.From, ".TestTestFileMatchesSource") && strings.HasPrefix(e.To, "doc:tools/") {
			t.Errorf("TestTestFileMatchesSource is outside tools tree and must not link %s", e.To)
		}
	}
}

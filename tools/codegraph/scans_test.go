package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

var scanFiles = map[string]string{
	"internal/reporoot/root.go": `package reporoot

import "io/fs"

func Root() string { return "" }

func Path(parts ...string) string { return "" }

func WalkDir(root string, fn fs.WalkDirFunc) error { return nil }
`,
	"app/app.go":      "package app\n",
	"app/app_test.go": "package app\n",
	"app/x.tr":        "değişken x = 1\n",
	"lib/a.go":        "package lib\n",
	"docs/d.md":       "# D\n",
	"checks/checks_test.go": `package checks

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"example.com/audit/internal/reporoot"
)

var roots = []string{"app", "lib"}

func TestGoOnly(t *testing.T) {
	root := reporoot.Root()
	_ = reporoot.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		return nil
	})
}

func TestRoots(t *testing.T) {
	for _, k := range roots {
		_ = reporoot.WalkDir(filepath.Join(reporoot.Root(), k), func(path string, d fs.DirEntry, err error) error {
			if filepath.Ext(path) != ".go" {
				return nil
			}
			return nil
		})
	}
}

func TestByHelper(t *testing.T) { _ = files(t, reporoot.Root(), ".tr") }

func files(t *testing.T, root, ext string) []string {
	var out []string
	_ = reporoot.WalkDir(filepath.Join(root, "app"), func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(path, ext) {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func TestTracked(t *testing.T) {
	out, _ := exec.Command("git", "-C", reporoot.Root(), "ls-files").Output()
	for _, f := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasSuffix(f, ".md"):
			_ = f
		}
	}
}

func TestPackagesOnly(t *testing.T) {
	_ = reporoot.WalkDir(reporoot.Root(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		return nil
	})
}

func TestCopy(t *testing.T) {
	_ = reporoot.WalkDir(t.TempDir(), func(path string, d fs.DirEntry, err error) error { return nil })
	_ = os.Remove("x")
}
`,
}

// Kural: depo genelini tarayan bir test (reporoot.WalkDir, `git ls-files`) taradığı kalıbın her dosyasının testidir;
// tests-for kalıba uyan dosyada onu doğrudan testlerden ayrı, kalıp başına tek satırla verir. Taban depo kökünden
// çözülür (reporoot.Root(), Join(kök, parça), paket düzeyi liste, bir kez atanan yerel, çağıranın verdiği parametre);
// süzgeç atlama koşulundan (`!HasSuffix(".go") || HasSuffix("_test.go")` → .go dahil, _test.go hariç), `Ext != ".go"`
// koşulundan, etiketsiz switch'ten ve çağıranın verdiği ekten okunur. Yalnızca klasör gezen (`!d.IsDir()` atlanır) ve
// geçici kopyayı gezen tarama sayılmaz (TCB 2026-10-25: internal.TestNoTurkishLiterals tam denetimde bir dosyada kaldı,
// o dosyanın tests-for cevabı bu testi göstermemişti).
func TestRepositoryScansApplyToEveryFileOfTheirPattern(t *testing.T) {
	g, err := buildGraph(auditModule(t, scanFiles))
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	const checks = "checks."
	want := map[string][]string{ // dosya -> onu tarayan testler
		"app/app.go":      {"TestGoOnly", "TestRoots"},
		"lib/a.go":        {"TestGoOnly", "TestRoots"},
		"app/app_test.go": {"TestRoots"},
		"app/x.tr":        {"TestByHelper"},
		"docs/d.md":       {"TestTracked"},
	}
	for file, tests := range want {
		got := map[string]bool{}
		for _, h := range ix.scansCovering(file) {
			for _, tn := range h.tests {
				got[strings.TrimPrefix(tn.Name, checks)] = true
			}
		}
		for _, name := range tests {
			if !got[name] {
				t.Errorf("%s: %s scans it, not listed (got %v)", file, name, got)
			}
		}
		if len(got) != len(tests) {
			t.Errorf("%s: want exactly %v, got %v", file, tests, got)
		}
	}
	for _, e := range g.Edges {
		if e.Kind == edgeScans && (strings.HasSuffix(e.From, ".TestPackagesOnly") || strings.HasSuffix(e.From, ".TestCopy")) {
			t.Errorf("a folder-only walk or a temporary copy is not a file scan: %+v", e)
		}
	}
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["go:file:lib/a.go"], 12)
	if !strings.Contains(out.String(), "repository-wide checks that scan it") ||
		!strings.Contains(out.String(), "*.go under the repository, not *_test.go") || !strings.Contains(out.String(), "checks.TestGoOnly") {
		t.Errorf("tests-for lib/a.go lacks the scan group:\n%s", out.String())
	}
}

// Kural (sınıf): depoda reporoot.WalkDir ya da `git ls-files` ile depo kökünden dosya listeleyen her test çağrısı bir
// tarama düğümüdür ve kalıbı en az bir izlenen dosyayı kapsar. Kökten olduğu metinden anlaşılan (reporoot.Root(),
// reporoot.Path, bunlarla atanmış yerel) ama tanınmayan çağrı ya da hiçbir dosyaya uymayan kalıp testi düşürür; yalnızca
// klasör gezen çağrı (`!d.IsDir()` atlanır) dosya taraması değildir. İstisna listesi yoktur.
func TestEveryRepositoryScanIsRecognized(t *testing.T) {
	root, _, g := repoGraph(t)
	tracked, ok := codeindex.GitTracked(root)
	if !ok {
		t.Skip("needs a git checkout")
	}
	scans := map[string]*Node{}
	for i := range g.Nodes {
		if n := &g.Nodes[i]; n.Kind == kindScan {
			scans[n.File+":"+strconv.Itoa(n.Line)] = n
			covered := false
			for _, f := range tracked {
				if decodeScope(n.Detail).covers(f) {
					covered = true
					break
				}
			}
			if !covered {
				t.Errorf("scan %s (%s) covers no tracked file", n.ID, n.Name)
			}
		}
	}
	for _, f := range tracked {
		if !strings.HasSuffix(f, "_test.go") || strings.Contains(f, "/testdata/") {
			continue
		}
		for _, site := range rootedListings(t, filepath.Join(root, filepath.FromSlash(f))) {
			if scans[f+":"+strconv.Itoa(site)] == nil {
				t.Errorf("%s:%d lists repository files from the root but is no scan node (tests-for misses its test)", f, site)
			}
		}
	}
}

var (
	rootedBase  = regexp.MustCompile(`reporoot\.(Root\(\)|Path\()`)
	foldersOnly = regexp.MustCompile(`if [^{]*!\w+\.IsDir\(\)[^{]*\{\s*return`)
	leadingName = regexp.MustCompile(`^(\w+)`)
)

// rootedListings bir test dosyasında depo kökünden dosya listeleyen çağrıların satırlarıdır (metinden, codegraph'tan
// bağımsız): reporoot.WalkDir'in tabanı ya da `git -C <kök> ls-files`'ın kökü reporoot.Root()/Path'i ya da onlarla
// atanmış bir yereli taşır; yalnızca klasör gezen geri çağırma sayılmaz.
func rootedListings(t *testing.T, file string) []int {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	text := func(n ast.Node) string {
		return string(src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
	}
	var lines []int
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		body := text(fn.Body)
		rooted := func(e ast.Expr) bool {
			s := text(e)
			if rootedBase.MatchString(s) {
				return true
			}
			if m := leadingName.FindStringSubmatch(strings.TrimPrefix(s, "filepath.Join(")); m != nil {
				id := m[1]
				if regexp.MustCompile(`\b` + id + `\s*:?=\s*(filepath\.Join\()?reporoot\.(Root\(\)|Path\()`).MatchString(body) {
					return true
				}
			}
			return false
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			line := fset.Position(call.Pos()).Line
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WalkDir" && text(sel.X) == "reporoot" && len(call.Args) == 2 {
				if rooted(call.Args[0]) && !foldersOnly.MatchString(text(call.Args[1])) {
					lines = append(lines, line)
				}
			}
			if s := text(call); strings.HasPrefix(s, "exec.Command(\"git\"") && strings.Contains(s, "\"ls-files\"") {
				if !strings.Contains(s, "\"-C\"") || rootedBase.MatchString(s) || len(call.Args) > 2 && rooted(call.Args[2]) {
					lines = append(lines, line)
				}
			}
			return true
		})
	}
	return lines
}

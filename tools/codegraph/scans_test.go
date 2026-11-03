package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
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
	"app/x.tr":        "TR_var x = 1\n",
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
				if regexp.MustCompile(`\b` + id + `(?:,\s*[\w_]+)*\s*:?=\s*((filepath\.Join\()?reporoot\.(Root\(\)|Path\()|repoGraph\()`).MatchString(body) {
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
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch {
				case sel.Sel.Name == "WalkDir" && text(sel.X) == "reporoot" && len(call.Args) == 2:
					if rooted(call.Args[0]) && !foldersOnly.MatchString(text(call.Args[1])) {
						lines = append(lines, line)
					}
				case sel.Sel.Name == "ListFiles" && text(sel.X) == "codeindex" && len(call.Args) == 5 && rooted(call.Args[0]):
					lines = append(lines, line)
				case sel.Sel.Name == "GitTracked" && text(sel.X) == "codeindex" && len(call.Args) == 1 && rooted(call.Args[0]):
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

// Kural (DENETİM istek 80 Ş-c2, istek 82): tools/full_check.sh'in repo_scans test dizisi ve repo_scan_pkgs paket dizisi,
// graftaki kind == "scan" düğümlerinin Test* testlerine ve onların paketlerine birebir eşittir (düzen kuralları adımında
// bütün koşulan paketler ve Fuzz* hedefleri dışarıda). Yeni bir tarama testi ya da paketi diziye yazılmazsa ya da dizide
// eskimiş ad kalırsa test düşer.
func TestFullCheckRepoScansMatchesScanNodes(t *testing.T) {
	root, _, g := repoGraph(t)
	data, err := os.ReadFile(filepath.Join(root, "tools/full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(data), "\r\n", "\n")
	gotTests := parseBashArray(t, script, "repo_scans")
	gotPkgs := parseBashArray(t, script, "repo_scan_pkgs")
	layoutMatch := regexp.MustCompile(`(?m)^\s*go test [^\n]*-short ((?:\./\S+\s*)+)[^\n]*\n\s*\[ "\$\{PIPESTATUS\[0\]\}" -eq 0 \] \|\| fail "düzen kuralları"`).FindStringSubmatch(script)
	if layoutMatch == nil {
		t.Fatal("tools/full_check.sh: could not locate layout-rules go test line")
	}
	wholePkgs := strings.Fields(layoutMatch[1])
	isWholeLayoutPkg := func(dir string) bool {
		for _, p := range wholePkgs {
			clean := strings.TrimPrefix(p, "./")
			if strings.HasSuffix(clean, "/...") {
				prefix := strings.TrimSuffix(clean, "...")
				if strings.HasPrefix(dir+"/", prefix) {
					return true
				}
				continue
			}
			if strings.TrimSuffix(clean, "/") == dir {
				return true
			}
		}
		return false
	}
	if !regexp.MustCompile(`(?s)repo_scans\[@\].*"\$\{repo_scan_pkgs\[@\]\}"[^\n]*\n\s*\[ "\$\{PIPESTATUS\[0\]\}" -eq 0 \] \|\| fail "depo taramaları"`).MatchString(script) {
		t.Fatal("tools/full_check.sh: depo taramaları step must run ${repo_scans[@]} across \"${repo_scan_pkgs[@]}\"")
	}

	byID := map[string]*Node{}
	for i := range g.Nodes {
		byID[g.Nodes[i].ID] = &g.Nodes[i]
	}
	wantTestsSet := map[string]bool{}
	wantPkgsSet := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind != edgeScans {
			continue
		}
		sc, tn := byID[e.To], byID[e.From]
		if sc == nil || sc.Kind != kindScan || tn == nil {
			continue
		}
		name := tn.Name[strings.LastIndex(tn.Name, ".")+1:]
		if !strings.HasPrefix(name, "Test") {
			continue
		}
		dir := path.Dir(tn.File)
		if isWholeLayoutPkg(dir) {
			continue
		}
		wantTestsSet[name] = true
		wantPkgsSet["./"+dir] = true
	}
	wantTests := slices.Sorted(maps.Keys(wantTestsSet))
	wantPkgs := slices.Sorted(maps.Keys(wantPkgsSet))
	slices.Sort(gotTests)
	slices.Sort(gotPkgs)
	if !slices.Equal(gotTests, wantTests) {
		t.Errorf("tools/full_check.sh repo_scans mismatch:\n  got (%d):  %v\n  want (%d): %v", len(gotTests), gotTests, len(wantTests), wantTests)
	}
	if !slices.Equal(gotPkgs, wantPkgs) {
		t.Errorf("tools/full_check.sh repo_scan_pkgs mismatch:\n  got (%d):  %v\n  want (%d): %v", len(gotPkgs), gotPkgs, len(wantPkgs), wantPkgs)
	}
}

func parseBashArray(t *testing.T, script, name string) []string {
	t.Helper()
	m := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `=\(\s*(.*?)\s*\)`).FindStringSubmatch(script)
	if m == nil {
		t.Fatalf("tools/full_check.sh: array %s not found", name)
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		out = append(out, strings.Fields(line)...)
	}
	return out
}

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// libraryImportFiles bir kitaplık (lib/modelx.tr katmanx'i aktarır, lib/katmanx_test.tr kendi modülünü aktarır; adlar depodaki
// kitaplıkta yok: sahte depo dizesi gerçek kitaplık dosyasına bağlanmasın), onu aktaran
// bir betik ve kitaplığı dize, kodlu dize, betik ve kalıpla koşan Go testleridir.
func libraryImportFiles(t *testing.T) map[string]string {
	coded, err := source.Encode("TR_import \"tr/katmanx\"\nTR_print(1)\n")
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"lib/modelx.tr":             "TR_import \"tr/katmanx\"\nTR_func h() {\n}\n",
		"lib/katmanx.tr":            "TR_func k() {\n}\n",
		"lib/katmanx_test.tr":       "TR_import \"tr/katmanx\"\nTR_test \"k\" {\n    k()\n}\n",
		"tools/run.tr":              "TR_import \"tr/modelx\"\nh()\n",
		"internal/reporoot/root.go": "package reporoot\n\nfunc Path(parts ...string) string { return \"\" }\n",
		"app/app_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/reporoot"
)

const program = ` + "`" + `TR_import "tr/modelx"
TR_print(1)
` + "`" + `

func writeAndRun(string) {}

func TestReady(t *testing.T) { writeAndRun(program) }

func TestCoded(t *testing.T) { writeAndRun(` + strconv.Quote(coded) + `) }

func TestScript(t *testing.T) { _ = reporoot.Path("tools/run.tr") }

func TestEveryLibraryFile(t *testing.T) { _ = reporoot.Path("lib", "*.tr") }
`,
	}
}

// Kural (parti K2, kritik): aktarma test erişiminde bir bağımlılıktır. Test dizesindeki TR programının `aktar "tr/X"`'i
// (düz ve kodlu yazımda; dize yorumlayıcıya da dosyaya da gitse) lib/X.tr'ye test bağıdır; bir `.tr` dosyasının testleri
// onu geçişli aktaran dosyaların testleridir (Go testinin koştuğu betik, aktaran kitaplık modülü, test dosyasının TR
// testleri; kendi test dosyası aynı uzaklıkta önce). Adıyla ya da aktarmayla okuyan test kalıpla ve gömmeyle okuyanların
// önünde yazılır; ön denetimin seçimi (changed --go-tests) aynı erişimi kullanır.
func TestALibraryModuleIsTestedByTheTestsThatImportIt(t *testing.T) {
	g, err := buildGraph(auditModule(t, libraryImportFiles(t)))
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	mustEdge(t, g, app+"TestReady", "tr:file:lib/modelx.tr", edgeTests, inferred)
	mustEdge(t, g, app+"TestCoded", "tr:file:lib/katmanx.tr", edgeTests, inferred)
	ix := newIndex(g)
	distances := ix.testDistances(ix.byID["tr:file:lib/katmanx.tr"])
	for test, want := range map[string]int{app + "TestCoded": 0, app + "TestReady": 2, app + "TestScript": 3,
		"tr:test:lib/katmanx_test.tr:2": 1} {
		if got, ok := distances[test]; !ok || got != want {
			t.Errorf("lib/katmanx.tr: %s at %d steps (found %v), want %d", test, got, ok, want)
		}
	}
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["tr:file:lib/modelx.tr"], 0)
	text := out.String()
	first := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "    ") {
			first = line
			break
		}
	}
	if !strings.Contains(first, "app.TestReady") || !strings.Contains(text, "among other files (1;") ||
		strings.Index(text, "app.TestEveryLibraryFile") < strings.Index(text, "among other files") {
		t.Errorf("tests-for lib/modelx.tr: the importing test first, the pattern reader in its own group last:\n%s", text)
	}
	ts := newTouchSet()
	ts.files["lib/katmanx.tr"] = true
	out.Reset()
	ix.printGoTests(&out, ts)
	for _, name := range []string{"TestReady", "TestCoded", "TestScript"} {
		if !strings.Contains(out.String(), "app "+name+"\n") {
			t.Errorf("changed --go-tests for lib/katmanx.tr misses %s:\n%s", name, out.String())
		}
	}
}

// Kural (parti K2, ikinci yol): depodaki Go test dosyalarında bir metin sabiti `aktar "tr/X"` (kodlu yazımda
// TR_kw_import) taşıyorsa ve lib/X.tr varsa tests-for lib/X.tr sabiti taşıyan testi (sabit bir testin dışındaysa
// dosyanın bir testini) verir; Go testinin reporoot.Path ile verdiği `.tr` dosyasının geçişli kitaplık aktarmaları da
// o testi verir. Aktarmalar codeindex'ten değil düz metinden okunur (anahtar sözcüğün adı codes'tan).
func TestEveryLibraryImportOfARepositoryTestReachesTheLibraryFile(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	imp := regexp.MustCompile(`(?:` + regexp.QuoteMeta(codes.Source().Name(codes.KwImport)) + `|TR_import)\s+"((?:tr/)?[^"]+)"`)
	reached := map[string]map[string]int{}
	testsOf := func(lib string) map[string]int {
		if reached[lib] == nil {
			reached[lib] = ix.testDistances(ix.byID[trFileID(lib)])
		}
		return reached[lib]
	}
	// closure bir .tr dosyasının geçişli aktardığı kitaplık dosyalarıdır (göreli aktarma dosyanın klasöründen).
	var closure func(file string, seen map[string]bool)
	closure = func(file string, seen map[string]bool) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return
		}
		for _, m := range imp.FindAllStringSubmatch(string(data), -1) {
			next := path.Join(path.Dir(file), m[1])
			if rest, ok := strings.CutPrefix(m[1], "tr/"); ok {
				next = "lib/" + rest
			}
			if next = strings.TrimSuffix(next, ".tr") + ".tr"; ix.byID[trFileID(next)] != nil && !seen[next] {
				seen[next] = true
				closure(next, seen)
			}
		}
	}
	strs, scripts := 0, 0
	for i := range g.Nodes {
		file := &g.Nodes[i]
		if file.Kind != kindFile || !strings.HasSuffix(file.File, "_test.go") || strings.Contains(file.File, "/testdata/") {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(file.File))
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, abs, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		testAt := func(line int) string {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && isTestName(fd.Name.Name) &&
					fset.Position(fd.Pos()).Line <= line && line <= fset.Position(fd.End()).Line {
					return fd.Name.Name
				}
			}
			return ""
		}
		want := func(lib, test, why string) {
			for id := range testsOf(lib) {
				if n := ix.byID[id]; n.File == file.File && (test == "" || strings.HasSuffix(n.Name, "."+test)) {
					return
				}
			}
			t.Errorf("tests-for %s misses %s %s (%s)", lib, file.File, test, why)
		}
		err = testkit.GoStringLiterals(abs, func(value string, line int) {
			for _, m := range imp.FindAllStringSubmatch(value, -1) {
				if lib, ok := strings.CutPrefix(m[1], "tr/"); ok && ix.byID[trFileID("lib/"+lib+".tr")] != nil {
					strs++
					want("lib/"+lib+".tr", testAt(line), fmt.Sprintf("its string at line %d imports %s", line, m[1]))
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			lit, isLit := call.Args[0].(*ast.BasicLit)
			if !isSel || !isLit || sel.Sel.Name != "Path" {
				return true
			}
			script, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.HasSuffix(script, ".tr") || ix.byID[trFileID(script)] == nil {
				return true
			}
			test := testAt(fset.Position(call.Pos()).Line)
			if test == "" {
				return true
			}
			scripts++
			seen := map[string]bool{}
			closure(script, seen)
			for lib := range seen {
				if strings.HasPrefix(lib, "lib/") {
					want(lib, test, "it runs "+script+", which imports it")
				}
			}
			return true
		})
	}
	if strs < 5 || scripts < 1 {
		t.Errorf("only %d library imports in test strings and %d scripts found; the test files were not read", strs, scripts)
	}
}

// Kural (parti K2): her kitaplık dosyasında tests-for'un ilk test satırı bir davranış testidir, gömülü kitaplığı ya da
// lib/*.tr kalıbını okuyan bir test değil (önceden lib/hazir.tr'nin altı testinden dördü lib/embedded.go üzerinden
// geliyordu, asıl davranış testi yoktu).
func TestALibraryFileListsItsBehaviourTestsFirst(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	checked := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind != kindFile || !strings.HasPrefix(n.ID, "tr:file:lib/") || strings.HasSuffix(n.File, "_test.tr") {
			continue
		}
		var out strings.Builder
		ix.printTestsFor(&out, n, 0)
		_, shared := ix.splitSharedSites(ix.in[n.ID])
		for _, line := range strings.Split(out.String(), "\n") {
			if !strings.HasPrefix(line, "    ") {
				continue
			}
			for _, e := range shared {
				if strings.Contains(line, ix.byID[e.From].Name+" ") {
					t.Errorf("tests-for %s starts with a test that reads every library file: %s", n.File, line)
				}
			}
			break
		}
		checked++
	}
	if checked < 4 {
		t.Errorf("only %d library files checked", checked)
	}
}

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

func TestWholeListWalkersCoverReceiverMethodsAndDictionaryLookups(t *testing.T) {
	files := map[string]string{
		"dict/dict.json": `{"TR_fn_print": "print"}`,
		"dict/dict_generated.go": `package dict

var identities = [...]string{"", "TR_fn_print"}
`,
		"dict/dict.go": `package dict

import _ "embed"

//go:embed dict.json
var rawJSON []byte

type Code uint16

type Table struct{ list []Code }

func Load() *Table { _ = rawJSON; return &Table{} }

func (d *Table) Codes() []Code { return d.list }

func FromSpelling(s string) (Code, bool) {
	if s == identities[1] {
		return 1, true
	}
	return 0, false
}
`,
		"app/app_test.go": `package app

import (
	"testing"

	"example.com/audit/dict"
)

var allowExceptions = map[string]string{"TR_future": "pending"}

func isKnownIdentity(w string) bool {
	_, ok := dict.FromSpelling(w)
	return ok
}

func TestMethodWalker(t *testing.T) {
	d := dict.Load()
	for _, k := range d.Codes() {
		_ = k
	}
}

func TestExceptionsNotStale(t *testing.T) {
	for w := range allowExceptions {
		if isKnownIdentity(w) {
			t.Fatal(w)
		}
	}
}

func TestSingleLookupOnly(t *testing.T) {
	_, _ = dict.FromSpelling("TR_fn_print")
}
`,
	}
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["file:dict/dict.json"], 0)
	text := out.String()
	for _, want := range []string{"app.TestMethodWalker", "app.TestExceptionsNotStale"} {
		if !strings.Contains(text, want) {
			t.Errorf("tests-for dict/dict.json missing whole-list walker %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, "app.TestSingleLookupOnly") {
		t.Errorf("tests-for dict/dict.json must not include single-entry lookup:\n%s", text)
	}
	ts := newTouchSet()
	ts.files["dict/dict.json"] = true
	got := ix.goTestSelection(ts)
	for _, want := range []goTest{{"app", "TestMethodWalker"}, {"app", "TestExceptionsNotStale"}} {
		if !slices.Contains(got, want) {
			t.Errorf("goTestSelection(dict/dict.json) = %v, missing %v", got, want)
		}
	}
}

func TestChangedGoTestsSelectsScansAndListWalkers(t *testing.T) {
	g, err := buildGraph(auditModule(t, scanFiles))
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	ts := newTouchSet()
	ts.files["lib/a.go"] = true
	got := ix.goTestSelection(ts)
	if !slices.Contains(got, goTest{"checks", "TestGoOnly"}) {
		t.Errorf("goTestSelection(lib/a.go) = %v, missing scan test checks.TestGoOnly", got)
	}
}

// Kural (DENETİM istek 79, Madde 1): kod listesini Go API'siyle gezen ya da .tr dosyalarını tarayan her Go testi ilgili
// dosyanın tests-for çıktısında görünür ve changed --go-tests seçiminde yer alır.
func TestCodeListAndTRFileWalkersAreInTestsForAndChangedGoTests(t *testing.T) {
	root, _, g := repoGraph(t)
	tracked, ok := codeindex.GitTracked(root)
	if !ok {
		t.Skip("needs a git checkout")
	}
	ix := newIndex(g)
	ix.root = root

	assertCoveredAndSelected := func(targetFile string, want goTest) {
		t.Helper()
		n, code := ix.resolve(targetFile, &queryOpts{})
		if code != exitOK || n == nil {
			t.Fatalf("resolve(%s): code=%d", targetFile, code)
		}
		var out strings.Builder
		ix.printTestsFor(&out, n, 0)
		short := path.Base(want.dir) + "." + want.name
		if !strings.Contains(out.String(), want.name) {
			t.Errorf("tests-for %s does not list %s", targetFile, short)
		}
		ts := newTouchSet()
		ts.files[targetFile] = true
		if n.Kind == kindFile && strings.HasSuffix(targetFile, ".go") {
			for id, d := range ix.fileDecls(n).nodes {
				ts.nodes[id] = d
			}
		} else {
			ts.nodes[n.ID] = n
		}
		sel := ix.goTestSelection(ts)
		if !slices.Contains(sel, want) {
			t.Errorf("goTestSelection(%s) does not select %v", targetFile, want)
		}
	}

	examplesDir := "cmd/tr/e2e"
	for _, fn := range g.Nodes {
		if fn.Kind == kindFunc && strings.HasSuffix(fn.Name, ".TestExamplesCoverNames") {
			examplesDir = path.Dir(fn.File)
			break
		}
	}
	// Üç yeniden üretim vakası ve tanımdaki beklenen kural testleri:
	assertCoveredAndSelected("internal/devtools/codeindex/read.go", goTest{"internal/lang/source", "TestHeaderRulesLiveOnlyInSourceAndLexer"})
	assertCoveredAndSelected("internal/lang/codes/definitions.json", goTest{"internal/devtools/testrunner", "TestIdentityExceptionsAreNotStale"})
	assertCoveredAndSelected("internal/lang/codes/definitions.json", goTest{"internal/lang/codes", "TestEveryCodeHasDescription"})
	assertCoveredAndSelected("internal/lang/codes/definitions.json", goTest{examplesDir, "TestExamplesCoverNames"})
	assertCoveredAndSelected("lib/layers.tr", goTest{"tools/codegraph", "TestDottedMemberSpellingsGolden"})
	assertCoveredAndSelected("lib/layers.tr", goTest{"tools/codegraph", "TestEveryMemberSpellingInRepoTRFilesResolves"})

	// Bağımsız kaynak taraması: depodaki *_test.go dosyalarında `range codes.Codes()` / `range codes.DefinitionCodes()`
	// yapan her Test* işlevi definitions.json'da hem tests-for'da görünmeli hem de goTestSelection'da seçilmeli.
	rangeCodesRe := regexp.MustCompile(`\brange\s+codes\.(Codes|DefinitionCodes)\(\)`)
	foundWalkers := 0
	for _, rel := range tracked {
		if !strings.HasSuffix(rel, "_test.go") || strings.Contains(rel, "/testdata/") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !rangeCodesRe.Match(src) {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			text := string(src[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset])
			if rangeCodesRe.MatchString(text) {
				foundWalkers++
				assertCoveredAndSelected("internal/lang/codes/definitions.json", goTest{path.Dir(rel), fn.Name.Name})
			}
		}
	}
	if foundWalkers < 10 {
		t.Fatalf("independent scan found only %d direct codes.Codes() test functions", foundWalkers)
	}

	// Grafiğin .tr dosyalarını kapsayan her scan düğümünün Test* testi örnek bir .tr dosyası için seçilmeli.
	trScans := 0
	for _, h := range ix.scansCovering("examples/tr/01_merhaba.tr") {
		for _, tn := range h.tests {
			if gt, ok := goTestOf(tn); ok && strings.HasPrefix(gt.name, "Test") {
				trScans++
				assertCoveredAndSelected("examples/tr/01_merhaba.tr", gt)
			}
		}
	}
	if trScans < 6 {
		t.Fatalf("expected at least 6 Test* scans covering examples/tr/01_merhaba.tr, got %d", trScans)
	}
}

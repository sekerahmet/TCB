package main

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"
)

var skipFiles = map[string]string{
	"p/p.go": "package p\n\nfunc Lib() int { return 1 }\n\nfunc Plain() int { return 2 }\n",
	"p/p_test.go": `package p

import (
	"runtime"
	"testing"
)

type gpu struct{ lt *int }

func backend() *gpu { return &gpu{} }

func needLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
}

func TestLibDirect(t *testing.T) {
	g := backend()
	if g.lt == nil {
		t.Skip("no library")
	}
	_ = Lib()
}

func TestLibHelper(t *testing.T) {
	needLinux(t)
	_ = Lib()
}

func TestPlain(t *testing.T) { _ = Plain() }

func toolOrSkip(t *testing.T) string {
	for _, name := range []string{"a", "b"} {
		if name == runtime.GOOS {
			return name
		}
	}
	t.Skip("no tool")
	return ""
}

func linkOrSkip(t *testing.T) {
	if err := link(); err == nil {
		return
	}
	if runtime.GOOS != "windows" {
		t.Skip("no link")
	}
}

func link() error { return nil }

func TestAfterLoop(t *testing.T) {
	_ = toolOrSkip(t)
	_ = Lib()
}

func TestAfterEarlyReturn(t *testing.T) {
	linkOrSkip(t)
	_ = Lib()
}

func TestInitAndElse(t *testing.T) {
	if n := Lib(); n > 5 {
		_ = n
	} else if n == 0 {
		t.Skipf("zero")
	}
}

func TestLate(t *testing.T) {
	a := Lib()
	b := Lib()
	c := Lib()
	d := Lib()
	if a+b+c+d == 0 {
		t.Skip("late")
	}
}

func TestShortOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	_ = Plain()
}

func TestFatalIsNotACondition(t *testing.T) {
	if err := link(); err != nil {
		t.Fatal(err)
	}
	if testing.Short() {
		t.Skip("long")
	}
}
`,
	"p/q_test.go": `//go:build go1.20

package p

import "testing"

func TestBuilt(t *testing.T) { _ = Plain() }

func onlyHere(t *testing.T) { t.Skip("not on this platform") }

func maybeHere(t *testing.T, n int) {
	if n > 3 {
		t.Skip("too many")
	}
}
`,
	"p/r_test.go": `package p

import "testing"

func TestHelperInConstrainedFile(t *testing.T) {
	onlyHere(t)
	_ = Plain()
}

func TestConditionInConstrainedFile(t *testing.T) {
	maybeHere(t, Plain())
}
`,
}

// Kural: testin başındaki `if koşul { t.Skip }` (ya da başta çağrılan yardımcınınki) koşulu teste etiket olur ve
// tests-for her testin yanına yazar; listelenen testlerin hepsi koşula bağlıysa "CI'da hiçbiri koşmayabilir" notu çıkar,
// koşulsuz bir test varsa çıkmaz (deneme 9 H2).
func TestTestsForMarksTestsThatSkipUnderACondition(t *testing.T) {
	root := auditModule(t, skipFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	skips := map[string]string{}
	for _, n := range g.Nodes {
		if n.Skip != "" {
			skips[n.Name] = n.Skip
		}
	}
	want := map[string]string{
		"p.TestLibDirect":                  "g.lt == nil",
		"p.TestLibHelper":                  `runtime.GOOS != "linux"`,
		"p.TestAfterLoop":                  "no earlier return (name == runtime.GOOS)",
		"p.TestAfterEarlyReturn":           `no earlier return (err := link(); err == nil) && runtime.GOOS != "windows"`,
		"p.TestInitAndElse":                "(n := Lib(); n <= 5) && n == 0",
		"p.TestLate":                       "a+b+c+d == 0",
		"p.TestShortOnly":                  "testing.Short()",
		"p.TestFatalIsNotACondition":       "testing.Short()",
		"p.TestBuilt":                      "not go1.20 (build constraint)",
		"p.TestHelperInConstrainedFile":    "go1.20 (build constraint)",
		"p.TestConditionInConstrainedFile": "go1.20 (build constraint) && n > 3",
	}
	for name, cond := range want {
		if skips[name] != cond {
			t.Errorf("%s skips when %q, want %q", name, skips[name], cond)
		}
	}
	if skips["p.TestPlain"] != "" {
		t.Errorf("TestPlain has no condition: %q", skips["p.TestPlain"])
	}
	if code, _, errOut := runCLI("build", "--root", root); code != exitOK {
		t.Fatalf("build: %d %s", code, errOut)
	}
	_, out, _ := runCLI("tests-for", "p.Lib", "--root", root, "--no-rebuild")
	if !strings.Contains(out, "[skips when: g.lt == nil]") || !strings.Contains(out, "CI may run none of them") {
		t.Errorf("both tests of Lib skip under a condition:\n%s", out)
	}
	if _, out, _ = runCLI("tests-for", "p.Plain", "--root", root, "--no-rebuild"); strings.Contains(out, "CI may run none") {
		t.Errorf("Plain has a test with no condition:\n%s", out)
	}
}

// Kural: depodaki her Go testi, gövdesi (iç içe işlev değişmezleri dışında) ya da çağırdığı bir test dosyası işlevi
// bir t.Skip içeriyorsa bir atlama etiketi taşır: yol okuyucusu hiçbir kalıbı (ön deyimli if, else dalı, dördüncü
// deyimden sonraki atlama, döngüden sonra) kaçırmaz (bağımsız denetim grup 6, H2: sekiz doğrudan atlama kaçıyordu).
// Bu denetim yolu değil yalnızca Skip çağrısının varlığını arar.
func TestEveryRepositoryTestThatSkipsIsLabelled(t *testing.T) {
	_, m, g := repoGraph(t)
	skips := map[string]string{}
	for _, n := range g.Nodes {
		skips[n.ID] = n.Skip
	}
	bodies := map[string]*ast.BlockStmt{}
	infos := map[string]*types.Info{}
	var tests []string
	for _, u := range m.Units {
		for _, f := range u.Files {
			if !isTestFile(m.Rel(m.Fset.Position(f.Pos()).Filename)) {
				continue
			}
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					if fn, _ := u.Info.Defs[fd.Name].(*types.Func); fn != nil {
						bodies[funcRef(fn)], infos[funcRef(fn)] = fd.Body, u.Info
						if fd.Recv == nil && isTestName(fd.Name.Name) {
							tests = append(tests, funcRef(fn))
						}
					}
				}
			}
		}
	}
	var skipsIn func(id string, seen map[string]bool) bool
	skipsIn = func(id string, seen map[string]bool) bool {
		if seen[id] || bodies[id] == nil {
			return false
		}
		seen[id] = true
		found := false
		ast.Inspect(bodies[id], func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				return false // alt test ya da geri çağrı: testin kendi atlaması değil
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && strings.HasPrefix(sel.Sel.Name, "Skip") {
					if tt := infos[id].TypeOf(sel.X); tt != nil && strings.Contains(tt.String(), "testing.") {
						found = true
					}
				}
				if callee := nameIdent(n.Fun); callee != nil {
					if fn, ok := infos[id].Uses[callee].(*types.Func); ok && skipsIn(funcRef(fn), seen) {
						found = true
					}
				}
			}
			return !found
		})
		return found
	}
	checked := 0
	for _, id := range tests {
		if !skipsIn(id, map[string]bool{}) {
			continue
		}
		checked++
		if skips[id] == "" {
			t.Errorf("%s calls t.Skip but carries no skip label", id)
		}
	}
	if checked < 50 {
		t.Errorf("only %d skipping tests found; the repository's test files were not read", checked)
	}
}

// Kural: hiçbir TestMain paketin testlerini koşmadan başarıyla bitmez (m.Run'dan önce `return` ya da `os.Exit(0)` yok).
// Atlama etiketleri TestMain'i okumaz; böyle bir TestMain eklenirse paketin bütün testleri koşulsuz görünür, bu test
// düşer ve okuyucunun TestMain'i de okuması gerekir (bağımsız denetim grup 6, H2; bugün depoda beş TestMain var, hiçbiri
// böyle değil).
func TestNoTestMainSkipsItsPackage(t *testing.T) {
	_, m, _ := repoGraph(t)
	checked := 0
	for _, u := range m.Units {
		for _, f := range u.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || fd.Name.Name != "TestMain" || fd.Body == nil {
					continue
				}
				checked++
				ran := false
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch n := n.(type) {
					case *ast.FuncLit:
						return false
					case *ast.CallExpr:
						if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Run" {
							if tt := u.Info.TypeOf(sel.X); tt != nil && tt.String() == "*testing.M" {
								ran = true
							}
						}
						if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Exit" && !ran && len(n.Args) == 1 {
							if lit, ok := n.Args[0].(*ast.BasicLit); ok && lit.Value == "0" {
								t.Errorf("%s: TestMain exits 0 before m.Run", m.Fset.Position(n.Pos()))
							}
						}
					case *ast.ReturnStmt:
						if !ran {
							t.Errorf("%s: TestMain returns before m.Run", m.Fset.Position(n.Pos()))
						}
					}
					return true
				})
			}
		}
	}
	if checked == 0 {
		t.Error("no TestMain found; the repository's test files were not read")
	}
}

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: depoyu tip bilgisiyle yükleyip (`goload.Load`, `packages.Load`, `(*types.Config).Check`) bir arayüzün
// uygulayıcılarını, bir adın kullanımlarını/seçimlerini ya da bir paketin kullanıcılarını denetleyen testler, ilgili
// dosya değiştiğinde (yeni bir dosyada arayüz uygulaması eklense bile) `changed --go-tests` ve `tests-for` tarafından
// seçilir; ilgisiz dosya ya da geçici klasör (`t.TempDir()`) yüklemeleri seçilmez.
func TestRepositoryTypeLoadedTestsAreSelected(t *testing.T) {
	root, _, g := repoGraph(t)
	graphPath := filepath.Join(t.TempDir(), "graph.json")
	if err := writeGraph(graphPath, g); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := commandIn("changed", root, graphPath, "--no-rebuild", "--go-tests", "--base", "0b9fda0b", "83fa8859")
	if code != exitOK {
		t.Fatalf("changed --go-tests 0b9fda0b..83fa8859: exit %d: %s", code, errOut)
	}
	lines := "\n" + out
	for _, want := range []string{
		"tools/impact TestInterfaceImplementers",
		"tools/impact TestNameUses",
		"internal/compute/tensor TestBackendDataCallsGoThroughTheGate",
	} {
		if !strings.Contains(lines, "\n"+want+"\n") {
			t.Errorf("changed --go-tests --base 0b9fda0b 83fa8859 did not select %q", want)
		}
	}
	if strings.Contains(lines, "\ntools/impact TestUnknownNames\n") {
		t.Errorf("TestUnknownNames queries nonexistent symbols and must not be selected")
	}

	code, tfOut, errOut := commandIn("tests-for", root, graphPath, "--no-rebuild", "internal/stdlib/ml/members_freeze.go")
	if code != exitOK {
		t.Fatalf("tests-for members_freeze.go: exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, tfOut, "impact.TestInterfaceImplementers [func] tools/impact/main_test.go:")

	pkgTest := "go:func:github.com/sekerahmet/TCB/tools/impact.TestMembersAndPackages"
	mustEdge(t, g, pkgTest, "go:file:internal/lang/interp/interrupt.go", edgeTests, inferred)
	mustEdge(t, g, pkgTest, "go:file:internal/lang/source/prepare.go", edgeTests, inferred)
	if edge(g, "go:func:github.com/sekerahmet/TCB/tools/codegraph.TestANameCodeAliasIsAUseOfTheCode", "go:file:internal/lang/interp/assign.go", edgeTests) != nil {
		t.Errorf("whole-graph extractor consistency test must not gain type-query edges to unrelated packages")
	}
}

func writeTypeQueryFixtureFiles(t *testing.T, root string) {
	t.Helper()
	testkit.WriteFile(t, root, "internal/reporoot/root.go", "package reporoot\n\nfunc Root() string { return \".\" }\n")
	testkit.WriteFile(t, root, "internal/go/packages/packages.go", "package packages\n\nimport \"go/types\"\n\n"+
		"type Config struct {\n\tDir   string\n\tTests bool\n}\n\n"+
		"type Package struct {\n\tID, Name, PkgPath string\n\tTypes             *types.Package\n\tTypesInfo         *types.Info\n\tImports           map[string]*Package\n}\n\n"+
		"func Load(*Config, ...string) ([]*Package, error) { return nil, nil }\n")
	testkit.WriteFile(t, root, "api/api.go", "package api\n\ntype Runner interface{ Run() }\n\nconst TargetConst = 1\n")
	testkit.WriteFile(t, root, "sub/sub.go", "package sub\n\nfunc SubVal() int { return 1 }\n")
	testkit.WriteFile(t, root, "worker/worker.go", "package worker\n\ntype Worker struct{}\n\nfunc (*Worker) Run() {}\n")
	testkit.WriteFile(t, root, "client/client.go", "package client\n\nimport (\n\t\"example.com/mini/api\"\n\t\"example.com/mini/sub\"\n)\n\nfunc Use() int { return api.TargetConst + sub.SubVal() }\n")
	testkit.WriteFile(t, root, "other/other.go", "package other\n\nfunc Unrelated() int { return 0 }\n")
	testkit.WriteFile(t, root, "audit/audit_test.go", `package audit

import (
	"go/types"
	"testing"

	"example.com/mini/internal/go/packages"
	"example.com/mini/internal/reporoot"
)

func loadRepo(t *testing.T) []*packages.Package {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{Dir: reporoot.Root(), Tests: true}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func TestRunnerImplementers(t *testing.T) {
	pkgs := loadRepo(t)
	var iface *types.Interface
	for _, p := range pkgs {
		if obj := p.Types.Scope().Lookup("Runner"); obj != nil {
			iface = obj.Type().Underlying().(*types.Interface)
		}
	}
	for _, p := range pkgs {
		for _, n := range p.Types.Scope().Names() {
			if obj := p.Types.Scope().Lookup(n); obj != nil && types.Implements(types.NewPointer(obj.Type()), iface) {
				_ = obj
			}
		}
	}
}

func TestSymbolUses(t *testing.T) {
	for _, p := range loadRepo(t) {
		for _, o := range p.TypesInfo.Uses {
			if o != nil && o.Name() == "TargetConst" {
				_ = o
			}
		}
	}
}

func TestPackageUsers(t *testing.T) {
	for _, p := range loadRepo(t) {
		for _, o := range p.TypesInfo.Uses {
			if o != nil && o.Pkg() != nil && o.Pkg().Path() == "example.com/mini/sub" {
				_ = o
			}
		}
	}
}

func TestTempModuleIgnored(t *testing.T) {
	pkgs, _ := packages.Load(&packages.Config{Dir: t.TempDir()}, "./...")
	for _, p := range pkgs {
		if obj := p.Types.Scope().Lookup("Runner"); obj != nil {
			_ = types.Implements(obj.Type(), obj.Type().Underlying().(*types.Interface))
		}
	}
}
`)
}

func TestPackagesLoadTypeQueriesSelectMatchingChangesOnly(t *testing.T) {
	root := gitFixture(t)
	writeTypeQueryFixtureFiles(t, root)
	commitAll(t, root, "setup")

	makeBranch := func(name, rel, body string) {
		gitMust(t, root, "checkout", "-q", "-b", name, "main")
		testkit.WriteFile(t, root, rel, body)
		commitAll(t, root, name)
		gitMust(t, root, "checkout", "-q", "main")
	}
	makeBranch("claude/impl", "worker/worker.go", "package worker\n\ntype Worker struct{ v int }\n\nfunc (*Worker) Run() {}\n")
	makeBranch("claude/use", "client/client.go", "package client\n\nimport (\n\t\"example.com/mini/api\"\n\t\"example.com/mini/sub\"\n)\n\nfunc Use() int { return api.TargetConst + sub.SubVal() + 1 }\n")
	makeBranch("claude/unrelated", "other/other.go", "package other\n\nfunc Unrelated() int { return 42 }\n")

	graph := buildInto(t, root)
	check := func(branch string, want, unwanted []string) {
		t.Helper()
		code, out, errOut := commandIn("changed", root, graph, "--no-rebuild", "--go-tests", branch)
		if code != exitOK {
			t.Fatalf("%s: exit %d: %s", branch, code, errOut)
		}
		lines := "\n" + out
		for _, w := range want {
			if !strings.Contains(lines, "\n"+w+"\n") {
				t.Errorf("%s: missing %q in:\n%s", branch, w, out)
			}
		}
		for _, u := range unwanted {
			if strings.Contains(lines, "\n"+u+"\n") {
				t.Errorf("%s: unexpected %q in:\n%s", branch, u, out)
			}
		}
	}

	check("claude/impl",
		[]string{"audit TestRunnerImplementers"},
		[]string{"audit TestSymbolUses", "audit TestPackageUsers", "audit TestTempModuleIgnored"})
	check("claude/use",
		[]string{"audit TestSymbolUses", "audit TestPackageUsers"},
		[]string{"audit TestRunnerImplementers", "audit TestTempModuleIgnored"})
	check("claude/unrelated",
		nil,
		[]string{"audit TestRunnerImplementers", "audit TestSymbolUses", "audit TestPackageUsers", "audit TestTempModuleIgnored"})

	// Yeni bir dosyada arayüz uygulayıcısı eklendiğinde (checked-out dalda ön denetim / edithook senaryosu):
	gitMust(t, root, "checkout", "-q", "-b", "claude/new_impl", "main")
	testkit.WriteFile(t, root, "extra/extra.go", "package extra\n\ntype Extra struct{}\n\nfunc (*Extra) Run() {}\n")
	commitAll(t, root, "new_impl")
	newGraph := buildInto(t, root)
	checkNewCode, newOut, newErr := commandIn("changed", root, newGraph, "--no-rebuild", "--go-tests", "claude/new_impl")
	if checkNewCode != exitOK || !strings.Contains("\n"+newOut, "\naudit TestRunnerImplementers\n") {
		t.Fatalf("new file implementer must select TestRunnerImplementers (exit %d, err %s):\n%s", checkNewCode, newErr, newOut)
	}
}

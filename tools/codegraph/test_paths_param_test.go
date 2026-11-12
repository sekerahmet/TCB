package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: test dosyasında (veya testing alan yardımcıda) dosya okuma çağrısına verilen çıplak dosya adı (dizgi, sabit,
// yerel değişken) çağıran testin paket klasöründeki dosyaya bağlanır; kökteki aynı adlı dosyaya bağlanmaz, `.` klasörü ve
// üretim kodundaki okuma bağlanmaz. Yorumda "/" sonrası gelen yol kuyruğu (`docs/README.md`) paket dosyasını anmaz, çıplak
// dosya adı yorumu (`app.go`) anar.
func TestBareReadCallsAndSlashTailComments(t *testing.T) {
	files := map[string]string{
		"README.md":          "# root\n",
		"docs/README.md":     "# docs\n",
		"app/README.md":      "# app\n",
		"app/config.json":    "{}\n",
		"app/local.json":     "{}\n",
		"app/app.go":         "package app\n\nimport \"os\"\n\nfunc ProdRead() { _, _ = os.ReadFile(\"README.md\") }\n",
		"sub/sub.go":         "package sub\n",
		"testutil/helper.go": "package testutil\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc ReadPkgReadme(t *testing.T) { _, _ = os.ReadFile(\"README.md\") }\n",
		"app/app_test.go": `package app

import (
	"os"
	"testing"

	"example.com/audit/testutil"
)

const cfgFile = "config.json"

// Kural docs/README.md icindedir.
func TestDirectBareRead(t *testing.T) {
	_, _ = os.ReadFile("README.md")
	_, _ = os.ReadDir(".")
}

func TestConstAndLocalBareRead(t *testing.T) {
	local := "local.json"
	_, _ = os.Open(cfgFile)
	_, _ = os.Stat(local)
}

func TestCrossPkgHelperBareRead(t *testing.T) {
	testutil.ReadPkgReadme(t)
}

// Bkz. app.go ve docs/README.md.
func TestCommentCiteOnlyBareNotSlashTail(t *testing.T) {}
`,
		"sub/sub_test.go": `package sub

import (
	"os"
	"testing"

	"example.com/audit/app"
)

func TestMissingInPkgDoesNotLinkRoot(t *testing.T) {
	_, _ = os.ReadFile("README.md")
	app.ProdRead()
}
`,
	}
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	const (
		appPkg = "go:func:example.com/audit/app."
		subPkg = "go:func:example.com/audit/sub."
	)
	mustEdge(t, g, appPkg+"TestDirectBareRead", "doc:app/README.md", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestConstAndLocalBareRead", "file:app/config.json", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestConstAndLocalBareRead", "file:app/local.json", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestCrossPkgHelperBareRead", "doc:app/README.md", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestCommentCiteOnlyBareNotSlashTail", "go:file:app/app.go", edgeCites, inferred)
	mustEdge(t, g, appPkg+"TestCommentCiteOnlyBareNotSlashTail", "doc:docs/README.md", edgeCites, inferred)

	if e := edge(g, subPkg+"TestMissingInPkgDoesNotLinkRoot", "doc:README.md", edgeTests); e != nil {
		t.Errorf("bare read in sub must not link root README.md: %+v", e)
	}
	if e := edge(g, subPkg+"TestMissingInPkgDoesNotLinkRoot", "doc:app/README.md", edgeTests); e != nil {
		t.Errorf("prod bare read must not link app/README.md: %+v", e)
	}
	if e := edge(g, appPkg+"TestCommentCiteOnlyBareNotSlashTail", "doc:app/README.md", edgeCites); e != nil {
		t.Errorf("slash-tail comment docs/README.md must not cite app/README.md: %+v", e)
	}
	for _, e := range g.Edges {
		if e.From == appPkg+"TestDirectBareRead" && e.Kind == edgeTests && e.To != "doc:app/README.md" && !strings.HasPrefix(e.To, "go:pkg:") {
			t.Errorf("ReadDir(\".\") or bare read created unexpected edge: %+v", e)
		}
	}

	ix := newIndex(g)
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["doc:app/README.md"], 20)
	testkit.MustContain(t, out.String(), "app.TestDirectBareRead", "app.TestCrossPkgHelperBareRead")
	if strings.Contains(out.String(), "TestCommentCiteOnlyBareNotSlashTail") {
		t.Errorf("tests-for app/README.md must not cite slash-tail comment:\n%s", out.String())
	}
}

// Kural: test dışı kodda çağıranın verdiği köke Join edilen yol (`filepath.Join(root, filepath.FromSlash(sabit))`, ya da
// paket düzeyi bir yol listesi üzerinde dönen değişken) o kökün dosyasıdır; işleve ulaşan testler geçici fikstür kökü
// verebilir, depodaki dosyayı okudukları sayılmaz (gaps, "`tests-for planning_questions.txt` sınav testini varsayılan
// çıktıda göstermiyor": buildGraph'a ulaşan 172 test sınav dosyasının okuyucusu sayılıyordu). Sabiti kökten okuyan kod
// (`os.ReadFile(sabit)`) okuma olarak kalır.
func TestAPathJoinedToAParameterRootIsNotTheCallersRead(t *testing.T) {
	files := map[string]string{
		"data/list.txt":   "a\n",
		"data/table.md":   "# t\n",
		"data/direct.txt": "b\n",
		"app/app.go": `package app

import (
	"os"
	"path/filepath"
)

const listFile, directFile = "data/list.txt", "data/direct.txt"

var tables = []string{"data/table.md"}

// Load kökün altındaki listeyi ve tabloları okur.
func Load(root string) {
	_, _ = os.ReadFile(filepath.Join(root, filepath.FromSlash(listFile)))
	for _, rel := range tables {
		_, _ = os.ReadFile(filepath.Join(root, rel))
	}
}

// Direct dosyayı çalışma klasöründen okur.
func Direct() { _, _ = os.ReadFile(directFile) }
`,
		"app/app_test.go": `package app

import "testing"

func TestLoadFixture(t *testing.T) { Load(t.TempDir()) }

func TestDirect(t *testing.T) { Direct() }
`,
	}
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	for _, to := range []string{"file:data/list.txt", "doc:data/table.md"} {
		if e := edge(g, app+"TestLoadFixture", to, edgeTests); e != nil {
			t.Errorf("a path joined to a parameter root is not the test's read: %+v", e)
		}
	}
	mustEdge(t, g, app+"TestDirect", "file:data/direct.txt", edgeTests, inferred)
}

// Kural: test dosyasında veya testing alan yardımcıda `filepath.Abs("compare_commits.py")` ve (`cmd.Dir` veya `-C`
// olmadan) `exec.Command` / `exec.CommandContext` çağrılarına verilen çıplak betik adları (`.py`, `.sh`, `.js`,
// `.mjs`, `.cjs`; `[]string{...}` ve `append([]string{...}, ...)` dilim öğeleri dahil) yalnız çağıran testin paket
// klasöründeki izlenen betiğe bağlanır; `cmd.Dir = tmp`, `-C` veya betik uzantısız çıplak argümanlar (`README.md`,
// `model.tr`, `--list`) bağ kurmaz (Madde 1).
func TestExecCommandAndFilepathAbsBareScripts(t *testing.T) {
	files := map[string]string{
		"run.py":                   "print(0)\n",
		"bench/run.py":             "print(1)\n",
		"bench/compare_commits.py": "print(2)\n",
		"bench/check.sh":           "exit 0\n",
		"bench/view.mjs":           "export const x = 1;\n",
		"bench/README.md":          "# bench\n",
		"bench/model.tr":           "# TR 4\nTR_print(1)\n",
		"bench/bench_test.go": `package bench

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func runPy(t *testing.T, args ...string) {
	t.Helper()
	_, _ = exec.Command("python", append([]string{"-I", "run.py"}, args...)...).CombinedOutput()
}

func TestViaAppendHelper(t *testing.T) {
	runPy(t, "--help")
}

func TestCommandContextAndSliceLocal(t *testing.T) {
	all := []string{"-e", "check.sh"}
	_ = exec.CommandContext(context.Background(), "sh", all...)
	_ = exec.Command("node", "view.mjs")
}

func TestFilepathAbsBare(t *testing.T) {
	script, _ := filepath.Abs("compare_commits.py")
	cmd := exec.Command("python", script)
	cmd.Dir = t.TempDir()
	_ = cmd
}

func TestCmdDirDoesNotLinkPkgScript(t *testing.T) {
	cmd := exec.Command("python", "-I", "run.py")
	cmd.Dir = t.TempDir()
	var cmd2 = exec.Command("sh", "check.sh")
	cmd2.Dir = t.TempDir()
	_, _ = cmd, cmd2
}

func TestDashCDoesNotLinkPkgScript(t *testing.T) {
	_ = exec.Command("git", "-C", t.TempDir(), "run.py")
}

func TestNonScriptBareArgsDoNotLink(t *testing.T) {
	_ = exec.Command("python", "README.md", "model.tr", "--list")
}
`,
	}
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	const pkg = "go:func:example.com/audit/bench."

	mustEdge(t, g, pkg+"TestViaAppendHelper", "file:bench/run.py", edgeTests, inferred)
	mustEdge(t, g, pkg+"TestCommandContextAndSliceLocal", "file:bench/check.sh", edgeTests, inferred)
	mustEdge(t, g, pkg+"TestCommandContextAndSliceLocal", "file:bench/view.mjs", edgeTests, inferred)
	mustEdge(t, g, pkg+"TestFilepathAbsBare", "file:bench/compare_commits.py", edgeTests, inferred)

	for _, bad := range []struct{ from, to string }{
		{pkg + "TestViaAppendHelper", "file:run.py"},
		{pkg + "TestCmdDirDoesNotLinkPkgScript", "file:bench/run.py"},
		{pkg + "TestCmdDirDoesNotLinkPkgScript", "file:bench/check.sh"},
		{pkg + "TestDashCDoesNotLinkPkgScript", "file:bench/run.py"},
		{pkg + "TestNonScriptBareArgsDoNotLink", "doc:bench/README.md"},
		{pkg + "TestNonScriptBareArgsDoNotLink", "tr:file:bench/model.tr"},
		{pkg + "TestNonScriptBareArgsDoNotLink", "file:bench/model.tr"},
	} {
		if e := edge(g, bad.from, bad.to, edgeTests); e != nil {
			t.Errorf("unexpected edge %s -> %s: %+v", bad.from, bad.to, e)
		}
	}

	_, _, rg := repoGraph(t)
	const repoBench = "go:func:github.com/sekerahmet/TCB/tools/bench."
	for _, fn := range []string{"TestRunPyArguments", "TestRunPyModelProgramBuilds", "TestRunPyTestsExist"} {
		mustEdge(t, rg, repoBench+fn, "file:tools/bench/run.py", edgeTests, inferred)
	}
	for _, fn := range []string{"TestCompareCommitsVerdict", "TestCompareCommitsNoBenchmark", "TestCompareCommitsEndToEnd", "TestCompareCommitsArguments"} {
		mustEdge(t, rg, repoBench+fn, "file:tools/bench/compare_commits.py", edgeTests, inferred)
	}
}

package main

import (
	"strings"
	"testing"
)

// Kural: bir dosya ya da belgeye gelen `tests` / `walks` kenarının yeri test düğümünün kendi bildirim aralığının
// dışındaysa (paket düzeyi değişken ya da yardımcı işlev), `edgeLine` testin kendi `dosya:satır` yerini yazar;
// kendi gövdesindeki doğrudan okuma satırı, işlev hedefli sorgular (`kindFunc`) ve `splitSharedSites` ayrımı korunur.
func TestFileTestEdgeShowsTestDeclarationWhenSiteIsOutsideBody(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p/p.go":          "package p\n\nfunc Target() int { return 1 }\n",
		"p/sub/data.txt":  "ok\n",
		"p/sub/other.txt": "ok\n",
		"p/p_test.go":     fileEdgeSiteTestSource,
		"p/h_test.go":     "package p\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc readHelper(t *testing.T) {\n\tt.Helper()\n\t_, _ = os.ReadFile(\"sub/data.txt\")\n}\n\nfunc callTarget(t *testing.T) {\n\tt.Helper()\n\t_ = Target()\n}\n",
	})
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	ix.root = root

	// 1. Olumlu ve olumsuz (a, c): p/sub/data.txt üzerinde değişken/yardımcı üzerinden okuyanlar kendi bildirim satırıyla,
	// gövde içinde doğrudan okuyan kendi okuma satırıyla ve glob ile okuyan `among other files` altında görünür.
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["file:p/sub/data.txt"], defaultLimit)
	got := out.String()

	head, shared, hasShared := strings.Cut(got, "tests that read it among other files")
	if !hasShared {
		t.Fatalf("splitSharedSites must keep glob reader in shared-sites section:\n%s", got)
	}
	for _, want := range []string{
		"p.TestViaPkgVar [func] at p/p_test.go:11",
		"p.TestViaHelper [func] at p/p_test.go:15",
		"p.TestDirectBodyRead [func] at p/p_test.go:21",
	} {
		if !strings.Contains(head, want) {
			t.Errorf("direct tests section missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"p.TestViaPkgVar [func] at p/p_test.go:9",
		"p.TestViaHelper [func] at p/h_test.go:10",
		"p.TestDirectBodyRead [func] at p/p_test.go:19",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected edge site %q:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(shared, "p.TestSharedGlob [func] at p/p_test.go:25") {
		t.Errorf("shared glob reader must stay under shared sites with its body line:\n%s", shared)
	}

	// 2. Olumsuz (b): Go işlevi hedefinde (`kindFunc`) `edgeLine` yardımcı içindeki satırı korur (değiştirmez).
	fnEdge := &Edge{
		From: "go:func:example.com/audit/p.TestViaHelper",
		To:   "go:func:example.com/audit/p.Target",
		Kind: edgeTests,
		File: "p/h_test.go",
		Line: 15,
	}
	if gotLine := ix.edgeLine(fnEdge, fnEdge.From, true); gotLine != "p.TestViaHelper [func] at p/h_test.go:15" {
		t.Errorf("function target must preserve helper site, got %q", gotLine)
	}
	out.Reset()
	ix.printWhoUses(&out, ix.byID["go:func:example.com/audit/p.Target"], defaultLimit)
	if !strings.Contains(out.String(), "p.callTarget [func] at p/h_test.go:15") {
		t.Errorf("who-uses on function target must preserve call site:\n%s", out.String())
	}

	// 3. Depo grafiği: ptx_ozet.txt üzerinde gpu.TestMain paket değişkeni satırı (behavior_ptx_digest_test.go:29)
	// yerine kendi bildirim yeriyle (helpers_test.go:19) yazılır.
	repoRoot, _, rg := repoGraph(t)
	rix := newIndex(rg)
	rix.root = repoRoot
	ptx := rix.byID["file:internal/compute/gpu/testdata/ptx_ozet.txt"]
	if ptx == nil {
		t.Fatal("ptx_ozet.txt node not found")
	}
	out.Reset()
	rix.printTestsFor(&out, ptx, defaultLimit)
	ptxOut := out.String()
	if !strings.Contains(ptxOut, "gpu.TestMain [func] at internal/compute/gpu/helpers_test.go:19  [INFERRED]") {
		t.Errorf("ptx_ozet.txt must show gpu.TestMain at helpers_test.go:19:\n%s", ptxOut)
	}
	if strings.Contains(ptxOut, "gpu.TestMain [func] at internal/compute/gpu/behavior_ptx_digest_test.go:") {
		t.Errorf("ptx_ozet.txt must not show package variable line for gpu.TestMain:\n%s", ptxOut)
	}
}

const fileEdgeSiteTestSource = `package p

import (
	"os"
	"path/filepath"
	"testing"
)

var pkgDataPath = filepath.Join("sub", "data.txt")

func TestViaPkgVar(t *testing.T) {
	_, _ = os.ReadFile(pkgDataPath)
}

func TestViaHelper(t *testing.T) {
	readHelper(t)
}

func TestDirectBodyRead(t *testing.T) {
	_ = 1
	_, _ = os.ReadFile("sub/data.txt")
}

func TestSharedGlob(t *testing.T) {
	matches, _ := filepath.Glob("sub/*.txt")
	for _, m := range matches {
		_, _ = os.ReadFile(m)
	}
}

func TestCallsTargetViaHelper(t *testing.T) {
	callTarget(t)
}
`

// Kural: `tools/bench/*_test.go` içindeki `Test*`/`Fuzz*` birim testleri ölçüm değil gerçek testtir (`<- tests` altında
// yer alır); `Benchmark*` işlevleri (`tools/bench/*_test.go` içindekiler dahil) ve `tools/bench/` altındaki test dışı
// kodlar (`run.py`, `tools/bench/tr/*.tr`) ölçüm olarak kalır (Madde 1).
func TestToolsBenchTestFilesCountAsTestsWhileBenchmarksStayMeasurements(t *testing.T) {
	root := auditModule(t, map[string]string{
		"pkg/p.go":                   "package pkg\n\nfunc Target() int { return 1 }\n",
		"tools/bench/fixture_run.py": "print(1)\n",
		"tools/bench/fixture_run_test.go": `package bench

import (
	"os/exec"
	"testing"

	"example.com/audit/pkg"
)

func TestRunPyCheck(t *testing.T) {
	_ = exec.Command("python", "-I", "fixture_run.py")
	_ = pkg.Target()
}

func BenchmarkRunPy(b *testing.B) {
	_ = exec.Command("python", "-I", "fixture_run.py")
	_ = pkg.Target()
}
`,
	})
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	ix.root = root

	if isMeasurement(ix.byID["go:func:example.com/audit/tools/bench.TestRunPyCheck"]) {
		t.Error("TestRunPyCheck in tools/bench/fixture_run_test.go must not be a measurement")
	}
	if !isMeasurement(ix.byID["go:func:example.com/audit/tools/bench.BenchmarkRunPy"]) {
		t.Error("BenchmarkRunPy in tools/bench/fixture_run_test.go must stay a measurement")
	}
	if !isMeasurement(ix.byID["file:tools/bench/fixture_run.py"]) {
		t.Error("tools/bench/fixture_run.py must stay a measurement")
	}

	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["file:tools/bench/fixture_run.py"], defaultLimit)
	runPyOut := out.String()
	measHead, measTail, hasMeas := strings.Cut(runPyOut, "<- measurements, not tests")
	if !hasMeas {
		t.Fatalf("expected BenchmarkRunPy under measurements, not tests:\n%s", runPyOut)
	}
	if !strings.Contains(measHead, "<- tests (1):") || !strings.Contains(measHead, "bench.TestRunPyCheck") {
		t.Errorf("TestRunPyCheck must appear under <- tests:\n%s", runPyOut)
	}
	if !strings.Contains(measTail, "bench.BenchmarkRunPy") || strings.Contains(measTail, "TestRunPyCheck") {
		t.Errorf("only BenchmarkRunPy may appear under measurements:\n%s", runPyOut)
	}

	repoRoot, _, rg := repoGraph(t)
	rix := newIndex(rg)
	rix.root = repoRoot
	if !isMeasurement(rix.byID["tr:file:tools/bench/tr/fib.tr"]) {
		t.Error("tools/bench/tr/fib.tr must remain a measurement")
	}
	out.Reset()
	rix.printTestsFor(&out, rix.byID["file:tools/bench/run.py"], defaultLimit)
	repoRunOut := out.String()
	for _, want := range []string{"<- tests (3):", "bench.TestRunPyArguments", "bench.TestRunPyModelProgramBuilds", "bench.TestRunPyTestsExist"} {
		if !strings.Contains(repoRunOut, want) {
			t.Errorf("repo tests-for tools/bench/run.py missing %q:\n%s", want, repoRunOut)
		}
	}
	if strings.Contains(repoRunOut, "<- measurements, not tests") {
		t.Errorf("repo tests-for tools/bench/run.py must not classify TestRunPy* as measurements:\n%s", repoRunOut)
	}
}

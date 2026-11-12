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

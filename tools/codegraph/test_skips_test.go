package main

import (
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
	if skips["p.TestLibDirect"] != "g.lt == nil" || skips["p.TestLibHelper"] != `runtime.GOOS != "linux"` || skips["p.TestPlain"] != "" {
		t.Errorf("skip conditions: %v", skips)
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

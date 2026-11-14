package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// Kural: TR programlarını tip denetleyicisinden (Checker.CheckProgram) geçiren ortak test yardımcıları,
// seçenek bayrağını açık geçiren çağrılar ve denetleyici alt komutunu çalıştıran alt süreç testleri
// CheckProgram'a [INFERRED] tests kenarı kurar ve denetleyici dosyalarının cevabında yer alır: CheckProgram'ın
// dosyasında dar `run:` satırında (doğrudan test), öbür dosyalarda yalnız gruplarda (codegraph 7 A1: koşucular uzaktır,
// commit öncesi denetim koşar); bayrağı kapalı ya da denetleyiciye varmayan alt komutla çağrılan testler seçilmez.
func TestCheckerRunnerTestsLinkAndStayInTheAnswer(t *testing.T) {
	files := map[string]string{
		"internal/lang/checker/checker.go": `package checker

type Checker struct{}

func New() *Checker { return &Checker{} }

func (c *Checker) CheckProgram() { c.checkRule() }
`,
		"internal/lang/checker/rules.go": `package checker

func (c *Checker) checkRule() {}
`,
		"internal/lang/source/prepare.go": `package source

import "example.com/audit/internal/lang/checker"

type Options struct {
	TypeCheck bool
}

func Prepare(opt Options) {
	if !opt.TypeCheck {
		return
	}
	checker.New().CheckProgram()
}
`,
		"internal/testkit/runner/runner.go": `package runner

import "example.com/audit/internal/lang/checker"

func MustExecChecked(src string) {
	checker.New().CheckProgram()
}

func ExecUnchecked(src string) {}
`,
		"pkg/ml/value_test.go": `package ml

import (
	"testing"

	"example.com/audit/internal/lang/source"
	"example.com/audit/internal/testkit/runner"
)

func mustRun(t *testing.T, src string) { runner.MustExecChecked(src) }

func TestViaRunnerHelper(t *testing.T) { mustRun(t, "x") }

func TestViaOptionEnabled(t *testing.T) { source.Prepare(source.Options{TypeCheck: true}) }

func TestViaOptionDisabled(t *testing.T) { source.Prepare(source.Options{TypeCheck: false}) }

func TestUncheckedOnly(t *testing.T) { runner.ExecUnchecked("x") }
`,
		"cmd/cli/main.go": `package main

import "example.com/audit/internal/lang/checker"

func checkCmd() { checker.New().CheckProgram() }
func fmtCmd()   {}

func dispatch(cmd string) {
	switch cmd {
	case "check":
		checkCmd()
	case "fmt":
		fmtCmd()
	}
}

func main() { dispatch("fmt") }
`,
		"cmd/cli/e2e/e2e_test.go": `package e2e

import (
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	_ = exec.Command("go", "build", "-o", "cli", "example.com/audit/cmd/cli")
}

func runCheck(file string) {
	_ = exec.Command("cli", "check", file)
}

func TestSubprocessCheck(t *testing.T) { runCheck("doc.md") }

func TestSubprocessFmt(t *testing.T) { _ = exec.Command("cli", "fmt", "doc.md") }
`,
	}
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	const (
		checkID = "go:func:example.com/audit/internal/lang/checker.Checker.CheckProgram"
		mlPkg   = "go:func:example.com/audit/pkg/ml."
		e2ePkg  = "go:func:example.com/audit/cmd/cli/e2e."
	)
	mustEdge(t, g, mlPkg+"TestViaRunnerHelper", checkID, edgeTests, inferred)
	mustEdge(t, g, mlPkg+"TestViaOptionEnabled", checkID, edgeTests, inferred)
	mustEdge(t, g, e2ePkg+"TestSubprocessCheck", checkID, edgeTests, inferred)
	for _, neg := range []string{mlPkg + "TestViaOptionDisabled", mlPkg + "TestUncheckedOnly", e2ePkg + "TestSubprocessFmt"} {
		if e := edge(g, neg, checkID, edgeTests); e != nil {
			t.Errorf("unexpected checker test edge from %s: %+v", neg, e)
		}
	}

	ix := newIndex(g)
	for _, c := range []struct {
		target string
		inRun  bool // CheckProgram'ın dosyasında koşucular doğrudan testtir, dar satırdadır; öbür dosyada uzaktır (codegraph 7 A1)
	}{{"internal/lang/checker/checker.go", true}, {"internal/lang/checker/rules.go", false}} {
		target := c.target
		var buf bytes.Buffer
		ix.printTestsFor(&buf, ix.byID["go:file:"+target], 12)
		out := buf.String()
		run := strings.Join(runLines(out), "\n")
		for _, want := range []string{"TestSubprocessCheck", "TestViaRunnerHelper", "TestViaOptionEnabled"} {
			if !strings.Contains(out, want) || strings.Contains(run, want) != c.inRun {
				t.Errorf("%s: %q must be in the answer, in its run lines %v:\n%s", target, want, c.inRun, out)
			}
		}
		for _, notWant := range []string{"TestViaOptionDisabled", "TestUncheckedOnly", "TestSubprocessFmt"} {
			if strings.Contains(out, notWant) {
				t.Errorf("%s unexpectedly selected %q:\n%s", target, notWant, out)
			}
		}
	}
}

// Kural: depoda `tests-for internal/lang/checker/checker.go`'nun koşucuları (`cmd/trlang/e2e`'nin `TestDocTRCodeBlocks`'u,
// `internal/stdlib/ml`'in `TestTensorIsNotACondition`'ı) dar satırda değildir; seçimdedir ve kalan sayısına girer: commit
// öncesi denetim ve G4 koşar (codegraph 7 A1–A3; checker.go'nun başka paketlerde 175 doğrudan testi var, sınırı aşıyor).
func TestRepoCheckerRunnersAreLeftToTheCommitCheck(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	file := ix.byID["go:file:internal/lang/checker/checker.go"]
	if file == nil {
		t.Fatal("checker.go node missing")
	}
	ts := ix.fileDecls(file)
	s := ix.fileTests(file, ts)
	p := ix.runPlanFor(file, ts, s)
	cmds := p.commands()
	inRun := runCovers(cmds)
	for _, want := range []goTest{
		{"cmd/trlang/e2e", "TestDocTRCodeBlocks"},
		{"internal/stdlib/ml", "TestTensorIsNotACondition"},
	} {
		if inRun(want) || !slices.Contains(p.rest, want) {
			t.Errorf("checker.go: %v must be left to the commit check (counted, not run):\n%s", want, strings.Join(cmds, "\n"))
		}
	}
	if p.hub <= narrowHubLimit {
		t.Errorf("checker.go: %d direct tests in other packages, more than %d expected", p.hub, narrowHubLimit)
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"
)

// Kural: TR programlarını tip denetleyicisinden (Checker.CheckProgram) geçiren ortak test yardımcıları,
// seçenek bayrağını açık geçiren çağrılar ve denetleyici alt komutunu çalıştıran alt süreç testleri
// hem CheckProgram'a [INFERRED] tests kenarı kurar hem de denetleyici dosyalarının `run:` satırlarında yer alır;
// bayrağı kapalı ya da denetleyiciye varmayan alt komutla çağrılan testler seçilmez.
func TestCheckerRunnerTestsLinkAndAppearInRunLine(t *testing.T) {
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
	for _, target := range []string{"internal/lang/checker/checker.go", "internal/lang/checker/rules.go"} {
		var buf bytes.Buffer
		ix.printTestsFor(&buf, ix.byID["go:file:"+target], 12)
		out := buf.String()
		for _, want := range []string{"./cmd/cli/e2e/", "./pkg/ml/", "TestSubprocessCheck", "TestViaRunnerHelper", "TestViaOptionEnabled"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s run lines missing %q:\n%s", target, want, out)
			}
		}
		for _, notWant := range []string{"TestViaOptionDisabled", "TestUncheckedOnly", "TestSubprocessFmt"} {
			if strings.Contains(out, notWant) {
				t.Errorf("%s unexpectedly selected %q:\n%s", target, notWant, out)
			}
		}
	}
}

// Kural: depoda `tests-for internal/lang/checker/checker.go`'nun `run:` satırları hem `cmd/trlang/e2e`'nin
// `TestDocTRCodeBlocks`'unu hem de `internal/stdlib/ml`'in `TestTensorIsNotACondition`'ını içerir.
func TestRepoCheckerTestsForRunLineIncludesDocBlocksAndMLRunner(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	file := ix.byID["go:file:internal/lang/checker/checker.go"]
	if file == nil {
		t.Fatal("checker.go node missing")
	}
	ts := ix.fileDecls(file)
	s := ix.fileTests(file, ts)
	cmds := ix.runPlanFor(file, ts, s).commands()
	inRun := runCovers(cmds)
	for _, want := range []goTest{
		{"cmd/trlang/e2e", "TestDocTRCodeBlocks"},
		{"internal/stdlib/ml", "TestTensorIsNotACondition"},
	} {
		if !inRun(want) {
			t.Errorf("checker.go run: lines do not cover %v:\n%s", want, strings.Join(cmds, "\n"))
		}
	}
}

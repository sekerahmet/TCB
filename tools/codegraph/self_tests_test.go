package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// selfTestsOf n'nin tests-for cevabının koşu planının dışarıda tuttuğu codegraph testleridir (S3'ün saydıkları).
func (ix *index) selfTestsOf(n *Node) []goTest {
	ab := &answerBuffer{}
	ix.testsForBody(n)(ab, defaultLimit)
	if ab.plan == nil {
		return nil
	}
	return ab.plan().selfTests
}

// Kural (codegraph dilim 6, D6-3, Karar 3): codegraph'ın repoRule çağırmayan testleri tools/codegraph ve
// internal/devtools/codeindex dışındaki dosyaların tests-for koşu satırına ve commit kipinin seçimine
// (changed --go-tests --without-codegraph-self-tests) girmez, sayıları S3 ile yazılır; depo kuralları (repoRule) girer.
// İki klasörün içindeki dosyada hepsi seçimde kalır; bayraksız changed --go-tests (ön denetim) hiçbirini çıkarmaz.
func TestCodegraphSelfTestsStayOutOfOtherFilesSelections(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	rule, self := "TestEveryTrackedDataFileHasATestOrAReason", "TestEveryTrackedFileIsANode"
	data := ix.byID["file:internal/lang/messages/tr.json"]
	own := ix.byID[fileID("tools/codegraph/data_coverage_test.go")]
	if data == nil || own == nil {
		t.Fatal("tr.json or data_coverage_test.go is not in the graph")
	}
	// tests-for'un dar satırında ikisi de yok (kural bir taramadır); kalan sayısı codegraph testlerini de sayar (codegraph 7 A3).
	var out strings.Builder
	ix.printTestsForShort(&out, data)
	run := strings.Join(runLines(out.String()), "\n")
	left := ix.selfTestsOf(data)
	ab := &answerBuffer{}
	ix.testsForBody(data)(ab, defaultLimit)
	p := ab.plan()
	countLine := fmt.Sprintf("  narrow: these lines run the near tests; %d more are left to the commit check and G4 (--list names them)", len(p.rest)+len(left))
	if strings.Contains(run, self) || len(left) == 0 || slices.ContainsFunc(p.rest, ix.isCodegraphSelfTest) || !strings.Contains(out.String(), countLine) {
		t.Errorf("tests-for tr.json must not run the self-test %s and count the %d left out with the rest (%q):\n%s", self, len(left), countLine, out.String())
	}
	out.Reset()
	ix.printTestsForShort(&out, own)
	if len(ix.selfTestsOf(own)) > 0 {
		t.Errorf("a file of tools/codegraph keeps every codegraph test in its selection:\n%s", out.String())
	}

	ask := func(files ...string) (stdout, stderr string, plain []goTest) {
		ts := newTouchSet()
		for _, f := range files {
			ts.files[f] = true
			if n := ix.byID[fileID(f)]; n != nil {
				for id, d := range ix.fileDecls(n).nodes {
					ts.nodes[id] = d
				}
			}
		}
		var so, se strings.Builder
		ix.printGoTestsWithoutSelfTests(&so, &se, ts, false)
		return so.String(), se.String(), ix.goTestSelection(ts)
	}
	so, se, plain := ask("internal/lang/messages/tr.json")
	if !strings.Contains(so, "tools/codegraph "+rule+"\n") || strings.Contains(so, "tools/codegraph "+self+"\n") ||
		!strings.Contains(se, "codegraph tests (the full check on Colab runs them)") {
		t.Errorf("the commit mode's selection for tr.json keeps the rule, leaves out the self-test and counts it on stderr:\n%s\nstderr: %s", so, se)
	}
	if !slices.Contains(plain, goTest{codegraphPkg, self}) {
		t.Errorf("changed --go-tests without the flag (the pre-check) must still select %s", self)
	}
	so, se, _ = ask("internal/lang/messages/tr.json", "tools/codegraph/data_coverage_test.go")
	if !strings.Contains(so, "tools/codegraph "+self+"\n") {
		t.Errorf("a change in tools/codegraph keeps the self-tests it selects:\n%s\nstderr: %s", so, se)
	}
}

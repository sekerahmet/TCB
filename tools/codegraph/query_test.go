package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

func TestQueryShowsNeighbours(t *testing.T) {
	code, out, _ := cli(t, "query", "shape.Describe")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "shape.Describe [func] shape/shape.go:21", "-> calls", "shape.Shape.Area", "-> uses_message", "H0001",
		"<- calls", "shape.Twice")
}

func TestQueryAmbiguousListsCandidates(t *testing.T) {
	code, out, _ := cli(t, "query", "Code")
	if code != exitAmbiguous {
		t.Fatalf("exit %d, want %d", code, exitAmbiguous)
	}
	testkit.MustContain(t, out, "ambiguous", "messages.Code", "codes.Code")
}

func TestQueryQualifiedNameDisambiguates(t *testing.T) {
	code, out, _ := cli(t, "query", "internal/lang/codes.Code")
	if code != exitOK || !strings.Contains(out, "codes.Code [type]") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestQueryNotFound(t *testing.T) {
	code, out, errOut := cli(t, "query", "Zzzzz")
	if code != exitNotFound || out != "" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	testkit.MustContain(t, errOut, "not found")
}

func TestFlagsMayFollowNames(t *testing.T) {
	code, out, _ := cli(t, "query", "shape.Describe", "--limit", "1")
	if code != exitOK || !strings.Contains(out, "shape.Describe") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestWhoUsesMessage(t *testing.T) {
	code, out, _ := cli(t, "who-uses", "H0001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "first message", "<- uses_message", "shape.Describe [func] at shape/shape.go:")
	code, out, _ = cli(t, "who-uses", "H0002")
	if code != exitOK || !strings.Contains(out, "no uses") {
		t.Errorf("H0002 is unused: exit %d\n%s", code, out)
	}
}

func TestTestsFor(t *testing.T) {
	code, out, _ := cli(t, "tests-for", "shape.Describe")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "shape.TestDescribe", "rules guarded by these tests", "Empty shapes fail")
	code, out, _ = cli(t, "tests-for", "shape.Square.Area")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "shape.TestSquare_Area")
	_, out, _ = cli(t, "tests-for", "shape.Orphan")
	testkit.MustContain(t, out, "no tests found")
}

func TestPath(t *testing.T) {
	code, out, _ := cli(t, "path", "shape.Twice", "H0001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "path (directed, 2 steps)", "-calls->", "-uses_message->", "H0001")
	// İki ayrık düğüm arasında yol yoktur (fikstürde her şey birbirine bağlı, bu yüzden elle kurulmuş grafik).
	g := &Graph{SchemaVersion: schemaVersion, Extractor: extractorRevision,
		Nodes: []Node{{ID: "a", Kind: kindFunc, Name: "p.a"}, {ID: "b", Kind: kindFunc, Name: "p.b"}}}
	file := filepath.Join(t.TempDir(), "g.json")
	if err := writeGraph(file, g); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	if code := run([]string{"path", "--graph", file, "--no-rebuild", "p.a", "p.b"}, &so, &se); code != exitNotFound || !strings.Contains(se.String(), "no path") {
		t.Errorf("exit %d: %s", code, se.String())
	}
}

func TestUsageErrors(t *testing.T) {
	var so, se bytes.Buffer
	if code := run(nil, &so, &se); code != exitUsage {
		t.Errorf("no args: exit %d", code)
	}
	if code := run([]string{"bogus"}, &so, &se); code != exitUsage {
		t.Errorf("unknown command: exit %d", code)
	}
	if code, _, _ := cli(t, "path", "only-one"); code != exitUsage {
		t.Errorf("path with one name: exit %d", code)
	}
}

func TestMissingGraphFile(t *testing.T) {
	var so, se bytes.Buffer
	code := run([]string{"query", "--graph", filepath.Join(t.TempDir(), "none.json"), "x"}, &so, &se)
	if code != exitFail || !strings.Contains(se.String(), "run `go run ./tools/codegraph build`") {
		t.Errorf("exit %d: %s", code, se.String())
	}
}

func TestBuildCommandWritesGraph(t *testing.T) {
	slowFixture(t)
	out := filepath.Join(t.TempDir(), "sub", "graph.json")
	var so, se bytes.Buffer
	code := run([]string{"build", "--root", filepath.Join("testdata", "mod"), "--out", out}, &so, &se)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, se.String())
	}
	g, err := readGraph(out)
	if err != nil || len(g.Nodes) == 0 {
		t.Fatalf("graph not readable: %v", err)
	}
	testkit.MustContain(t, so.String(), "nodes", "EXTRACTED", "warnings:")
}

func TestTestsForIndirect(t *testing.T) {
	// Report → Twice → Describe: TestReport sınar Describe'ı üç adım uzaktan.
	_, out, _ := cli(t, "tests-for", "shape.Describe")
	testkit.MustContain(t, out, "shape.TestDescribe", "tests, indirect", "shape.TestReport", "3 steps via shape.Twice", "[INFERRED]")
	if strings.Count(out, "shape.TestDescribe") != 1 {
		t.Errorf("a direct test is not repeated as indirect:\n%s", out)
	}
	// Twice'ı yalnızca TestReport sınar, iki adım uzaktan (Twice ← Report ← TestReport); Report doğrudan.
	_, out, _ = cli(t, "tests-for", "shape.Twice")
	testkit.MustContain(t, out, "shape.TestReport", "2 steps via shape.Report")
	_, out, _ = cli(t, "tests-for", "shape.Report")
	if strings.Contains(out, "indirect") {
		t.Errorf("TestReport calls Report directly:\n%s", out)
	}
}

func TestIndirectDepthIsLimited(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{ID: "t", Kind: kindFunc, Name: "p.TestX", File: "p_test.go"},
		{ID: "a", Kind: kindFunc, Name: "p.a"}, {ID: "b", Kind: kindFunc, Name: "p.b"},
		{ID: "c", Kind: kindFunc, Name: "p.c"}, {ID: "d", Kind: kindFunc, Name: "p.d"},
		{ID: "e", Kind: kindFunc, Name: "p.e"},
	}}
	for _, p := range [][2]string{{"t", "a"}, {"a", "b"}, {"b", "c"}, {"c", "d"}, {"d", "e"}} {
		g.Edges = append(g.Edges, Edge{From: p[0], To: p[1], Kind: edgeCalls, Confidence: extracted})
	}
	ix := newIndex(g)
	if got := ix.indirectTests(ix.byID["e"]); len(got) != 0 {
		t.Errorf("5 steps away must not be found: %+v", got)
	}
	got := ix.indirectTests(ix.byID["d"])
	if len(got) != 1 || got[0].steps != 4 {
		t.Errorf("4 steps away must be found: %+v", got)
	}
}

func TestTextQueries(t *testing.T) {
	// ileti metninin başı/alt dizesi tek düğüme uyar
	code, out, _ := cli(t, "query", "first message")
	if code != exitOK || !strings.HasPrefix(out, "H0001 [message]") {
		t.Errorf("message text: exit %d\n%s", code, out)
	}
	// kural adının başı
	code, out, _ = cli(t, "query", "empty shapes")
	if code != exitOK || !strings.Contains(out, "Empty shapes fail [rule]") {
		t.Errorf("rule name: exit %d\n%s", code, out)
	}
	// kural açıklamasının alt dizesi
	code, out, _ = cli(t, "query", "zero area is an error")
	if code != exitOK || !strings.Contains(out, "[rule]") {
		t.Errorf("rule text: exit %d\n%s", code, out)
	}
	// birden çok düğüm: adaylar
	code, out, _ = cli(t, "query", "H000")
	if code != exitAmbiguous || !strings.Contains(out, "H0001") || !strings.Contains(out, "H0002") {
		t.Errorf("two messages: exit %d\n%s", code, out)
	}
	// çok kısa sorgu metin araması yapmaz
	if code, _, _ = cli(t, "query", "fi"); code != exitNotFound {
		t.Errorf("2-letter query: exit %d", code)
	}
}

func TestWhoUsesGoConstants(t *testing.T) {
	code, out, _ := cli(t, "who-uses", "shape.Sides")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "shape.Sides [const] shape/shape.go:", "const shape.Sides untyped int = 4", "<- uses", "shape.Corners [func]")
	code, out, _ = cli(t, "who-uses", "messages.H0001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "messages.H0001 [const]", "shape.Describe [func]")
	// Kodun düğümü Go sabitini gösterir; kullanılmayan kod "no uses" der ama sabitini yine bildirir.
	_, out, _ = cli(t, "who-uses", "H0002")
	testkit.MustContain(t, out, "Go constant: messages.H0002 [const]", "no uses")
	code, out, _ = cli(t, "who-uses", "shape.Default")
	if code != exitOK || !strings.Contains(out, "shape.Corners") {
		t.Errorf("var: exit %d\n%s", code, out)
	}
}

func TestCalleesAndCallsInto(t *testing.T) {
	code, out, _ := cli(t, "callees", "shape.Both")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "calls 2 distinct functions in 1 packages", "internal/lang/messages (2):", "messages.New [func]", "messages.Text [func]")
	_, out, _ = cli(t, "callees", "shape.Describe", "--in", "messages")
	testkit.MustContain(t, out, "(in messages)", "messages.New")
	if strings.Contains(out, "Shape.Area") {
		t.Errorf("--in must restrict the packages:\n%s", out)
	}
	if code, _, errOut := cli(t, "callees", "shape.Square"); code != exitUsage || !strings.Contains(errOut, "needs a function") {
		t.Errorf("a type has no callees: exit %d: %s", code, errOut)
	}

	_, out, _ = cli(t, "calls-into", "messages", "--min", "2")
	testkit.MustContain(t, out, "at least 2 distinct functions of internal/lang/messages: 1", "shape.Both [func]", "messages.New, messages.Text")
	if strings.Contains(out, "shape.Describe") {
		t.Errorf("Describe calls only one function of messages:\n%s", out)
	}
	_, out, _ = cli(t, "calls-into", "messages", "--min", "1", "--in", "shape")
	testkit.MustContain(t, out, "shape.Describe [func]", "shape.Both [func]")
	_, out, _ = cli(t, "calls-into", "messages", "--min", "1", "--in", "internal/lang/codes")
	testkit.MustContain(t, out, ": 0")
	if code, _, _ := cli(t, "calls-into", "shape.Describe"); code != exitNotFound {
		t.Errorf("calls-into needs a package: exit %d", code)
	}
}

func TestDidYouMean(t *testing.T) {
	code, out, errOut := cli(t, "query", "shape.Descrbe")
	if code != exitNotFound || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	testkit.MustContain(t, errOut, "not found", "did you mean:", "shape.Describe [func]")
	// Çok uzak bir ad öneri vermez.
	_, _, errOut = cli(t, "query", "qqqqzzzz")
	if strings.Contains(errOut, "did you mean") {
		t.Errorf("no close name:\n%s", errOut)
	}
}

func TestNameCodeChain(t *testing.T) {
	// Dildeki ad (Türkçe de İngilizce de) kodun düğümüne çözülür ve zincir yazılır.
	for _, q := range []string{"eger", "if"} {
		code, out, _ := cli(t, "query", q)
		if code != exitOK {
			t.Fatalf("%s: exit %d\n%s", q, code, out)
		}
		testkit.MustContain(t, out, "A001 [name]", "names: eger · if", "Go constant: codes.A001 [const]",
			"registered or used in Go", "shape at shape/shape.go:", "shape.runIf [shape/shape.go:")
	}
}

func TestWhoUsesByPackage(t *testing.T) {
	code, out, _ := cli(t, "who-uses", "messages.H0001", "--by-package")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "users by package (1 packages):", "shape: 2 functions, 3 use sites")
}

// Kural: dirMatches süzgeç yoksa (nil) her klasörü kabul eder, boş ama nil olmayan listede hiçbirini;
// underAny ise boş listede her zaman yanlıştır; klasörün kendisi ve altı uyar, ön ek benzeri uymaz.
// Neden: iki işlev tek bir önek denetiminde birleşti; nil ile boş liste farkı sessizce kayarsa
// sorgu süzgeci ya da kural denetimi tersine dönerdi.
func TestDirMatchesAndUnderAnyKeepTheirEmptyListRules(t *testing.T) {
	if !dirMatches("x/y", nil) || dirMatches("x/y", []string{}) {
		t.Error("dirMatches: nil her şeyi, boş liste hiçbir şeyi kabul etmeli")
	}
	if underAny("x/y", nil) || underAny("x/y", []string{}) {
		t.Error("underAny: boş listede yanlış olmalı")
	}
	for file, want := range map[string]bool{"x/y": true, "x/y/z.go": true, "x/yz": false, "x": false} {
		if got := underAny(file, []string{"x/y"}); got != want {
			t.Errorf("underAny(%q, [x/y]) = %v", file, got)
		}
		if got := dirMatches(file, []string{"x/y"}); got != want {
			t.Errorf("dirMatches(%q, [x/y]) = %v", file, got)
		}
	}
}

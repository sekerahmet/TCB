package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// branchFixture main'de fikstürü, ondan ayrılan üç claude/ dalını kurar:
// a ve b aynı satırı değiştirir (çakışır), c yalnızca belgeyi.
func branchFixture(t *testing.T) string {
	t.Helper()
	root := gitFixture(t)
	shape := filepath.Join(root, "shape", "shape.go")
	edit := func(branch string, apply func()) {
		gitMust(t, root, "checkout", "-q", "-b", branch, "main")
		apply()
		commitAll(t, root, branch)
		gitMust(t, root, "checkout", "-q", "main")
	}
	edit("claude/a", func() {
		replaceIn(t, shape, "_ = codes.KwIf", "_ = codes.KwIf // from a")
		replaceIn(t, shape, "func Orphan() {}", "func Orphan() {}\n\n// Added makes a new function.\nfunc Added() {}")
		replaceIn(t, filepath.Join(root, "internal", "lang", "messages", "tr.json"),
			"\"H0002\": \"second message\"", "\"H0002\": \"second message\",\n\"H0003\": \"third\"")
	})
	edit("claude/b", func() { replaceIn(t, shape, "_ = codes.KwIf", "_ = codes.KwIf // from b") })
	edit("claude/c", func() { replaceIn(t, filepath.Join(root, "docs", "guide.md"), "Second heading", "Another heading") })
	return root
}

func TestChangedMapsHunksTestsAndOtherBranches(t *testing.T) {
	root := branchFixture(t)
	graph := buildInto(t, root)
	code, out, errOut := commandIn("changed", root, graph, "claude/a")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out,
		"changed main...claude/a", "old-side lines",
		"shape.Describe [func]",                 // gövdesindeki satır değişti
		"H0003 (added or removed",               // grafikte olmayan yeni kod
		"shape.TestDescribe [func]", "  direct", // doğrudan test
		"shape.TestReport [func]", "  3 steps [INFERRED]", // Report → Twice → Describe
		"claude/b: 1 shared nodes (go:func:example.com/mini/shape.Describe)",
		"CONFLICT in shape/shape.go")
	if strings.Contains(out, "claude/c") {
		t.Errorf("claude/c touches other files only:\n%s", out)
	}
	if strings.Contains(out, "shape.Added [func]") {
		t.Errorf("a function that exists only on the branch has no graph node:\n%s", out)
	}
	// ... ama dalın kendi dosyasından adlandırılır.
	testkit.MustContain(t, out, "declarations on the branch that the graph does not have", "shape/shape.go: Added (func)")
}

// Kural: changed --go-tests ön denetimin koşacağı Go testlerini satır başına "klasör test" olarak verir: yalnız bir
// veri dosyası (grafiğin ayrıştırmadığı bir .txt) değişse de onu okuyan test seçilir. Ön denetim eskiden yalnız .go
// dosyası değişen paketleri koşuyordu: planning_questions.txt kırıkken codegraph sınavı koşmadan "geçti" dedi.
func TestChangedGoTestsSelectsTheTestsOfAChangedDataFile(t *testing.T) {
	root := gitFixture(t)
	testkit.WriteFile(t, root, "data/cases.txt", "a\n")
	testkit.WriteFile(t, root, "shape/cases_test.go", "package shape\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\n"+
		"func TestCasesReadable(t *testing.T) {\n\tif _, err := os.ReadFile(filepath.Join(\"..\", \"data/cases.txt\")); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n")
	commitAll(t, root, "cases")
	gitMust(t, root, "checkout", "-q", "-b", "claude/d", "main")
	testkit.WriteFile(t, root, "data/cases.txt", "a\nb\n")
	commitAll(t, root, "claude/d")
	graph := buildInto(t, root)
	code, out, errOut := commandIn("changed", root, graph, "--go-tests", "claude/d")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains("\n"+out, "\nshape TestCasesReadable\n") {
		t.Errorf("the test that reads the changed data file must be selected:\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if f := strings.Fields(line); len(f) != 2 || !strings.HasPrefix(f[1], "Test") && !strings.HasPrefix(f[1], "Fuzz") {
			t.Errorf("each line must be \"folder TestName\": %q", line)
		}
	}
}

func TestChangedOnCheckedOutBranchMapsNewSide(t *testing.T) {
	slowFixture(t)
	root := branchFixture(t)
	gitMust(t, root, "checkout", "-q", "claude/a")
	graph := buildInto(t, root)
	code, out, errOut := commandIn("changed", root, graph, "claude/a")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "new-side lines", "shape.Added [func]", "H0003 [message]")
}

func TestChangedAcceptsRangeAndRejectsUnknownRef(t *testing.T) {
	slowFixture(t)
	root := branchFixture(t)
	graph := buildInto(t, root)
	code, out, _ := commandIn("changed", root, graph, "main..claude/c")
	if code != exitOK || !strings.Contains(out, "doc sections") || !strings.Contains(out, "Usage [heading]") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if code, _, _ := commandIn("changed", root, graph, "claude/missing"); code != exitFail {
		t.Errorf("unknown ref: exit %d", code)
	}
}

// Kural: çıkış yapılmış dalda `changed` ve `dups --changed` değişen satırları ortak atadan çalışma ağacına kadar alır
// (changeHunks): commit'lenmemiş düzenleme ve sahnelenmiş yeni dosya görünür, commit'lenmiş değişikliğin üstüne eklenen
// satırlar onu kaydırmaz. Commit'ten önce koşan denetim (ön denetim, `dups --changed main`) aksi halde o anki kopyayı ve
// testleri kaçırırdı (MEKANİK 2026-10-25, DENETİM 2026-10-26).
func TestChangedSeesTheWorkingTree(t *testing.T) {
	root := branchFixture(t)
	gitMust(t, root, "checkout", "-q", "claude/a")
	shape := filepath.Join(root, "shape", "shape.go")
	// Commit'lenmiş değişikliklerin (Describe'daki yorum, yeni Added) üstüne 20 satır, Orphan'a commit'lenmemiş bir düzenleme.
	replaceIn(t, shape, "package shape", "package shape"+strings.Repeat("\n// pad", 20))
	replaceIn(t, shape, "func Orphan() {}", "func Orphan() { _ = Sides }")
	graph := buildInto(t, root)
	code, out, errOut := commandIn("changed", root, graph, "--all", "claude/a")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "uncommitted edits included", "shape.Describe [func]", "shape.Added [func]", "shape.Orphan [func]",
		"shape.TestDescribe [func]")
	for _, wrong := range []string{"shape.Square.Area [method]", "shape.Twice [func]", "shape.Report [func]"} {
		if strings.Contains(out, wrong) {
			t.Errorf("lines shifted by the uncommitted edit map to %s:\n%s", wrong, out)
		}
	}

	// Yalnız bir testin değişmesi: test kendini koşar (eksikler listesi, ince ayar: test dosyasının kendi testleri).
	gitMust(t, root, "checkout", "-q", "--", ".")
	replaceIn(t, filepath.Join(root, "shape", "shape_test.go"), "func TestTwiceName(t *testing.T) {}", "func TestTwiceName(t *testing.T) { t.Log() }")
	code, out, errOut = commandIn("changed", root, graph, "--all", "HEAD")
	if code != exitOK || !strings.Contains(out, "changed tests (they run themselves) (1):\n    shape.TestTwiceName [func]") {
		t.Errorf("a changed test runs itself: exit %d\n%s%s", code, out, errOut)
	}

	root = branchWithCopy(t)
	testkit.WriteFile(t, root, "extra.go", strings.TrimPrefix(loopSource("d", "int"), "src:"))
	gitMust(t, root, "add", "extra.go") // sahnelenmiş yeni dosya
	data, err := os.ReadFile(filepath.Join(root, "old2.go"))
	if err != nil {
		t.Fatal(err)
	}
	copyE := strings.TrimPrefix(loopSource("e", "int"), "src:package p\n")
	testkit.WriteFile(t, root, "old2.go", string(data)+copyE) // commit'lenmiş dosyada sahnelenmemiş kopya
	var so, se bytes.Buffer
	if code := run([]string{"dups", "--root", root, "--min", "60", "--json", "--changed", "main"}, &so, &se); code != exitOK {
		t.Fatalf("dups: exit %d: %s", code, se.String())
	}
	var res dupResult
	if err := json.Unmarshal(so.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	oldEnd := strings.Count(string(data), "\n")
	for _, c := range res.Clusters {
		for _, l := range c.Locs {
			if l.Changed {
				files[l.File] = true
				if l.File == "old2.go" && l.From <= oldEnd {
					t.Errorf("old2.go:%d is committed code, only the appended copy (after line %d) changed", l.From, oldEnd)
				}
			}
		}
	}
	if got := fmt.Sprint(files); got != "map[extra.go:true new.go:true old2.go:true]" {
		t.Errorf("uncommitted copies: changed files %s, want extra.go (staged), new.go (committed) and old2.go (unstaged)", got)
	}
}

func TestParseDiff(t *testing.T) {
	diff := "diff --git a/x.go b/x.go\nindex 1..2 100644\n--- a/x.go\n+++ b/x.go\n@@ -3,2 +3 @@ func f() {\n-a\n-b\n+c\n@@ -10,0 +10,2 @@\n+d\n+e\n"
	hs := parseDiff(diff)
	if len(hs) != 2 || hs[0].file != "x.go" || hs[0].oldStart != 3 || hs[0].oldCount != 2 || hs[0].newCount != 1 ||
		hs[1].oldCount != 0 || hs[1].newStart != 10 || hs[1].newCount != 2 || len(hs[0].changedLines) != 3 {
		t.Errorf("hunks: %+v", hs)
	}
	pureRename := "diff --git a/old.tr b/new.tr\nsimilarity index 100%\nrename from old.tr\nrename to new.tr\n"
	if r := parseDiff(pureRename); len(r) != 1 || r[0].file != "new.tr" || r[0].oldFile != "old.tr" || r[0].newCount != 0 {
		t.Errorf("pure rename: %+v", r)
	}
}

// Kural: `changed` ve `dups --changed` yeniden adlandırmayı (`git diff -M` ve aralıktan HEAD'e ad zinciri) izler:
// adı değişen dosya "mapped to no node"a düşmez, bugünkü düğümlerine ve testlerine bağlanır; yeniden adlandırılan
// Go dosyasındaki dokunulmamış kopya dalın yeni tekrarı sayılmaz.
func TestChangedAndDupsChangedFollowRenames(t *testing.T) {
	root := gitFixture(t)
	replaceIn(t, filepath.Join(root, "lib", "geo.tr"), "TR_print(k.alan())", "TR_print(k.alan()) # iz")
	commitAll(t, root, "edit-geo")
	gitMust(t, root, "mv", "lib/geo.tr", "lib/shapes.tr")
	gitMust(t, root, "mv", "lib/geo_test.tr", "lib/shapes_test.tr")
	replaceIn(t, filepath.Join(root, "lib", "shapes.tr"), "# geo:", "# shapes:")
	replaceIn(t, filepath.Join(root, "lib", "shapes_test.tr"), `TR_import "tr/geo"`, `TR_import "tr/shapes"`)
	commitAll(t, root, "rename-geo")
	graph := buildInto(t, root)

	code, out, errOut := commandIn("changed", root, graph, "--no-rebuild", "HEAD~2..HEAD~1")
	if code != exitOK {
		t.Fatalf("before rename: exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "shapes.yaz_alan [tr_func] lib/shapes.tr:", "lib/shapes_test.tr")
	if strings.Contains(out, "mapped to no node") || strings.Contains(out, "none found") {
		t.Errorf("commit before rename must map to today's file and tests:\n%s", out)
	}

	code, out, errOut = commandIn("changed", root, graph, "--no-rebuild", "HEAD~1..HEAD")
	if code != exitOK {
		t.Fatalf("rename commit: exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "lib/shapes.tr [file] lib/shapes.tr:1", "lib/shapes_test.tr")
	if strings.Contains(out, "mapped to no node") || strings.Contains(out, "lib/geo.tr") {
		t.Errorf("rename commit must map to new file nodes, not old paths or unmapped:\n%s", out)
	}

	dupRoot := branchWithCopy(t)
	gitMust(t, dupRoot, "mv", "old2.go", "renamed.go")
	gitMust(t, dupRoot, "commit", "-q", "-m", "rename old2")
	var so, se bytes.Buffer
	if code := run([]string{"dups", "--root", dupRoot, "--min", "60", "--json", "--changed", "main"}, &so, &se); code != exitOK {
		t.Fatalf("dups: exit %d: %s", code, se.String())
	}
	var res dupResult
	if err := json.Unmarshal(so.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Clusters {
		for _, l := range c.Locs {
			if l.Changed && l.File != "new.go" {
				t.Errorf("untouched duplicate in renamed file %s marked changed: %+v", l.File, l)
			}
		}
	}
}

// Eski bir sürümün kurduğu grafik (düğümlerde son satır yok) imzası aynı olsa da yeniden
// kurulur: aksi halde hunk'lar yalnızca bildirim satırına denk gelen düğümlere eşlenirdi.
func TestChangedRebuildsGraphFromOlderExtractor(t *testing.T) {
	slowFixture(t)
	root := branchFixture(t)
	graph := buildInto(t, root)
	g, err := readGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	g.Extractor = 0
	for i := range g.Nodes {
		g.Nodes[i].End = 0
	}
	if err := writeGraph(graph, g); err != nil {
		t.Fatal(err)
	}
	// --no-rebuild: eski grafik kullanılır, uyarı verilir, işlev eşlenmez.
	code, out, errOut := commandIn("changed", root, graph, "--no-rebuild", "claude/a")
	if code != exitOK || !strings.Contains(errOut, "older codegraph") || strings.Contains(out, "shape.Describe [func]") {
		t.Fatalf("stale format with --no-rebuild: exit %d\nout: %s\nerr: %s", code, out, errOut)
	}
	code, out, errOut = commandIn("changed", root, graph, "claude/a")
	if code != exitOK || !strings.HasPrefix(errOut, "graph rebuilt (") || !strings.Contains(out, "shape.Describe [func]") {
		t.Errorf("an older graph must be rebuilt: exit %d\nout: %s\nerr: %s", code, out, errOut)
	}
}

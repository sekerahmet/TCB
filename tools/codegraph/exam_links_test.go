package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: planlama sınavının bir sorusunda adı geçen her düğüm (komut argümanı ya da beklenti sözcüğü: Go/TR bildirimi,
// ad/ileti kodu, belge ya da dosya yolu, en az iki parçalı nokta son eki dahil) yalnız TestPlanningQuestionsAreAnswered'dan
// names_in_exam kenarı alır; düz tests kenarı kurulmaz, tests-for ayrı başlıkta gösterir.
func TestEveryNodeTheExamNamesIsLinkedFromTheExam(t *testing.T) {
	_, m, g := repoGraph(t)
	ix := newIndex(g)
	examID := "go:func:" + m.Path + "/tools/codegraph." + examTest
	byKey := map[string][]string{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		for _, k := range examNodeKeys(n) {
			byKey[k] = append(byKey[k], n.ID)
		}
	}
	linked := map[string]bool{}
	for _, e := range g.Edges {
		if e.File == examFile && e.Kind == edgeTests {
			t.Fatalf("exam question created a direct tests edge to %s", e.To)
		}
		if e.Kind == edgeNamesInExam {
			if e.From != examID {
				t.Errorf("names_in_exam edge from %s, want only %s", e.From, examID)
			}
			linked[e.To] = true
		}
	}
	checked := 0
	for _, r := range parseExam(t, filepath.Join("testdata", "planning_questions.txt")) {
		words := append([]string(nil), r.args...)
		for _, c := range r.checks {
			_, val, _ := strings.Cut(c, ": ")
			for _, part := range strings.Split(strings.ReplaceAll(val, " || ", " => "), " => ") {
				words = append(words, strings.Fields(part)...)
			}
		}
		for _, raw := range words {
			cands := []string{raw, strings.Trim(raw, ",:;()[]`\"")}
			if f, _, ok := strings.Cut(raw, ":"); ok && strings.Contains(f, "/") {
				cands = append(cands, f)
			}
			for _, w := range cands {
				for _, id := range byKey[w] {
					if id == examID {
						continue
					}
					checked++
					if !linked[id] {
						t.Errorf("planning_questions.txt:%d names %s (%q), missing names_in_exam from %s", r.line, id, w, examTest)
					}
				}
			}
		}
	}
	if !stamped(examFile) {
		t.Errorf("%s is not in stamped(); editing exam questions would leave graph stale", examFile)
	}
	if checked < 200 {
		t.Fatalf("only %d exam links verified; independent scan is broken", checked)
	}
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID[fileID("internal/lang/interp/compiler_expr.go")], 0)
	got := out.String()
	testkit.MustContain(t, got, "<- exam questions that name it (1; questions are plain text in "+examFile+"):", examTest)
	if direct, _, ok := strings.Cut(got, "<- exam questions that name it"); ok && strings.Contains(direct, examTest) {
		t.Errorf("%s mixed into direct/indirect test groups:\n%s", examTest, got)
	}
}

// Kural: sınavda anılan düğümü değiştiren, yeniden adlandıran ya da silen değişiklik changed --go-tests seçiminde sınavı
// gösterir; anılmayan düğüm değişikliği ya da silinmesi göstermez; tests-for'da doğrudan testlere karışmaz; sınav dosyası
// değişince sourceStamp değişir.
func TestExamSelectionOnModifyRenameDeleteAndStamp(t *testing.T) {
	root := gitFixture(t)
	shapeFile := filepath.Join(root, "shape", "shape.go")
	origShape := "\nfunc NamedInExam() int { return 1 }\nfunc (s Square) Perimeter() float64 { return 4 * s.Side }\nfunc UnnamedHelper() int { return 2 }\n"
	replaceIn(t, shapeFile, "func Orphan() {}", "func Orphan() {}"+origShape)
	testkit.WriteFile(t, root, "tools/codegraph/exam_test.go", "package main\n\nimport \"testing\"\n\n"+
		"func TestPlanningQuestionsAreAnswered(t *testing.T) {}\n"+
		"func TestExamFormatOnly(t *testing.T) {}\n")
	questions := "codegraph query shape.NamedInExam | contains: Square.Perimeter | contains: shape.Describe | src: fixture, 2026-11-05\n"
	testkit.WriteFile(t, root, examFile, questions)
	commitAll(t, root, "add exam fixture")

	graph := buildInto(t, root)
	code, out, errOut := commandIn("tests-for", root, graph, "shape.NamedInExam")
	if code != exitOK {
		t.Fatalf("tests-for NamedInExam exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "no tests found (only exam questions that name it, below)", "<- exam questions that name it", examTest)
	if strings.Contains(out, "TestExamFormatOnly") || strings.Contains(out, "<- tests") {
		t.Errorf("unexpected test link in tests-for NamedInExam:\n%s", out)
	}
	_, descOut, _ := commandIn("tests-for", root, graph, "shape.Describe")
	if before, _, ok := strings.Cut(descOut, "<- exam questions that name it"); !ok || strings.Contains(before, examTest) {
		t.Errorf("Describe must list exam separately from direct/indirect tests:\n%s", descOut)
	}

	// Windows'ta genel core.autocrlf checkout'ta CRLF yazar, satır sonuyla silme eşleşmezdi.
	gitMust(t, root, "config", "core.autocrlf", "false")
	checkChanged := func(name string, edit func(), wantExam bool) {
		t.Helper()
		gitMust(t, root, "checkout", "-q", "-B", "claude/"+name, "main")
		edit()
		gPath := buildInto(t, root)
		c, got, eOut := commandIn("changed", root, gPath, "--go-tests", "HEAD")
		if c != exitOK {
			t.Fatalf("%s: exit %d: %s", name, c, eOut)
		}
		has := strings.Contains("\n"+got, "\ntools/codegraph "+examTest+"\n")
		if has != wantExam {
			t.Errorf("%s: exam selected = %v, want %v; output:\n%s", name, has, wantExam, got)
		}
		gitMust(t, root, "checkout", "-q", "-f", "main")
	}

	checkChanged("modify-named", func() {
		replaceIn(t, shapeFile, "func NamedInExam() int { return 1 }", "func NamedInExam() int { return 9 }")
	}, true)
	checkChanged("modify-unnamed", func() {
		replaceIn(t, shapeFile, "func UnnamedHelper() int { return 2 }", "func UnnamedHelper() int { return 8 }")
	}, false)
	checkChanged("rename-named", func() {
		replaceIn(t, shapeFile, "func NamedInExam() int { return 1 }", "func RenamedAway() int { return 1 }")
	}, true)
	checkChanged("delete-method", func() {
		replaceIn(t, shapeFile, "func (s Square) Perimeter() float64 { return 4 * s.Side }\n", "")
	}, true)
	checkChanged("delete-unnamed", func() {
		replaceIn(t, shapeFile, "func UnnamedHelper() int { return 2 }\n", "")
	}, false)

	s1, err := sourceStamp(root)
	if err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, root, examFile, questions+"codegraph query shape.Orphan | contains: shape.Orphan | src: fixture, 2026-11-05\n")
	s2, err := sourceStamp(root)
	if err != nil {
		t.Fatal(err)
	}
	if s1 == s2 {
		t.Errorf("editing %s did not change sourceStamp", examFile)
	}
}

// Kural: sınav kenarı (names_in_exam) başlığa ve pakete gitmez. Başlık metni ve paket yolu sınavın sözcüğüyle çakışır
// (`codegraph path internal/stdlib/ml internal/compute/gpu` iki paketin yolunu yazar), ama sınav onları sınamaz (gaps,
// "`examNodeKeys`'in süzgeci testsiz": tür süzgeci kaldırılınca iki koruyan test de geçiyordu).
func TestExamLinksSkipHeadingsAndPackages(t *testing.T) {
	for _, n := range []*Node{{Kind: kindHeading, Name: "Kurulum", File: "README.md"}, {Kind: kindPackage, Name: "internal/compute/gpu"}} {
		if keys := examNodeKeys(n); keys != nil {
			t.Errorf("a %s is not an exam key: %q", n.Kind, keys)
		}
	}
	_, _, g := repoGraph(t)
	kinds := map[string]string{}
	for i := range g.Nodes {
		kinds[g.Nodes[i].ID] = g.Nodes[i].Kind
	}
	for _, e := range g.Edges {
		if k := kinds[e.To]; e.Kind == edgeNamesInExam && (k == kindHeading || k == kindPackage) {
			t.Errorf("names_in_exam reaches a %s: %s (%s:%d)", k, e.To, e.File, e.Line)
		}
	}
}

// Kural: silinen satırdaki sözcük karışık harfli mi diye bakılırken ilk harf rune olarak kesilir (gaps, "`çok` sınavı
// seçiyor": `tok[1:]` bayt kesiyordu, ilk harfi ASCII dışı olan küçük harfli sözcük karışık harfli sayılıyordu).
func TestADeletedLowercaseWordWithANonASCIIFirstLetterIsNoExamWord(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, examFile, "codegraph query çok fooBar | contains: x | src: fixture, 2026-11-09\n")
	ix := newIndex(&Graph{})
	ix.root = root
	for _, c := range []struct {
		line string
		want bool
	}{
		{"-\tx := çok", false},
		{"-\ty := fooBar", true},
	} {
		ts := newTouchSet()
		ts.hunks = []hunk{{file: "app/app.go", changedLines: []string{c.line}}}
		if got := ix.deletedHunkNamesExam(ts); got != c.want {
			t.Errorf("deleted line %q selects the exam: %v, want %v", c.line, got, c.want)
		}
	}
}

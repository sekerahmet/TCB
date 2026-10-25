package main

import (
	"bytes"
	"fmt"
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
	if code := run([]string{"path", "--graph", file, "--no-rebuild", "p.a", "p.b"}, &so, &se); code != exitNotFound || !strings.Contains(se.String(), "no directed path") {
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

// Kural: test ile hedef arasındaki zincir tek kurala bağlıdır (test_reach.go): en çok testMaxSteps adım ve en çok
// testMaxHops paket sınırı. Sınırın içi bulunur, bir adım ya da bir sınır ötesi bulunmaz.
func TestIndirectChainIsLimitedByOneRule(t *testing.T) {
	chain := func(n int, dirOf func(i int) string) *index {
		g := &Graph{Nodes: []Node{{ID: "t", Kind: kindFunc, Name: "p.TestX", File: "p_test.go"}}}
		prev := "t"
		for i := 1; i <= n; i++ {
			id := "f" + string(rune('a'+i))
			g.Nodes = append(g.Nodes, Node{ID: id, Kind: kindFunc, Name: "p." + id, File: dirOf(i) + "/x.go"})
			g.Edges = append(g.Edges, Edge{From: prev, To: id, Kind: edgeCalls, Confidence: extracted})
			prev = id
		}
		return newIndex(g)
	}
	same := func(int) string { return "." }
	last := func(n int) *Node { return &Node{ID: "f" + string(rune('a'+n))} }
	ix := chain(testMaxSteps, same)
	if got := ix.indirectTests(last(testMaxSteps)); len(got) != 1 || got[0].steps != testMaxSteps {
		t.Errorf("%d steps away must be found: %+v", testMaxSteps, got)
	}
	ix = chain(testMaxSteps+1, same)
	if got := ix.indirectTests(last(testMaxSteps + 1)); len(got) != 0 {
		t.Errorf("%d steps away must not be found: %+v", testMaxSteps+1, got)
	}
	// Paket sınırı: test "." klasöründe, zincir her adımda başka klasöre geçer.
	across := func(i int) string { return "d" + string(rune('a'+i)) }
	ix = chain(testMaxHops, across)
	if got := ix.indirectTests(last(testMaxHops)); len(got) != 1 {
		t.Errorf("%d package boundary must be found: %+v", testMaxHops, got)
	}
	ix = chain(testMaxHops+1, across)
	if got := ix.indirectTests(last(testMaxHops + 1)); len(got) != 0 {
		t.Errorf("%d package boundaries must not be found: %+v", testMaxHops+1, got)
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
		testkit.MustContain(t, out, "TR_kw_if [name]", "names: eger · if", "Go constant: codes.KwIf [const]",
			"registered or used in Go", "shape.table at shape/shape.go:", "shape.runIf [shape/shape.go:")
	}
}

func TestWhoUsesByPackage(t *testing.T) {
	code, out, _ := cli(t, "who-uses", "messages.H0001", "--by-package")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	testkit.MustContain(t, out, "users by package (1 packages):", "shape: 2 functions, 3 use sites")
}

// Kural: tür düğümünde alanlar metotlardan ayrı bir grupta ve sınırdan bağımsız hep gösterilir; metotlar ayrı grupta
// sınırlı kalır (yapının biçimi bilinecek ilk şeydir, 34 metodun altında kaybolmamalı).
func TestQueryShowsFieldsApartFromMethodsRegardlessOfTheLimit(t *testing.T) {
	var src strings.Builder
	src.WriteString("package p\n\ntype C struct {\n\t// root genel kapsamdır.\n\troot int\n\tsource *C\n}\n\n")
	for i := 0; i < 20; i++ {
		src.WriteString("func (c *C) m" + string(rune('a'+i)) + "() {}\n")
	}
	root := auditModule(t, map[string]string{"p/p.go": src.String()})
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"query", "p.C", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("query: %d %s", code, errb.String())
	}
	got := out.String()
	fields, methods, _ := strings.Cut(got, "-> contains (20)")
	if !strings.Contains(fields, "fields (2, always shown)") || !strings.Contains(fields, "p.C.root") || !strings.Contains(fields, "p.C.source  *p.C") {
		t.Errorf("both fields above the methods, without --all:\n%s", got)
	}
	if !strings.Contains(methods, "more") || strings.Contains(methods, "p.C.root") {
		t.Errorf("methods stay limited and carry no fields:\n%s", got)
	}
}

// Kural: metin aramasında önce bütün sözcük eşleşir (Türkçe harf katlamasıyla, sözcük sınırında): "ölü" "bölüm"ün içinde
// değil, "ölü kod"da bulunur; "ışık" "IŞIK"ı bulur; alt çizgi sözcüğün parçasıdır.
func TestHasWordMatchesWholeTurkishWords(t *testing.T) {
	for _, c := range []struct {
		text, q string
		want    bool
	}{
		{"ölü kod denetimi", "ölü", true},
		{"Bölümler", "ölü", false},
		{"bölüm_ve_kalan", "bölüm", false},
		{"IŞIK yanar", "ışık", true},
		{"İstanbul'da", "istanbul", true},
		{"(ölü)", "ölü", true},
		{"", "ölü", false},
	} {
		if got := hasWord(c.text, c.q); got != c.want {
			t.Errorf("hasWord(%q, %q) = %v, want %v", c.text, c.q, got, c.want)
		}
	}
}

// Kural: "did you mean" en yakın altı öneriyi yazar, kalanını öbür çıktılar gibi "... N more" ile bildirir; --all
// hepsini yazar (bağımsız denetim O3: liste sessizce altıda kesiliyordu).
func TestSuggestionsSayHowManyMore(t *testing.T) {
	g := &Graph{}
	for i := 1; i <= 8; i++ {
		g.Nodes = append(g.Nodes, Node{ID: fmt.Sprintf("n%d", i), Kind: kindFunc, Name: fmt.Sprintf("p.Abcdef%d", i)})
	}
	ix := newIndex(g)
	var short, all strings.Builder
	ix.printSuggestions(&short, "abcdef", nil, false)
	ix.printSuggestions(&all, "abcdef", nil, true)
	if strings.Count(short.String(), "[func]") != 6 || !strings.Contains(short.String(), "... 2 more") {
		t.Errorf("six and the rest counted:\n%s", short.String())
	}
	if strings.Count(all.String(), "[func]") != 8 || strings.Contains(all.String(), "more") {
		t.Errorf("--all lists every suggestion:\n%s", all.String())
	}
}

// Kural: küçük harfli sorgu tam harfle bir düğüme çözülünce yalnızca harfi farklı eşleşmeler de söylenir (cevap
// değişmez, belirsizlik çıkmaz); büyük harfli sorgu harfi seçmiştir (bağımsız denetim grup 7: `query identifier`
// ast.Identifier'ı hiç anmıyordu, yanlış bir "yok").
func TestOtherCaseMatchesAreMentioned(t *testing.T) {
	ix := newIndex(&Graph{Nodes: []Node{
		{ID: "go:func:m/interp.identifier", Kind: kindFunc, Name: "interp.identifier"},
		{ID: "go:type:m/ast.Identifier", Kind: kindType, Name: "ast.Identifier"},
		{ID: "go:func:m/ast.identifierList", Kind: kindFunc, Name: "ast.identifierList"},
	}})
	chosen := ix.byID["go:func:m/interp.identifier"]
	if got := ix.otherCase("identifier", chosen, nil); len(got) != 1 || !strings.Contains(got[0], "ast.Identifier [type]") {
		t.Errorf("other case of identifier: %v", got)
	}
	if got := ix.otherCase("Identifier", ix.byID["go:type:m/ast.Identifier"], nil); got != nil {
		t.Errorf("a capitalised query chose its case: %v", got)
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

var testPathFiles = map[string]string{
	"lib/a.tr":      "fonksiyon a() {\n}\n",
	"lib/b.tr":      "fonksiyon b() {\n}\n",
	"tools/run.tr":  "fonksiyon r() {\n}\n",
	"docs/guide.md": "# Guide\n",
	"app/app.go":    "package app\n",
	"app/app_test.go": `package app

import (
	"path/filepath"
	"testing"

	"example.com/audit/internal/reporoot"
)

func TestRunsScript(t *testing.T) { _ = reporoot.Path("tools/run.tr") }

func libFiles() []string {
	files, _ := filepath.Glob(reporoot.Path("lib", "*.tr"))
	return files
}

func TestCoversLibrary(t *testing.T) { _ = libFiles() }

func TestReadsDoc(t *testing.T) { _ = []string{"README.md", "docs/guide.md"} }

func TestPatternFixture(t *testing.T) { _ = "lib/*.tr" }

func helperNobodyCalls() string { return "tools/run.tr" }
`,
	"internal/reporoot/root.go": "package reporoot\n\nfunc Path(parts ...string) string { return \"\" }\n",
}

// Kural: bir testin dize sabitindeki depo yolu (tek yol, Path/Join ile birleşen parçalar, Glob kalıbı) var olan
// dosya ya da belgeye INFERRED `tests` kenarı verir; yol bir yardımcıdaysa onu çağıran test bağlanır; tek
// başına joker kalıbı (sınama verisi), çıplak dosya adı ve hiçbir testin çağırmadığı yardımcı bağlanmaz.
func TestGoTestsLinkToTheRepositoryFilesTheyUse(t *testing.T) {
	g, err := buildGraph(auditModule(t, testPathFiles))
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	mustEdge(t, g, app+"TestRunsScript", "tr:file:tools/run.tr", edgeTests, inferred)
	mustEdge(t, g, app+"TestCoversLibrary", "tr:file:lib/a.tr", edgeTests, inferred)
	mustEdge(t, g, app+"TestCoversLibrary", "tr:file:lib/b.tr", edgeTests, inferred)
	var guide string
	for _, n := range g.Nodes {
		if n.Kind == kindDoc && n.File == "docs/guide.md" {
			guide = n.ID
		}
	}
	if guide == "" {
		t.Fatal("no doc node for docs/guide.md")
	}
	mustEdge(t, g, app+"TestReadsDoc", guide, edgeTests, inferred)
	for _, e := range g.Edges {
		if e.Kind != edgeTests || !strings.HasPrefix(e.To, "tr:file:") && e.To != guide {
			continue
		}
		switch {
		case e.From == app+"TestPatternFixture":
			t.Errorf("a bare glob literal is test data, not a repository path: %+v", e)
		case e.From == app+"helperNobodyCalls":
			t.Errorf("a helper no test calls is not a test: %+v", e)
		case e.To == "tr:file:lib/a.tr" && e.From == app+"TestRunsScript":
			t.Errorf("unrelated link: %+v", e)
		}
	}
}

var testNoteFiles = map[string]string{
	"lib/a.tr": "fonksiyon a() {\n}\n",
	"app/doc_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/reporoot"
)

// Belgeli kural: lib/a.tr yazılışı korur.
// İkinci satır gösterilmez.
func TestWithDoc(t *testing.T) { _ = reporoot.Path("lib/a.tr") }
`,
	"app/head_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/reporoot"
)

// Baş yorum: dosyanın kuralı.
// Devamı gösterilmez.
const unrelated = 1

func TestWithoutDoc(t *testing.T) { _ = reporoot.Path("lib/a.tr") }
`,
	"app/none_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/reporoot"
)

func TestNoComment(t *testing.T) { _ = reporoot.Path("lib/a.tr") }
`,
	"internal/reporoot/root.go": "package reporoot\n\nfunc Path(parts ...string) string { return \"\" }\n",
}

// Kural: tests-for her Go testinin altına koruduğu kuralın ilk satırını yazar: önce testin doc yorumu, yoksa test
// dosyasının baş yorumu (içe aktarmalardan sonraki ilk // bloğu); yorumsuz testte not yoktur.
func TestTestsForShowsTheRuleLineOfEachGoTest(t *testing.T) {
	root := auditModule(t, testNoteFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "lib/a.tr", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{
		"TestWithDoc [func] at app/doc_test.go:11  [INFERRED]\n      Belgeli kural: lib/a.tr yazılışı korur.\n",
		"TestWithoutDoc [func] at app/head_test.go:13  [INFERRED]\n      Baş yorum: dosyanın kuralı.\n",
		"TestNoComment [func] at app/none_test.go:9  [INFERRED]\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tests-for lacks %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "İkinci satır") || strings.Contains(got, "Devamı") || strings.Contains(got, "TestNoComment [func] at app/none_test.go:9  [INFERRED]\n      ") {
		t.Errorf("only the first comment line, and none for a test without a comment:\n%s", got)
	}
}

var embedFiles = map[string]string{
	"internal/lang/codes/tr.json": "{\n  \"TR_kw_if\": { \"name\": \"eger\", \"family\": \"keyword\" }\n}\n",
	"internal/lang/codes/codes.go": `package codes

import _ "embed"

//go:embed tr.json
var trJSON []byte

var all = parse(trJSON)

func parse(b []byte) []string { return []string{string(b)} }

// All bütün girdileri verir.
func All() []string { return all }

// Name tek bir girdiyi verir.
func Name(i int) string { return all[i] }
`,
	"app/cover_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/lang/codes"
)

// Kural: her girdi gezilir.
func TestWalksEveryEntry(t *testing.T) {
	for _, e := range codes.All() {
		_ = e
	}
}

// Kural: tek girdiye bakar.
func TestLooksOneUp(t *testing.T) { _ = codes.Name(0) }
`,
}

// Kural: gömülen veri dosyası düğümdür; gömen değişkenden `embeds`, dosyayı kullanan test(ler)den `tests` gelir;
// dosyadaki bütün girdileri gezen test (range) ayrıca `walks` alır, tek girdiye bakan almaz. Ad ve ileti kodları
// tanımlandıkları dosyadan `contains` alır; `tests-for <kod>` kodun dosyasını gezen testi kural satırıyla gösterir.
func TestEmbeddedDataFilesLinkToTheTestsThatWalkThem(t *testing.T) {
	root := auditModule(t, embedFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const file = "file:internal/lang/codes/tr.json"
	const app = "go:func:example.com/audit/app."
	mustEdge(t, g, "go:var:example.com/audit/internal/lang/codes.trJSON", file, edgeEmbeds, inferred)
	mustEdge(t, g, app+"TestWalksEveryEntry", file, edgeTests, inferred)
	mustEdge(t, g, app+"TestLooksOneUp", file, edgeTests, inferred)
	mustEdge(t, g, app+"TestWalksEveryEntry", file, edgeWalks, inferred)
	if edge(g, app+"TestLooksOneUp", file, edgeWalks) != nil {
		t.Error("a test that looks one entry up does not walk the list")
	}
	mustEdge(t, g, file, "name:TR_kw_if", edgeContains, extracted)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "TR_kw_if", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	walks, _, _ := strings.Cut(out.String(), "<- tests (")
	if !strings.Contains(walks, "tests that walk the whole list (1") || !strings.Contains(walks, "TestWalksEveryEntry") ||
		!strings.Contains(walks, "Kural: her girdi gezilir.") || strings.Contains(walks, "TestLooksOneUp") {
		t.Errorf("tests-for TR_kw_if must list only the walking test, with its rule line first:\n%s", out.String())
	}
}

var fileTestFiles = map[string]string{
	"docs/rules.md":  "# Rules\n",
	"tools/check.sh": "#!/bin/sh\n",
	"README.md":      "# mini\n\n**Kural metni.** Kural metni burada. Denetim: `TestCites`.\n",
	"app/app.go":     "package app\n",
	"app/app_test.go": `package app

import (
	"path/filepath"
	"testing"
)

func TestReads(t *testing.T) { _ = filepath.Join("tools", "check.sh") }

// TestCites docs/rules.md'deki kuralı sınar; app.go'yu adıyla anar.
func TestCites(t *testing.T) { t.Error("kural için bkz. tools/check.sh") }

func TestQuiet(t *testing.T) { _ = "docs/missing.md" }
`,
	"app/header_test.go": `package app

import "testing"

// Bu dosyanın kuralları: docs/rules.md.

func TestHeader(t *testing.T) {}
`,
}

// Kural: depoda izlenen her dosyanın yolu tests-for'da o dosyaya çözülür ve cevap okuyanları, anan testleri ve (belgede)
// kurallarını koruyan testleri ayrı gruplarda verir: yolu bir sabitle kuran test (`filepath.Join("tools", "check.sh")`)
// "tests" grubunda; yolu yorumunda ya da iletisinde anan test "cite" grubunda (aynı test okuyorsa bir kez, okuyan olarak);
// dosyanın başındaki kural yorumu dosyanın bütün testlerinindir; yorumdaki çıplak dosya adı yalnızca yorumun klasöründen
// çözülür; README'nin `Denetim:` kuralının testi belgenin cevabındadır. Hiçbir şeyin bağlanmadığı belgede cevap neye
// bakıldığını söyler (TCB 2026-10-25, 1. ve 2. madde: tam denetim betiğinin yolu belirsizdi, geliştirici
// belgesini anan testler görünmüyordu).
func TestTestsForAFileListsItsReadersCitersAndRuleGuards(t *testing.T) {
	root := auditModule(t, fileTestFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	mustEdge(t, g, app+"TestReads", "file:tools/check.sh", edgeTests, inferred)
	mustEdge(t, g, app+"TestCites", "doc:docs/rules.md", edgeCites, inferred)
	mustEdge(t, g, app+"TestCites", "file:tools/check.sh", edgeCites, inferred)
	mustEdge(t, g, app+"TestCites", "go:file:app/app.go", edgeCites, inferred)
	mustEdge(t, g, app+"TestHeader", "doc:docs/rules.md", edgeCites, inferred)
	if e := edge(g, app+"TestReads", "doc:docs/rules.md", edgeCites); e != nil {
		t.Errorf("a file comment cites for the tests of its own file only: %+v", e)
	}
	ix := newIndex(g)
	ix.root = root
	answer := func(q string) string {
		t.Helper()
		var out, errb strings.Builder
		n, code := ix.resolve(q, &queryOpts{stdout: &out, stderr: &errb})
		if code != exitOK {
			t.Fatalf("resolve %s: exit %d: %s%s", q, code, out.String(), errb.String())
		}
		ix.printTestsFor(&out, n, 0)
		return out.String()
	}
	check := answer("tools/check.sh")
	readers, cited, _ := strings.Cut(check, "tests that cite it")
	if !strings.Contains(readers, "app.TestReads") || !strings.Contains(cited, "app.TestCites") || strings.Contains(cited, "TestReads") {
		t.Errorf("tests-for tools/check.sh: the reader under tests, the citing test apart:\n%s", check)
	}
	rules := answer("docs/rules.md")
	if !strings.Contains(rules, "no tests read it") || !strings.Contains(rules, "app.TestCites") || !strings.Contains(rules, "app.TestHeader") {
		t.Errorf("tests-for docs/rules.md: the citing tests, and that none reads it:\n%s", rules)
	}
	readme := answer("README.md")
	if !strings.Contains(readme, "guard the rules it states") || !strings.Contains(readme, "Kural metni") || !strings.Contains(readme, "app.TestCites") {
		t.Errorf("tests-for README.md: its rule and the test that guards it:\n%s", readme)
	}
	if got := answer("./app/app.go"); !strings.HasPrefix(got, "app/app.go [file]") {
		t.Errorf("a ./ path resolves to its file:\n%s", got)
	}
	testkit.WriteFile(t, root, "docs/empty.md", "# Empty\n")
	g2, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix = newIndex(g2)
	ix.root = root
	if got := answer("docs/empty.md"); !strings.Contains(got, "no tests found (searched:") {
		t.Errorf("an unlinked doc says what was searched:\n%s", got)
	}
}

// Kural: git'in izlemediği bir dosyanın yolu ad aramasına düşmez (yolu metninde anan bildirimler "belirsiz" diye
// gelmez); cevap grafiğin ona hiç bakmadığını söyler ve "bulunamadı" çıkış koduyla biter.
func TestTestsForAnUntrackedFileSaysNothingWasSearched(t *testing.T) {
	root := gitFixture(t)
	graph := buildInto(t, root)
	testkit.WriteFile(t, root, "notes/draft.sh", "echo shape.Describe\n")
	code, out, errOut := commandIn("tests-for", root, graph, "--no-rebuild", "notes/draft.sh")
	if code != exitNotFound || out != "" || !strings.Contains(errOut, "git does not track") {
		t.Errorf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
}

var rootPathFiles = map[string]string{
	"README.md":     "# mini\n",
	"SECURITY.md":   "# Security\n",
	"docs/a.md":     "# A\n",
	"docs/b.md":     "# B\n",
	"app/README.md": "# app\n",
	"app/app.go":    "package app\n",
	"app/doc_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/reporoot"
)

func docs() []string {
	list := []string{"README.md", "docs/a.md"}
	_ = []string{"add", "docs/b.md", "SECURITY.md"}
	_ = []string{"SECURITY.md", "lib/missing.md"}
	for _, p := range []string{"docs/*.md"} {
		_ = reporoot.Path(p)
	}
	return list
}

func TestDocs(t *testing.T) { _ = docs() }

func TestSecurity(t *testing.T) { _ = reporoot.Path("SECURITY.md") }

func TestFixture(t *testing.T) { _ = []string{"README.md", "a.tr"} }
`,
	"internal/reporoot/root.go": "package reporoot\n\nfunc Path(parts ...string) string { return \"\" }\n",
}

// Kural: depo kökünden göreli yollar çıplak adla da bağlanır, ama yalnızca kökten: reporoot'un yol işlevine verilen tek
// parça (fikstürde TestSecurity), depoda bulunan "/" içeren bir yolun yanındaki liste öğesi (docs içindeki liste); yol
// olmayan öğe taşıyan ya da yolları depoda olmayan liste bağlanmaz; dize listesi üzerinde dönen değişkenle kurulan yol
// listenin her değeridir (`range []string{"docs/*.md"}`). Yalnızca adlardan oluşan liste fikstürdür (TestFixture),
// klasördeki aynı adlı belge bağlanmaz (TCB 2026-10-25, 2. madde: belge testlerinin listesi kök belgesini ve docs
// belgelerini okuyordu, tests-for "no tests found" diyordu).
func TestRootRelativePathsLinkFromTheRootOnly(t *testing.T) {
	g, err := buildGraph(auditModule(t, rootPathFiles))
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	mustEdge(t, g, app+"TestDocs", "doc:README.md", edgeTests, inferred)
	mustEdge(t, g, app+"TestDocs", "doc:docs/a.md", edgeTests, inferred)
	mustEdge(t, g, app+"TestDocs", "doc:docs/b.md", edgeTests, inferred)
	mustEdge(t, g, app+"TestSecurity", "doc:SECURITY.md", edgeTests, inferred)
	for _, e := range g.Edges {
		if e.Kind != edgeTests {
			continue
		}
		if e.To == "doc:app/README.md" || e.From == app+"TestFixture" && strings.HasPrefix(e.To, "doc:") ||
			e.From == app+"TestDocs" && e.To == "doc:SECURITY.md" {
			t.Errorf("a bare name links from the root, and a list of bare names is a fixture: %+v", e)
		}
	}
}

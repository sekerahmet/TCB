package main

import (
	"strings"
	"testing"
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
	"internal/lang/codes/tr.json": "{\n  \"A001\": { \"ad\": \"eger\", \"aile\": \"keyword\" }\n}\n",
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
	mustEdge(t, g, file, "name:A001", edgeContains, extracted)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "A001", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	walks, _, _ := strings.Cut(out.String(), "<- tests (")
	if !strings.Contains(walks, "tests that walk the whole list (1") || !strings.Contains(walks, "TestWalksEveryEntry") ||
		!strings.Contains(walks, "Kural: her girdi gezilir.") || strings.Contains(walks, "TestLooksOneUp") {
		t.Errorf("tests-for A001 must list only the walking test, with its rule line first:\n%s", out.String())
	}
}

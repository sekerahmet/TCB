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

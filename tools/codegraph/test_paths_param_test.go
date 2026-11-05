package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: test dosyasında (veya testing alan yardımcıda) dosya okuma çağrısına verilen çıplak dosya adı (dizgi, sabit,
// yerel değişken) çağıran testin paket klasöründeki dosyaya bağlanır; kökteki aynı adlı dosyaya bağlanmaz, `.` klasörü ve
// üretim kodundaki okuma bağlanmaz. Yorumda "/" sonrası gelen yol kuyruğu (`docs/README.md`) paket dosyasını anmaz, çıplak
// dosya adı yorumu (`app.go`) anar.
func TestBareReadCallsAndSlashTailComments(t *testing.T) {
	files := map[string]string{
		"README.md":          "# root\n",
		"docs/README.md":     "# docs\n",
		"app/README.md":      "# app\n",
		"app/config.json":    "{}\n",
		"app/local.json":     "{}\n",
		"app/app.go":         "package app\n\nimport \"os\"\n\nfunc ProdRead() { _, _ = os.ReadFile(\"README.md\") }\n",
		"sub/sub.go":         "package sub\n",
		"testutil/helper.go": "package testutil\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc ReadPkgReadme(t *testing.T) { _, _ = os.ReadFile(\"README.md\") }\n",
		"app/app_test.go": `package app

import (
	"os"
	"testing"

	"example.com/audit/testutil"
)

const cfgFile = "config.json"

// Kural docs/README.md icindedir.
func TestDirectBareRead(t *testing.T) {
	_, _ = os.ReadFile("README.md")
	_, _ = os.ReadDir(".")
}

func TestConstAndLocalBareRead(t *testing.T) {
	local := "local.json"
	_, _ = os.Open(cfgFile)
	_, _ = os.Stat(local)
}

func TestCrossPkgHelperBareRead(t *testing.T) {
	testutil.ReadPkgReadme(t)
}

// Bkz. app.go ve docs/README.md.
func TestCommentCiteOnlyBareNotSlashTail(t *testing.T) {}
`,
		"sub/sub_test.go": `package sub

import (
	"os"
	"testing"

	"example.com/audit/app"
)

func TestMissingInPkgDoesNotLinkRoot(t *testing.T) {
	_, _ = os.ReadFile("README.md")
	app.ProdRead()
}
`,
	}
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	const (
		appPkg = "go:func:example.com/audit/app."
		subPkg = "go:func:example.com/audit/sub."
	)
	mustEdge(t, g, appPkg+"TestDirectBareRead", "doc:app/README.md", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestConstAndLocalBareRead", "file:app/config.json", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestConstAndLocalBareRead", "file:app/local.json", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestCrossPkgHelperBareRead", "doc:app/README.md", edgeTests, inferred)
	mustEdge(t, g, appPkg+"TestCommentCiteOnlyBareNotSlashTail", "go:file:app/app.go", edgeCites, inferred)
	mustEdge(t, g, appPkg+"TestCommentCiteOnlyBareNotSlashTail", "doc:docs/README.md", edgeCites, inferred)

	if e := edge(g, subPkg+"TestMissingInPkgDoesNotLinkRoot", "doc:README.md", edgeTests); e != nil {
		t.Errorf("bare read in sub must not link root README.md: %+v", e)
	}
	if e := edge(g, subPkg+"TestMissingInPkgDoesNotLinkRoot", "doc:app/README.md", edgeTests); e != nil {
		t.Errorf("prod bare read must not link app/README.md: %+v", e)
	}
	if e := edge(g, appPkg+"TestCommentCiteOnlyBareNotSlashTail", "doc:app/README.md", edgeCites); e != nil {
		t.Errorf("slash-tail comment docs/README.md must not cite app/README.md: %+v", e)
	}
	for _, e := range g.Edges {
		if e.From == appPkg+"TestDirectBareRead" && e.Kind == edgeTests && e.To != "doc:app/README.md" && !strings.HasPrefix(e.To, "go:pkg:") {
			t.Errorf("ReadDir(\".\") or bare read created unexpected edge: %+v", e)
		}
	}

	ix := newIndex(g)
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["doc:app/README.md"], 20)
	testkit.MustContain(t, out.String(), "app.TestDirectBareRead", "app.TestCrossPkgHelperBareRead")
	if strings.Contains(out.String(), "TestCommentCiteOnlyBareNotSlashTail") {
		t.Errorf("tests-for app/README.md must not cite slash-tail comment:\n%s", out.String())
	}
}

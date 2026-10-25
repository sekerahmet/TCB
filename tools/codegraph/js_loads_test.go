package main

import (
	"strings"
	"testing"
)

var jsLoadFiles = map[string]string{
	"ui/view.js":           "const helper = require('./helper');\nimport { x } from \"../shared/index.js\";\n",
	"ui/helper.js":         "module.exports = {};\n",
	"ui/media/table.js":    "var table = 1;\n",
	"ui/unused.js":         "var x = require('vscode');\n",
	"shared/index.js":      "export const x = 1;\n",
	"ui/test/view_test.js": "const view = require('../view');\nconst fs = require('fs');\nfs.readFileSync(path.join(__dirname, '..', 'media', 'table.js'), 'utf8');\n",
	"app/app.go":           "package app\n",
	"app/app_test.go": `package app

import (
	"path/filepath"
	"testing"
)

func TestRunsJS(t *testing.T) { _ = filepath.Join("ui", "test", "view_test.js") }
`,
}

// Kural: bir JavaScript dosyasının göreli yüklediği depo dosyası (`require('./x')`, `import ... from '../x/index.js'`,
// bütün parçaları metin olan `path.join(__dirname, ...)`) `loads` kenarı alır; paket adı (`vscode`, `fs`) almaz.
// tests-for bir dosyayı yükleyen test dosyasını koşan Go testini, yükleme zinciriyle verir: TestRunsJS view_test.js'yi
// koşar, o view.js'yi, view.js helper.js'yi yükler; "via" testin koştuğu dosyadır (TCB 2026-10-25, 4. madde:
// eklentinin kodlu görünüm modülü için tests-for eklentinin testlerini görmüyordu).
func TestTestsForAJavaScriptFileFollowsTheFilesThatLoadIt(t *testing.T) {
	root := auditModule(t, jsLoadFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	mustEdge(t, g, "file:ui/test/view_test.js", "file:ui/view.js", edgeLoads, extracted)
	mustEdge(t, g, "file:ui/test/view_test.js", "file:ui/media/table.js", edgeLoads, extracted)
	mustEdge(t, g, "file:ui/view.js", "file:ui/helper.js", edgeLoads, extracted)
	mustEdge(t, g, "file:ui/view.js", "file:shared/index.js", edgeLoads, extracted)
	for _, e := range g.Edges {
		if e.Kind == edgeLoads && e.From == "file:ui/unused.js" {
			t.Errorf("a package name is not a repository file: %+v", e)
		}
	}
	mustEdge(t, g, "go:func:example.com/audit/app.TestRunsJS", "file:ui/test/view_test.js", edgeTests, inferred)
	ix := newIndex(g)
	for file, want := range map[string]string{
		"ui/view.js":   "2 steps via ui/test/view_test.js",
		"ui/helper.js": "3 steps via ui/test/view_test.js",
	} {
		var out strings.Builder
		ix.printTestsFor(&out, ix.byID["file:"+file], 0)
		if !strings.Contains(out.String(), "app.TestRunsJS") || !strings.Contains(out.String(), want) {
			t.Errorf("tests-for %s: want TestRunsJS, %q:\n%s", file, want, out.String())
		}
	}
	if d := ix.testDistances(ix.byID["file:ui/helper.js"]); d["go:func:example.com/audit/app.TestRunsJS"] != 3 {
		t.Errorf("changed uses the same chain (testDistances): %v", d)
	}
}

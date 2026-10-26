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

// Kural: JavaScript testi bir veri dosyasını çoğu zaman bir dizin sabitiyle ya da bir yardımcıyla okur:
// `const ROOT = path.join(__dirname, '..')`, `path.join(ROOT, 'l10n', 'x.json')`, zincirli sabit (`FILE =
// path.join(FOLDER, 'deneme.tr')`) ve `readJSON = (...parts) => ... path.join(ROOT, ...parts)` ile `readJSON('l10n',
// 'x.json')`. Bütün parçaları metin olan her biçim `loads` kenarıdır; dosyayı okuyan JS testini koşan Go testi tests-for'da
// görünür (eksikler listesi, ana oturum 2026-10-26: `tests-for editors/vscode/l10n/bundle.l10n.tr.json` test yok diyordu).
func TestAJavaScriptTestReadsAFileThroughADirectoryConstantOrAHelper(t *testing.T) {
	files := map[string]string{}
	for k, v := range jsLoadFiles {
		files[k] = v
	}
	files["ui/l10n/bundle.json"] = "{}\n"
	files["ui/media/page.html"] = "<p></p>\n"
	files["ui/deneme.tr"] = "yazdır(1)\n"
	files["ui/test/l10n_test.js"] = "const ROOT = path.join(__dirname, '..');\n" +
		"const MEDIA = path.join(ROOT, 'media');\n" +
		"const FILE = path.join(ROOT, \"deneme.tr\");\n" +
		"const readJSON = (...parts) => JSON.parse(fs.readFileSync(path.join(ROOT, ...parts), 'utf8'));\n" +
		"const b = readJSON('l10n', 'bundle.json');\n" +
		"const html = fs.readFileSync(path.join(MEDIA, 'page.html'), 'utf8');\n" +
		"const other = path.join(unknown, 'helper.js');\n"
	files["app/app_test.go"] = "package app\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n)\n\n" +
		"func TestRunsJS(t *testing.T) { _ = filepath.Join(\"ui\", \"test\", \"view_test.js\"); _ = filepath.Join(\"ui\", \"test\", \"l10n_test.js\") }\n"
	g, err := buildGraph(auditModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"ui/l10n/bundle.json", "ui/media/page.html", "ui/deneme.tr"} {
		mustEdge(t, g, "file:ui/test/l10n_test.js", "file:"+target, edgeLoads, extracted)
	}
	for _, e := range g.Edges {
		if e.Kind == edgeLoads && e.From == "file:ui/test/l10n_test.js" && e.To == "file:ui/helper.js" {
			t.Errorf("a path built from an unknown variable is not seen: %+v", e)
		}
	}
	_, _, rg := repoGraph(t)
	ix := newIndex(rg)
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["file:editors/vscode/l10n/bundle.l10n.tr.json"], 0)
	if !strings.Contains(out.String(), "e2e.TestExtensionJSTests") {
		t.Errorf("tests-for editors/vscode/l10n/bundle.l10n.tr.json lacks the Go test running l10n_test.js:\n%s", out.String())
	}
}

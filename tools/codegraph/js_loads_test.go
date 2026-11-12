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
	files["ui/deneme.tr"] = "TR_print(1)\n"
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

// Kural: Python (`.py`) betiklerinde `__file__` tabanlı `os.path.abspath` / `os.path.dirname` / `os.path.join` modül
// sabitleri (`ROOT`, `BENCH`, `MODEL_TR`) ve bütün parçaları sabit olan satır içi `os.path.join(...)` ifadeleri hedef
// düğümün gerçek kimliğine (`tr:file:...`, `doc:...`, `file:...`) `loads` kenarı çeker; bilinmeyen değişkenle kurulan
// `os.path.join(tmp, "model.tr")` ve dış paket `import json` bağ kurmaz (Madde 1, DENETİM 510 N4).
func TestPythonRelativeLoadsAndTrackedTargetNodeIDs(t *testing.T) {
	files := map[string]string{
		"examples/tr/fixture_model.tr":         "# TR 4\nTR_print(1)\n",
		"docs/fixture_note.md":                 "# Note\n",
		"tools/bench/py/fixture_char_model.py": "x = 1\n",
		"tools/bench/fixture_unused.tr":        "# TR 4\nTR_print(2)\n",
		"tools/bench/fixture_run.py": "import json, os\n" +
			"ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))\n" +
			"BENCH = os.path.join(ROOT, \"tools\", \"bench\")\n" +
			"MODEL_TR = os.path.join(\"examples\", \"tr\", \"fixture_model.tr\")\n" +
			"DOC_MD = os.path.join(ROOT, \"docs\", \"fixture_note.md\")\n" +
			"def load(tmp):\n" +
			"    open(os.path.join(ROOT, MODEL_TR))\n" +
			"    open(os.path.join(BENCH, 'py', 'fixture_char_model.py'))\n" +
			"    open(os.path.join(tmp, \"fixture_unused.tr\"))\n",
		"tools/bench/fixture_run_test.go": "package bench\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
			"func TestRunsPy(t *testing.T) { _ = exec.Command(\"python\", \"-I\", \"fixture_run.py\") }\n",
	}
	root := auditModule(t, files)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{
		"tr:file:examples/tr/fixture_model.tr",
		"doc:docs/fixture_note.md",
		"file:tools/bench/py/fixture_char_model.py",
	} {
		mustEdge(t, g, "file:tools/bench/fixture_run.py", to, edgeLoads, extracted)
	}
	for _, e := range g.Edges {
		if e.Kind == edgeLoads && e.From == "file:tools/bench/fixture_run.py" &&
			e.To != "tr:file:examples/tr/fixture_model.tr" &&
			e.To != "doc:docs/fixture_note.md" &&
			e.To != "file:tools/bench/py/fixture_char_model.py" {
			t.Errorf("unknown variable or external import must not create loads edge: %+v", e)
		}
	}

	ix := newIndex(g)
	ix.root = root
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["tr:file:examples/tr/fixture_model.tr"], defaultLimit)
	if !strings.Contains(out.String(), "bench.TestRunsPy") || !strings.Contains(out.String(), "2 steps via tools/bench/fixture_run.py") {
		t.Errorf("tests-for fixture_model.tr must list bench.TestRunsPy via tools/bench/fixture_run.py:\n%s", out.String())
	}

	repoRoot, _, rg := repoGraph(t)
	mustEdge(t, rg, "file:tools/bench/run.py", "tr:file:examples/tr/36_karakter_modeli.tr", edgeLoads, extracted)
	rix := newIndex(rg)
	rix.root = repoRoot
	out.Reset()
	rix.printTestsFor(&out, rix.byID["tr:file:examples/tr/36_karakter_modeli.tr"], defaultLimit)
	got := out.String()
	for _, want := range []string{
		"bench.TestRunPyModelProgramBuilds",
		"2 steps via tools/bench/run.py",
		"bench.TestRunPyTestsExist",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tests-for 36_karakter_modeli.tr missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "measurements, not tests") {
		t.Errorf("TestRunPyTestsExist must not fall into measurements, not tests:\n%s", got)
	}
}

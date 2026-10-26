package main

import "testing"

var snippetFiles = map[string]string{
	"internal/lang/codes/tr.json": "{\n  \"TR_fn_checkpoint\": { \"name\": \"kontrol_noktası\", \"family\": \"x\" },\n  \"TR_arg_every_minutes\": { \"name\": \"her_dakika\", \"family\": \"x\" },\n  \"TR_mem_update\": { \"name\": \"güncelle\", \"family\": \"x\" },\n  \"TR_fn_pretrained_model\": { \"name\": \"hazır_model\", \"family\": \"x\" }\n}\n",
	"lib/hazirx.tr":               "fonksiyon hazır_model(klasör) {\n    sonuç klasör\n}\n",
	"internal/lang/lexer/lexer.go": `package lexer

// New TR metnini okur.
func New(input string) int { return len(input) }
`,
	"app/run_test.go": `package app

import (
	"fmt"
	"testing"

	"example.com/audit/internal/lang/lexer"
)

// run TR kodunu okuyucuya iletir; ikinci bir yardımcı onu sarar.
func run(t *testing.T, code string) int { return lexer.New(code) }

func runIn(t *testing.T, dir, code string) int { t.Helper(); return run(t, code) }

func program(dir string) string {
	return fmt.Sprintf(` + "`" + `değişken kn = kontrol_noktası(%q, {}, her_dakika: 3)
kn.güncelle(1)
` + "`" + `, dir)
}

func notTR(t *testing.T, text string) int { return len(text) }

func TestFormatted(t *testing.T) { run(t, program("d")) }

func TestWrapped(t *testing.T) {
	code := ` + "`" + `değişken kn = kontrol_noktası(1, {})` + "`" + `
	runIn(t, "d", code)
}

func TestConcatenated(t *testing.T) {
	dir := t.TempDir()
	run(t, ` + "`" + `kontrol_noktası("` + "`" + ` + dir + ` + "`" + `", {}, her_dakika: 1)` + "`" + `)
}

func TestNotTRCode(t *testing.T) { notTR(t, "kn.güncelle(1)") }

func TestTable(t *testing.T) {
	for _, c := range []struct {
		code string
		n    int
	}{{"kontrol_noktası(1, {})", 1}, {code: "kn.güncelle(1)"}} {
		run(t, c.code+"\n")
	}
}

func TestNamedTable(t *testing.T) {
	cases := []struct{ code string }{{"kontrol_noktası(2, {}, her_dakika: 1)"}}
	for _, c := range cases {
		run(t, c.code)
	}
}

func far5(t *testing.T) int { return run(t, "kontrol_noktası(2, {})") }
func far4(t *testing.T) int { return far5(t) }
func far3(t *testing.T) int { return far4(t) }
func far2(t *testing.T) int { return far3(t) }
func far1(t *testing.T) int { return far2(t) }

func TestFar(t *testing.T) { far1(t) }
`,
	"app/file_test.go": `package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProgram TR kodunu bir .tr dosyasına yazar; dosyayı tr ikilisi koşar (debugger'ın yardımcısı gibi).
func writeProgram(t *testing.T, code string) string {
	path := filepath.Join(t.TempDir(), "program.tr")
	os.WriteFile(path, []byte(code), 0o644)
	return path
}

// writeFile klasöre adıyla bir dosya yazar (testkit.WriteFile gibi).
func writeFile(t *testing.T, dir, name, content string) {
	os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644)
}

func TestWrittenByHelper(t *testing.T) { writeProgram(t, "kontrol_noktası(1, {})") }

func TestWrittenDirectly(t *testing.T) {
	os.WriteFile(filepath.Join(t.TempDir(), "a.tr"), []byte("kn.güncelle(1)"), 0o644)
}

func TestWrittenByName(t *testing.T) {
	writeFile(t, t.TempDir(), "sub/b.tr", "kontrol_noktası(2, {}, her_dakika: 1)")
}

func TestWrittenFromAMap(t *testing.T) {
	for name, text := range map[string]string{"c.tr": "kn.güncelle(2)", "veri.json": "{}"} {
		writeFile(t, t.TempDir(), name, text)
	}
}

func TestWrittenLines(t *testing.T) {
	writeFile(t, t.TempDir(), "d.tr", strings.Join([]string{"değişken kn = kontrol_noktası(1, {})", "kn.güncelle(1)"}, "\n"))
}

func TestWrittenLibraryUse(t *testing.T) {
	writeProgram(t, "aktar \"tr/hazirx\"\nyazdır(hazır_model(\"k\"))\n")
}

func TestNotATRFile(t *testing.T) { writeFile(t, t.TempDir(), "notlar.txt", "kn.güncelle(1)") }
`,
}

// Kural: Go test dosyasında TR çalıştıran bir dize, dilin okuyucusuna varan bir işlevin dize parametresine giderse (yardımcı
// adıyla sabitlenmeden: çağrı zincirinden bulunur; Sprintf kalıbı, sabit, yerel değişken, eklenen parçalar dahil) testten
// kullandığı ad kodlarına `tests` kenarı çekilir; TR'ye gitmeyen bir yardımcıya verilen dize çekmez.
func TestGoTestsLinkToTheNameCodesOfTheTRTheyRun(t *testing.T) {
	g, err := buildGraph(auditModule(t, snippetFiles))
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	for _, c := range []struct{ test, name string }{
		{"TestFormatted", "TR_fn_checkpoint"}, {"TestFormatted", "TR_arg_every_minutes"}, {"TestFormatted", "TR_mem_update"},
		{"TestWrapped", "TR_fn_checkpoint"}, {"TestConcatenated", "TR_fn_checkpoint"}, {"TestConcatenated", "TR_arg_every_minutes"},
		{"TestTable", "TR_fn_checkpoint"}, {"TestTable", "TR_mem_update"}, {"TestNamedTable", "TR_arg_every_minutes"},
	} {
		mustEdge(t, g, app+c.test, "name:"+c.name, edgeTests, inferred)
	}
	// Dosyaya yazılıp tr ikilisine verilen TR metni de: yolu .tr ile biten dosyaya os.WriteFile'la (ya da içeriğini ve
	// yolunu ona ileten bir yardımcıyla) yazılan içerik; yol filepath.Join'le, ad sözlük tablosundan, içerik satırlardan
	// kurulabilir. Kitaplıktan aktarılan adın kodu da gelir. .tr olmayan dosyaya yazılan metin TR değildir.
	for _, c := range []struct{ test, name string }{
		{"TestWrittenByHelper", "TR_fn_checkpoint"}, {"TestWrittenDirectly", "TR_mem_update"}, {"TestWrittenByName", "TR_arg_every_minutes"},
		{"TestWrittenFromAMap", "TR_mem_update"}, {"TestWrittenLines", "TR_fn_checkpoint"}, {"TestWrittenLines", "TR_mem_update"},
		{"TestWrittenLibraryUse", "TR_fn_pretrained_model"},
	} {
		mustEdge(t, g, app+c.test, "name:"+c.name, edgeTests, inferred)
	}
	if e := edge(g, app+"TestNotATRFile", "name:TR_mem_update", edgeTests); e != nil {
		t.Errorf("text written to a file that is not .tr is not TR: %+v", e)
	}
	if e := edge(g, app+"TestNotTRCode", "name:TR_mem_update", edgeTests); e != nil {
		t.Errorf("a string given to a helper that never reaches the TR reader is not TR: %+v", e)
	}
	// TR metni testten beş adım uzakta: bu bağ "doğrudan" test sayılır, bu yüzden zincir en çok yarı sınırdır (4 adım;
	// daha uzunu tests-for'da zayıf kanıttır). Bağımsız denetim S3.
	if e := edge(g, app+"TestFar", "name:TR_fn_checkpoint", edgeTests); e != nil {
		t.Errorf("TR text five steps from the test is weak evidence, not a tests edge: %+v", e)
	}
}

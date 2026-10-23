package main

import "testing"

var snippetFiles = map[string]string{
	"internal/lang/codes/tr.json": "{\n  \"T702\": { \"ad\": \"kontrol_noktası\", \"aile\": \"x\" },\n  \"T984\": { \"ad\": \"her_dakika\", \"aile\": \"x\" },\n  \"Y010\": { \"ad\": \"güncelle\", \"aile\": \"x\" }\n}\n",
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
		{"TestFormatted", "T702"}, {"TestFormatted", "T984"}, {"TestFormatted", "Y010"},
		{"TestWrapped", "T702"}, {"TestConcatenated", "T702"}, {"TestConcatenated", "T984"},
	} {
		mustEdge(t, g, app+c.test, "name:"+c.name, edgeTests, inferred)
	}
	if e := edge(g, app+"TestNotTRCode", "name:Y010", edgeTests); e != nil {
		t.Errorf("a string given to a helper that never reaches the TR reader is not TR: %+v", e)
	}
}

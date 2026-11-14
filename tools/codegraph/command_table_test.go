package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: README.md'deki soru-komut tablosu ("Which command answers which question") ile usage sabitindeki
// komut listesi (gerekçeli muaflar dışında) aynı kümede kalır: usage'daki her komutun kendi satırı vardır,
// yani bir satırın komut sütunundaki ilk ters tırnaklı parçanın ilk sözcüğüdür; her satırın o sözcüğü de
// bilinen bir komuttur.
func TestCommandTableMatchesUsage(t *testing.T) {
	// Tabloda yer alması gerekmeyen usage komutları:
	// - build: grafik gerektiğinde kendiliğinden kurulur (README.md: "Nobody has to remember `build`.").
	exempt := map[string]string{
		"build": "Nobody has to remember `build`. (README.md)",
	}
	// help komutu usage sabitinde değil, run() içindeki switch kolundadır (main.go:95).
	const helpCommand = "help"
	var helpOut, helpErr strings.Builder
	if code := run([]string{helpCommand}, &helpOut, &helpErr); code != exitOK {
		t.Fatalf("help command failed with exit %d: %s", code, helpErr.String())
	}

	cmds := usageCommands(usage)
	if len(cmds) == 0 {
		t.Fatal("no commands parsed from usage")
	}
	inUsage := map[string]bool{}
	for _, c := range cmds {
		inUsage[c] = true
	}
	for c := range exempt {
		if !inUsage[c] {
			t.Errorf("exempt command %q is no longer in usage", c)
		}
	}

	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := readmeCommandRows(t, string(data))
	inTable := map[string]bool{}
	for i, r := range rows {
		cell := r.command
		spans := backtick.FindAllStringSubmatch(cell, -1)
		if len(spans) == 0 {
			t.Errorf("README.md command table row %d has no backticked command: %q", i+1, cell)
			continue
		}
		first := rowCommand(cell)
		if first == "" {
			t.Errorf("README.md command table row %d has empty first backtick span: %q", i+1, cell)
			continue
		}
		if !inUsage[first] && first != helpCommand {
			t.Errorf("README.md command table row %d starts with %q, which is not in usage (or %q)", i+1, first, helpCommand)
		}
		// Yalnız satırın ilk parçası sayılır: notlarda geçen komut adları (untested satırındaki
		// `tests-for <function>` gibi) o komutun kendi satırının yerini tutmaz (DENETİM 423 N1).
		inTable[first] = true
	}

	for _, c := range cmds {
		if _, ok := exempt[c]; ok {
			continue
		}
		if !inTable[c] {
			t.Errorf("command %q in usage is missing from README.md question-to-command table", c)
		}
	}
}

// usageCommands usage sabitindeki girintili komut satırlarından komut adlarını çıkarır.
func usageCommands(text string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "  ") || len(line) <= 2 || line[2] == ' ' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		add(fields[0])
	}
	return out
}

// commandRow soru-komut tablosunun bir veri satırının komut ve flow sütunlarıdır.
type commandRow struct{ command, flow string }

// rowCommand komut sütununun ilk ters tırnaklı parçasının ilk sözcüğüdür (yoksa boş).
func rowCommand(cell string) string {
	spans := backtick.FindAllStringSubmatch(cell, -1)
	if len(spans) == 0 {
		return ""
	}
	if f := strings.Fields(spans[0][1]); len(f) > 0 {
		return f[0]
	}
	return ""
}

// readmeCommandRows README.md'nin soru-komut tablosundaki veri satırlarının komut ve flow sütunlarını verir.
func readmeCommandRows(t *testing.T, doc string) []commandRow {
	t.Helper()
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	const heading = "## Which command answers which question\n"
	_, rest, ok := strings.Cut(doc, heading)
	if !ok {
		t.Fatalf("README.md has no %q section", strings.TrimSpace(heading))
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	var cells []commandRow
	tableRow := 0
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		tableRow++
		if tableRow <= 2 {
			continue // başlık ve hizalama satırları
		}
		cols := splitCells(strings.ReplaceAll(line, `\|`, "\x00"))
		if len(cols) != 3 {
			t.Fatalf("README.md command table row %d has %d columns, want 3 (question, command, flow): %q", tableRow-2, len(cols), raw)
		}
		cells = append(cells, commandRow{strings.ReplaceAll(cols[1], "\x00", "|"), strings.ReplaceAll(cols[2], "\x00", "|")})
	}
	if len(cells) == 0 {
		t.Fatal("README.md command table has no data rows")
	}
	return cells
}

// flowValues soru-komut tablosunun flow sütununun değerleridir (README.md'de tablonun altındaki cümle).
var flowValues = map[string]bool{"gate": true, "hook": true, "merge audit": true, "agent rules": true, "none": true}

// Kural (codegraph dilim 5): tablonun flow sütunu her komutun hangi akışa bağlı olduğunu söyler ve değerleri kümededir.
// gate ve hook iki yönlü sınanır: satır gate diyorsa tools/full_check.sh komutu `go run ./tools/edithook codegraph
// <komut>` ya da derlenmiş ikiliyle `"$bin" <komut>` biçiminde çağırır, çağırıyorsa satır gate der (yorum satırı
// sayılmaz; build bu biçimlerle çağrılmaz, tabloda satırı da yok); hook için tools/edithook'un test dışı Go
// dosyalarındaki `codegraph(<kök>, "<komut>", ...)` çağrısı. merge audit ve agent rules depo dışındaki belgelere
// dayanır: onlarda yalnız kümeye bakılır.
func TestCommandTableFlowsMatchTheGateAndHook(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	root := reporoot.Root()
	callers := []struct {
		flow, file string
		calls      map[string]bool
	}{
		{"gate", "tools/full_check.sh", gateCommands(t, filepath.Join(root, "tools", "full_check.sh"))},
		{"hook", "tools/edithook", hookCommands(t, filepath.Join(root, "tools", "edithook"))},
	}
	for _, c := range callers {
		if len(c.calls) == 0 {
			t.Fatalf("no codegraph call found in %s", c.file)
		}
	}
	for i, r := range readmeCommandRows(t, string(data)) {
		cmd := rowCommand(r.command)
		says := map[string]bool{}
		for _, f := range strings.Split(r.flow, ",") {
			f = strings.TrimSpace(f)
			if !flowValues[f] {
				t.Errorf("README.md command table row %d (%s): flow %q is not one of gate, hook, merge audit, agent rules, none", i+1, cmd, f)
			}
			says[f] = true
		}
		for _, c := range callers {
			if c.calls[cmd] != says[c.flow] {
				verb, col := "calls", "does not say "+c.flow
				if !c.calls[cmd] {
					verb, col = "does not call", "says "+c.flow
				}
				t.Errorf("README.md command table row %d (%s): %s %s \"codegraph %s\", but the flow column %s", i+1, cmd, c.file, verb, cmd, col)
			}
		}
	}
}

// gateCalls full_check.sh'in codegraph'ı çağırdığı iki biçimdir: kancanın yolu ve derlenmiş ikili.
var gateCalls = regexp.MustCompile(`go run \./tools/edithook codegraph ([a-z-]+)|"\$bin" ([a-z-]+)`)

// gateCommands betiğin yorum dışı satırlarında codegraph'a çağrılan komutlardır.
func gateCommands(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		code, _, _ := strings.Cut(line, " #")
		if strings.HasPrefix(strings.TrimSpace(code), "#") {
			continue
		}
		for _, m := range gateCalls.FindAllStringSubmatch(code, -1) {
			out[m[1]+m[2]] = true
		}
	}
	return out
}

// hookCalls edithook'un codegraph(kök, "<komut>", ...) çağrısıdır.
var hookCalls = regexp.MustCompile(`\bcodegraph\([^,()]+,\s*"([a-z-]+)"`)

// hookCommands klasörün test dışı Go dosyalarında codegraph'a sabit adla çağrılan komutlardır.
func hookCommands(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := reporoot.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range hookCalls.FindAllStringSubmatch(string(data), -1) {
			out[m[1]] = true
		}
	}
	return out
}

// Kapının iki çağrı biçimi de sayılır (DENETİM 695 N2): kancanın yolu kalkıp yalnız derlenmiş ikili kalsa da komut
// kapıda sayılır; yorumda ya da satır sonu yorumunda anılan komut sayılmaz (DENETİM 682 N3).
func TestGateCommandsCountBothCallFormsAndSkipComments(t *testing.T) {
	path := testkit.WriteFile(t, t.TempDir(), "full_check.sh", "# codegraph dups --changed, then go run ./tools/edithook codegraph untested\n"+
		"\tout=$(\"$bin\" spec-refs 2>&1) || code=$?\n"+
		"\tout=$(go run ./tools/edithook codegraph changed --go-tests --base HEAD HEAD) # go run ./tools/edithook codegraph similar\n"+
		"\tgo run ./tools/codegraph build >/dev/null\n")
	got := gateCommands(t, path)
	if len(got) != 2 || !got["spec-refs"] || !got["changed"] {
		t.Errorf("gate commands: %v, want changed and spec-refs", got)
	}
}

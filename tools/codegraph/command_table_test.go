package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// usageAlias usage satırındaki parantez içi takma adı yakalar: "(or explain)".
var usageAlias = regexp.MustCompile(`\(or ([a-z-]+)\)`)

// Kural: README.md'deki soru-komut tablosu ("Which command answers which question") ile usage sabitindeki
// komut listesi (gerekçeli muaflar dışında) aynı kümede kalır: usage'daki her komutun kendi satırı vardır,
// yani bir satırın komut sütunundaki ilk ters tırnaklı parçanın ilk sözcüğüdür; her satırın o sözcüğü de
// bilinen bir komuttur.
func TestCommandTableMatchesUsage(t *testing.T) {
	// Tabloda yer alması gerekmeyen usage komutları:
	// - explain: query komutunun takma adıdır (README.md:35).
	// - build: grafik gerektiğinde kendiliğinden kurulur (README.md: "Nobody has to remember `build`.").
	exempt := map[string]string{
		"explain": "alias for query (README.md:35)",
		"build":   "Nobody has to remember `build`. (README.md)",
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
	rows := readmeCommandCells(t, string(data))
	inTable := map[string]bool{}
	for i, cell := range rows {
		spans := backtick.FindAllStringSubmatch(cell, -1)
		if len(spans) == 0 {
			t.Errorf("README.md command table row %d has no backticked command: %q", i+1, cell)
			continue
		}
		firstFields := strings.Fields(spans[0][1])
		if len(firstFields) == 0 {
			t.Errorf("README.md command table row %d has empty first backtick span: %q", i+1, cell)
			continue
		}
		first := firstFields[0]
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

// usageCommands usage sabitindeki girintili komut satırlarından komut adlarını ve takma adlarını çıkarır.
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
		for _, m := range usageAlias.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
	}
	return out
}

// readmeCommandCells README.md'nin soru-komut tablosundaki veri satırlarının komut sütunlarını verir.
func readmeCommandCells(t *testing.T, doc string) []string {
	t.Helper()
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	const heading = "## Which command answers which question\n"
	_, rest, ok := strings.Cut(doc, heading)
	if !ok {
		t.Fatalf("README.md has no %q section", strings.TrimSpace(heading))
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	var cells []string
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
		if len(cols) != 2 {
			t.Fatalf("README.md command table row %d has %d columns, want 2: %q", tableRow-2, len(cols), raw)
		}
		cells = append(cells, strings.ReplaceAll(cols[1], "\x00", "|"))
	}
	if len(cells) == 0 {
		t.Fatal("README.md command table has no data rows")
	}
	return cells
}

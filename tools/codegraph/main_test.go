package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Kural: `help`'in changed satırı ucu çıkış yapılmış HEAD olan farkın çalışma ağacına kadar alındığını söyler (gaps,
// "`changed`'in `help` satırı çalışma ağacını söylemiyor": yalnız README söylüyordu, yan oturumlar `changed HEAD`'i
// istek olarak yazdı; kural changeHunks'ta).
func TestHelpSaysChangedHeadReadsTheWorkingTree(t *testing.T) {
	var out, errOut strings.Builder
	if code := run([]string{"help"}, &out, &errOut); code != exitOK {
		t.Fatalf("help: exit %d: %s", code, errOut.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "changed <") {
			if !strings.Contains(line, `"changed HEAD", the diff runs to the working tree`) {
				t.Errorf("the changed line does not say the working tree is included:\n%s", line)
			}
			return
		}
	}
	t.Fatalf("help has no changed line:\n%s", out.String())
}

// Kural (K6): `explain` ve `history` komut değildir. İkisi çıkış 2 verir ve aynı işi gören yolu yazar: `explain`
// `query`'nin öbür adıydı, ileti aynı argümanlarla `query` satırını verir; `history`'yi git cevaplar, ileti adın dosyasını
// bulan `query` adımını ve her ad türü için sabitlenmiş `git log` biçimini verir. İletideki her biçim git'li fikstür
// deposunda bir kez koşar ve o bildirimi (dosyayı, kodu) değiştiren commit'i verir, öbürünü vermez.
func TestRetiredCommandsNameTheirReplacement(t *testing.T) {
	var out, errOut strings.Builder
	if code := run([]string{"explain", "shape.Square.Area"}, &out, &errOut); code != exitUsage || out.Len() != 0 {
		t.Fatalf("explain: exit %d, want %d; stdout %q", code, exitUsage, out.String())
	}
	if want := "codegraph: \"explain\" was another name for query and is gone: go run ./tools/codegraph query shape.Square.Area\n"; errOut.String() != want {
		t.Errorf("explain: stderr\n%q\nwant\n%q", errOut.String(), want)
	}
	if code, got, _ := cli(t, "query", "shape.Square.Area"); code != exitOK || !strings.HasPrefix(got, "shape.Square.Area [method]") {
		t.Errorf("the query the message names: exit %d\n%s", code, got)
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"history", "shape.Orphan"}, &out, &errOut); code != exitUsage || out.Len() != 0 {
		t.Fatalf("history: exit %d, want %d; stdout %q", code, exitUsage, out.String())
	}
	if errOut.String() != historyGone+"\n" {
		t.Errorf("history: stderr\n%q\nwant\n%q", errOut.String(), historyGone+"\n")
	}

	root := gitFixture(t)
	shape := filepath.Join(root, "shape", "shape.go")
	replaceIn(t, shape, "return s.Side * s.Side", "return s.Side * s.Side * 1")
	commitAll(t, root, "shape: touch Area")
	replaceIn(t, shape, "func Orphan() {}", "func Orphan() { _ = 1 }")
	commitAll(t, root, "shape: touch Orphan")
	replaceIn(t, shape, "type Square struct{ Side float64 }", "type Square struct{ Side float64 } // touched")
	commitAll(t, root, "shape: touch Square")
	replaceIn(t, filepath.Join(root, "shape", "leaf.go"), "func leaf() {}", "func leaf() { _ = 2 }")
	commitAll(t, root, "shape: touch leaf.go")
	tr := filepath.Join(root, "internal", "lang", "messages", "tr.json")
	defs := filepath.Join(root, "internal", "lang", "messages", "definitions.json")
	replaceIn(t, tr, "\"H0002\": \"second message\"", "\"H0002\": \"second message\",\n\"H0003\": \"third\"")
	replaceIn(t, defs, "\"H0002\": { \"param\": [] }", "\"H0002\": { \"param\": [] },\n\"H0003\": { \"param\": [] }")
	commitAll(t, root, "messages: add H0003")

	forms := regexp.MustCompile(`git log (.+?) for a (function|method|type|file|code)\b`).FindAllStringSubmatch(historyGone, -1)
	cases := map[string]struct{ name, file, want string }{
		"function": {"Orphan", "shape/shape.go", "shape: touch Orphan"},
		"method":   {"Area", "shape/shape.go", "shape: touch Area"},
		"type":     {"Square", "shape/shape.go", "shape: touch Square"},
		"file":     {"", "shape/leaf.go", "shape: touch leaf.go"},
		"code":     {"H0003", "", "messages: add H0003"},
	}
	if len(forms) != len(cases) {
		t.Fatalf("the message has %d git log forms, want %d:\n%s", len(forms), len(cases), historyGone)
	}
	for _, f := range forms {
		c := cases[f[2]]
		line := strings.NewReplacer("<name>", c.name, "<file>", c.file, "<code>", c.name).Replace(f[1])
		got, err := gitOut(root, append([]string{"log"}, splitArgs(strings.ReplaceAll(line, "'", "\""))...)...)
		if err != nil {
			t.Errorf("%s: git log %s: %v", f[2], line, err)
			continue
		}
		subjects := string(got)
		if !strings.Contains(subjects, c.want) {
			t.Errorf("%s: git log %s does not list %q:\n%s", f[2], line, c.want, subjects)
		}
		for _, other := range cases {
			if other.want != c.want && strings.Contains(subjects, other.want) {
				t.Errorf("%s: git log %s also lists %q:\n%s", f[2], line, other.want, subjects)
			}
		}
	}
}

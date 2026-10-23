package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// gitFixture testdata modülünün kopyasını geçici bir git deposu yapar ve ilk commit'i atar.
func gitFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	dst := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), dst)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"},
	} {
		if _, err := gitOut(dst, args...); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// buildInto graph.json'ı kurar ve yolunu verir.
func buildInto(t *testing.T, root string) string {
	t.Helper()
	out := filepath.Join(root, ".codegraph", "graph.json")
	var so, se bytes.Buffer
	if code := run([]string{"build", "--root", root, "--out", out}, &so, &se); code != exitOK {
		t.Fatalf("build: exit %d: %s", code, se.String())
	}
	return out
}

func TestStampIsStableAndFollowsContent(t *testing.T) {
	root := gitFixture(t)
	a, err := sourceStamp(root)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := sourceStamp(root); a != b {
		t.Error("the stamp of an unchanged tree changes")
	}
	file := filepath.Join(root, "shape", "extra.go")
	write := func(body string) {
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package shape\n\nfunc One() {}\n")
	b, _ := sourceStamp(root)
	write("package shape\n\nfunc Two() {}\n")
	c, _ := sourceStamp(root)
	if a == b || b == c {
		t.Errorf("an untracked .go file and its content must change the stamp: %s %s %s", a, b, c)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d, _ := sourceStamp(root); d != c {
		t.Error("a changed file that is not a source must not change the stamp")
	}
	// TR kaynağı ve go.mod da kaynaktır (bağımsız denetim Y8: lib/*.tr'ye eklenen işlev grafiği eskitmiyordu).
	for _, f := range []string{"lib/geo.tr", "go.mod"} {
		before, _ := sourceStamp(root)
		appendTo(t, filepath.Join(root, filepath.FromSlash(f)), "\n")
		if after, _ := sourceStamp(root); after == before {
			t.Errorf("a change to %s must change the stamp", f)
		}
	}
}

func appendTo(t *testing.T, file, text string) {
	t.Helper()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

// Kural: grafiğin bir düğüme kaynak yaptığı her dosya türü imzaya girer; yoksa o dosyadaki değişiklik grafiği eskitmez
// ve sorgu eski cevabı verir. Bağımsız kaynak çıkarıcının kendi çıktısıdır: fikstür grafiğindeki her düğümün dosyası
// (Go, sözlük, belge, TR) stamped olmalı.
func TestStampCoversEverySourceTheGraphReads(t *testing.T) {
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root) // testdata altında go list paket görmez
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, n := range g.Nodes {
		if n.File == "" || n.Kind == kindFile || n.Kind == kindDoc {
			continue // dosya düğümü yalnızca varlığı sayar; içeriği okunmaz
		}
		kinds[filepath.Ext(n.File)] = true
		if !stamped(n.File) {
			t.Errorf("%s [%s] comes from %s, whose changes do not change the stamp (fresh.go stampExts)", n.Name, n.Kind, n.File)
		}
	}
	for _, ext := range []string{".go", ".json", ".md", ".tr"} {
		if !kinds[ext] {
			t.Errorf("the fixture has no node from a %s file; the rule is not exercised", ext)
		}
	}
}

func TestQueryRebuildsStaleGraph(t *testing.T) {
	root := gitFixture(t)
	graph := buildInto(t, root)
	code, _, errOut := commandIn("query", root, graph, "shape.Describe")
	if code != exitOK || strings.Contains(errOut, "rebuilt") {
		t.Fatalf("a fresh graph must not be rebuilt: exit %d: %s", code, errOut)
	}
	extra := "package shape\n\n// Fresh appears after a source change.\nfunc Fresh() {}\n"
	if err := os.WriteFile(filepath.Join(root, "shape", "fresh.go"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = commandIn("query", root, graph, "--no-rebuild", "shape.Fresh")
	if code != exitNotFound || strings.Contains(errOut, "rebuilt") {
		t.Errorf("--no-rebuild must use the old graph: exit %d: %s", code, errOut)
	}
	code, out, errOut := commandIn("query", root, graph, "shape.Fresh")
	if code != exitOK || !strings.Contains(out, "shape.Fresh [func]") {
		t.Fatalf("the query must see the new function: exit %d\nout: %s\nerr: %s", code, out, errOut)
	}
	if !strings.HasPrefix(errOut, "graph rebuilt (") || strings.Count(errOut, "\n") != 1 {
		t.Errorf("stderr must be the single line \"graph rebuilt (N s)\": %q", errOut)
	}
	if _, _, errOut = commandIn("query", root, graph, "shape.Fresh"); errOut != "" {
		t.Errorf("the rebuilt graph is fresh, stderr: %q", errOut)
	}
}

func TestQueryKeepsOldGraphWhenRebuildFails(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	graph := buildInto(t, root)
	if err := os.WriteFile(filepath.Join(root, "shape", "broken.go"), []byte("package shape\n\nfunc {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := commandIn("query", root, graph, "shape.Describe")
	if code != exitOK || !strings.Contains(out, "shape.Describe") {
		t.Errorf("the old graph still answers: exit %d: %s", code, out)
	}
	if !strings.Contains(errOut, "warning: rebuild failed") || strings.Contains(errOut, "rebuilt (") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestQueryWarnsWhenGitFails(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	graph := buildInto(t, root)
	notRepo := t.TempDir()
	code, out, errOut := commandIn("query", notRepo, graph, "shape.Describe")
	if code != exitOK || !strings.Contains(out, "shape.Describe") {
		t.Errorf("exit %d: %s", code, out)
	}
	if !strings.Contains(errOut, "cannot check whether the graph is stale") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestChangedEntriesParsesRenames(t *testing.T) {
	status := []byte(" M a.go\x00?? b.txt\x00R  new.md\x00old.md\x00A  c.json\x00")
	var got []string
	for _, e := range changedEntries(status) {
		got = append(got, strings.TrimSpace(e.status)+":"+e.path)
	}
	want := "M:a.go,A:c.json,R:new.md,old:old.md"
	if strings.Join(got, ",") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}

func TestRuleName(t *testing.T) {
	cases := map[string]string{
		"- **Katman sınırı.** Bir paket üsttekini bilmez.": "Katman sınırı",
		"**Kural adı.** Gövde":                             "Kural adı",
		"Önce ölç, sonra değiştir. Ayrıntı **burada** var": "Önce ölç, sonra değiştir",
		"Bütün dosyalar **bütün** olmalı":                  "Bütün dosyalar bütün olmalı",
		"1. Bir madde `kod` ile. İkinci cümle":             "Bir madde kod ile",
		"Katmanlar, dil içi sıra, dış bağımlılıklar":       "Katmanlar, dil içi sıra, dış bağımlılıklar",
	}
	for in, want := range cases {
		if got := ruleName(in); got != want {
			t.Errorf("ruleName(%q) = %q, want %q", in, got, want)
		}
	}
}

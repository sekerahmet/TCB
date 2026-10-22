package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// commitAll geçici fikstür deposunda her şeyi verilen konuyla commit'ler.
func commitAll(t *testing.T, root, subject string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", subject},
	} {
		if _, err := gitOut(root, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func replaceIn(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s lacks %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryOfFunctionListsOnlyCommitsThatChangedIt(t *testing.T) {
	root := gitFixture(t)
	file := filepath.Join(root, "shape", "shape.go")
	replaceIn(t, file, "return s.Side * s.Side", "return s.Side * s.Side * 1")
	commitAll(t, root, "shape: touch Area")
	replaceIn(t, file, "func Orphan() {}", "func Orphan() { _ = 1 }")
	commitAll(t, root, "shape: touch Orphan")
	graph := buildInto(t, root)
	code, out, errOut := commandIn("history", root, graph, "shape.Square.Area")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "shape.Square.Area [method]", "shape: touch Area", "fixture")
	if strings.Contains(out, "touch Orphan") {
		t.Errorf("a commit that did not touch Area is listed:\n%s", out)
	}
	if strings.Contains(out, "@@") || strings.Contains(out, "diff --git") {
		t.Errorf("history lists commits, not diffs:\n%s", out)
	}
	_, out, _ = commandIn("history", root, graph, "--limit", "1", "shape.Square.Area")
	testkit.MustContain(t, out, "shape: touch Area", "limited to 1")
}

func TestHistoryOfCodeMarksAddedAndRemoved(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	tr := filepath.Join(root, "internal", "lang", "messages", "tr.json")
	defs := filepath.Join(root, "internal", "lang", "messages", "definitions.json")
	replaceIn(t, tr, "\"H0002\": \"second message\"", "\"H0002\": \"second message\",\n\"H0003\": \"third\"")
	replaceIn(t, defs, "\"H0002\": { \"param\": [] }", "\"H0002\": { \"param\": [] },\n\"H0003\": { \"param\": [] }")
	commitAll(t, root, "messages: add H0003")
	replaceIn(t, tr, ",\n\"H0003\": \"third\"", "")
	replaceIn(t, defs, ",\n\"H0003\": { \"param\": [] }", "")
	commitAll(t, root, "messages: drop H0003")

	events, err := scanCodeHistory(root, messageFiles, "")
	if err != nil {
		t.Fatal(err)
	}
	list := events["H0003"]
	if len(list) != 4 || !list[0].removed || !list[len(list)-1].added {
		t.Fatalf("H0003 events newest first (removed twice, added twice): %+v", list)
	}
	if list[0].commit.subject != "messages: drop H0003" || list[3].commit.subject != "messages: add H0003" {
		t.Errorf("commits: %+v", list)
	}
	// Taşınan (aynı commit'te eklenen ve silinen) satır olay sayılmaz: H0001 yalnızca ilk commit'te eklendi.
	if got := events["H0001"]; len(got) != 2 || !got[0].added {
		t.Errorf("H0001: %+v", got)
	}

	graph := buildInto(t, root)
	code, out, errOut := commandIn("history", root, graph, "H0001")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "H0001 [message]", "added", "fixture")
}

func TestHistoryUnsupportedKind(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	graph := buildInto(t, root)
	code, _, errOut := commandIn("history", root, graph, "Empty shapes fail")
	if code != exitUsage || !strings.Contains(errOut, "not available for a rule node") {
		t.Errorf("exit %d: %s", code, errOut)
	}
}

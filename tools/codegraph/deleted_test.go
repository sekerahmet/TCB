package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// dropH0002 H0002'yi hem ileti kataloğundan hem tanımlardan siler ve commit'ler.
func dropH0002(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "internal", "lang", "messages")
	replaceIn(t, filepath.Join(dir, "tr.json"), ",\n\"H0002\": \"second message\"", "")
	replaceIn(t, filepath.Join(dir, "definitions.json"), ",\n\"H0002\": { \"param\": [] }", "")
	commitAll(t, root, "messages: drop H0002")
}

func runDeletedIn(root string, args ...string) (int, string, string) {
	return runCLI(append([]string{"deleted", "--root", root}, args...)...)
}

func TestDeletedListsCodesGoneFromTheDictionary(t *testing.T) {
	root := gitFixture(t)
	code, out, errOut := runDeletedIn(root)
	if code != exitOK || !strings.Contains(out, "messages: 0 deleted codes") {
		t.Fatalf("nothing is deleted yet: exit %d\n%s%s", code, out, errOut)
	}
	dropH0002(t, root)
	code, out, errOut = runDeletedIn(root)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "messages: 1 deleted codes", "H0002  removed ", "messages: drop H0002", "(added ", "fixture")
	if strings.Contains(out, "defined again") {
		t.Errorf("H0002 is not defined again:\n%s", out)
	}
}

func TestDeletedFlagsRedefinedCodes(t *testing.T) {
	root := gitFixture(t)
	dropH0002(t, root)
	dir := filepath.Join(root, "internal", "lang", "messages")
	replaceIn(t, filepath.Join(dir, "tr.json"), "\"H0001\": \"first message {name}\"", "\"H0001\": \"first message {name}\",\n\"H0002\": \"back\"")
	commitAll(t, root, "messages: reuse H0002 (wrong)")
	_, out, _ := runDeletedIn(root)
	testkit.MustContain(t, out, "messages: 0 deleted codes", "1 codes defined again after being deleted", "H0002  removed ")
}

func TestDeletedRetiredJSON(t *testing.T) {
	root := gitFixture(t)
	dropH0002(t, root)
	code, out, errOut := runDeletedIn(root, "--retired-json")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var got map[string]struct{ Commit, Reason string }
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	e, ok := got["H0002"]
	if !ok || len(got) != 1 || e.Commit == "" || !strings.Contains(e.Reason, "drop H0002") {
		t.Errorf("retired.json content: %+v", got)
	}
	if code, _, _ := runDeletedIn(root, "--retired-json", "--family", "nope"); code != exitUsage {
		t.Errorf("unknown family: exit %d", code)
	}
}

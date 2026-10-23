package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Alt klasörden --root ve --graph olmadan sorulabilir; grafik kökteki .codegraph'tadır.
func TestQueryFromSubdirectoryUsesRootGraph(t *testing.T) {
	top := t.TempDir()
	testkit.WritePath(t, filepath.Join(top, "go.mod"), "module "+repoModule+"\n")
	if err := writeGraph(filepath.Join(top, ".codegraph", "graph.json"), fixture(t)); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(top, "pkg", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	var so, se bytes.Buffer
	if code := run([]string{"query", "--no-rebuild", "shape.Describe"}, &so, &se); code != exitOK || !strings.Contains(so.String(), "shape.Describe") {
		t.Errorf("exit %d\n%s%s", code, so.String(), se.String())
	}
	// Grafik yoksa hata kökteki yolu söyler.
	if err := os.Remove(filepath.Join(top, ".codegraph", "graph.json")); err != nil {
		t.Fatal(err)
	}
	so.Reset()
	se.Reset()
	if code := run([]string{"query", "--no-rebuild", "x"}, &so, &se); code != exitFail || !strings.Contains(se.String(), ".codegraph") {
		t.Errorf("missing graph: exit %d: %s", code, se.String())
	}
}

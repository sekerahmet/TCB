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

func sameDir(t *testing.T, got, want string) {
	t.Helper()
	g, err1 := filepath.EvalSymlinks(got)
	w, err2 := filepath.EvalSymlinks(want)
	if err1 != nil || err2 != nil || g != w {
		t.Errorf("root = %q, want %q", got, want)
	}
}

func TestFindRootByModule(t *testing.T) {
	top := t.TempDir()
	testkit.WritePath(t, filepath.Join(top, "go.mod"), "module "+repoModule+"\n\ngo 1.25\n")
	deep := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	// Ara klasördeki başka bir modül kök sayılmaz.
	testkit.WritePath(t, filepath.Join(top, "a", "go.mod"), "module example.com/other\n")
	got, err := findRoot(deep)
	if err != nil {
		t.Fatal(err)
	}
	sameDir(t, got, top)
}

func TestFindRootFallsBackToGitToplevel(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	top := t.TempDir()
	testkit.WritePath(t, filepath.Join(top, "go.mod"), "module example.com/not-tr\n")
	if _, err := gitOut(top, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(top, "x", "y")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findRoot(deep)
	if err != nil {
		t.Fatal(err)
	}
	sameDir(t, got, top)
}

func TestFindRootErrorsWhenNothingMatches(t *testing.T) {
	dir := t.TempDir()
	if _, err := gitOut(dir, "rev-parse", "--show-toplevel"); err == nil {
		t.Skip("geçici klasör bir git deposunun içinde")
	}
	if _, err := findRoot(dir); err == nil || !strings.Contains(err.Error(), "--root") {
		t.Errorf("err = %v", err)
	}
}

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

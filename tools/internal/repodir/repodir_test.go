package repodir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: Find çalışma klasöründen yukarı doğru go.mod'un bulunduğu ilk klasörü verir (test paket
// klasöründen çalışır; kök, iki üst klasördür... daha derin olabilir: go.mod'un varlığına bakılır).
func TestFindReturnsTheDirectoryWithGoMod(t *testing.T) {
	root := Find()
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("Find() = %q: go.mod yok: %v", root, err)
	}
}

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
	testkit.WritePath(t, filepath.Join(top, "go.mod"), "module "+Module+"\n\ngo 1.25\n")
	deep := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	// Ara klasördeki başka bir modül kök sayılmaz.
	testkit.WritePath(t, filepath.Join(top, "a", "go.mod"), "module example.com/other\n")
	got, err := FindFrom(deep)
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
	if err := exec.Command("git", "-C", top, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(top, "x", "y")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindFrom(deep)
	if err != nil {
		t.Fatal(err)
	}
	sameDir(t, got, top)
}

func TestFindRootErrorsWhenNothingMatches(t *testing.T) {
	dir := t.TempDir()
	probe := exec.Command("git", "rev-parse", "--show-toplevel")
	probe.Dir = dir
	if probe.Run() == nil {
		t.Skip("geçici klasör bir git deposunun içinde")
	}
	if _, err := FindFrom(dir); err == nil || !strings.Contains(err.Error(), "--root") {
		t.Errorf("err = %v", err)
	}
}

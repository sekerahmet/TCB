package repodir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Kural: Find çalışma klasöründen yukarı doğru go.mod'un bulunduğu ilk klasörü verir (test paket
// klasöründen çalışır; kök, iki üst klasördür... daha derin olabilir: go.mod'un varlığına bakılır).
func TestFindReturnsTheDirectoryWithGoMod(t *testing.T) {
	root := Find()
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("Find() = %q: go.mod yok: %v", root, err)
	}
}

// FindFrom iç içe go.mod'lara takılmaz: depo kökü, go.mod'u TR modülünü söyleyen klasördür.
func TestFindFromSkipsNestedModules(t *testing.T) {
	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "go.mod"), []byte("module "+Module+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(top, "tools", "testdata", "mod")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example.com/fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindFrom(nested)
	if err != nil || got != top {
		t.Errorf("FindFrom = %q, %v; want %q", got, err, top)
	}
}

// FindFrom, go.mod'u TR modülünü söylemeyen bir depoda git'in kökünü verir (tools/codegraph'tan taşındı: findRoot
// yalnızca bunu iletiyordu, bağımsız denetim O2).
func TestFindFromFallsBackToGitToplevel(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "go.mod"), []byte("module example.com/not-tr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", top, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	deep := filepath.Join(top, "x", "y")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindFrom(deep)
	g, _ := filepath.EvalSymlinks(got)
	w, _ := filepath.EvalSymlinks(top)
	if err != nil || g != w {
		t.Errorf("FindFrom = %q, %v; want %q", got, err, top)
	}
}

// Ne TR modülü ne git varsa hata --root ister.
func TestFindFromErrorsWhenNothingMatches(t *testing.T) {
	dir := t.TempDir()
	if exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Run() == nil {
		t.Skip("geçici klasör bir git deposunun içinde")
	}
	if _, err := FindFrom(dir); err == nil || !strings.Contains(err.Error(), "--root") {
		t.Errorf("err = %v", err)
	}
}

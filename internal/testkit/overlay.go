package testkit

import (
	"encoding/json"

	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// BuildWithFile derlenmiş bir ikili verir: pkg (ör. "./cmd/tr") depodaki repoFile dosyası
// (depo köküne göre, ör. "internal/lang/codes/tr.json") content ile değiştirilmiş olarak derlenir.
// Değiştirme go build -overlay ile yapılır: depo dosyası değişmez, gömülü (go:embed) dosyalar da
// değişir. Bozuk bir veri dosyasının ikiliyi nasıl etkilediğini sınayan testler içindir.
func BuildWithFile(t testing.TB, pkg, repoFile, content string) string {
	t.Helper()
	dir := t.TempDir()
	replaced := WriteFile(t, dir, filepath.Base(repoFile), content)
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {reporoot.Path(filepath.FromSlash(repoFile)): replaced},
	})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := WriteFile(t, dir, "overlay.json", string(overlay))
	exe := filepath.Join(dir, filepath.Base(pkg))
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command("go", "build", "-overlay", overlayPath, "-o", exe, pkg)
	build.Dir = reporoot.Root()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -overlay %s: %v\n%s", pkg, err, out)
	}
	return exe
}

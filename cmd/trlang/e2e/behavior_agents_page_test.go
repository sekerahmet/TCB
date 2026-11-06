package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// AGENTS.md'nin eğitim programı yabancı bir ajanın ilk çalıştırdığı şeydir: yalnız denetimden
// geçmesi yetmez, çalışmalıdır. lib/README.md'nin örneği denetimden geçip çalışırken düşüyordu.
func TestAgentsPageProgramRuns(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	block := trBlockRe.FindStringSubmatch(string(data))
	if block == nil {
		t.Fatal("AGENTS.md'de ```tr bloğu yok")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "agents.tr")
	if err := os.WriteFile(file, []byte(block[2]), 0644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(trBinary, "run", file)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("AGENTS.md'nin programı çalışmıyor: %v\n%s", err, output)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last == "" {
		t.Fatalf("program metin üretmedi:\n%s", output)
	}
	if !strings.Contains(string(output), "loaded 21") {
		t.Fatalf("model yüklenmedi:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "model.safetensors")); err != nil {
		t.Fatalf("model kaydedilmedi: %v", err)
	}
}

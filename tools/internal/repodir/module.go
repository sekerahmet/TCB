package repodir

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Module TR deposunun modül yoludur; FindFrom'un aradığı kök, go.mod'u bunu söyleyen klasördür.
const Module = "github.com/sekerahmet/TCB"

// Resolve --root değerini mutlak yola çevirir. Verilmemişse (boş) bugünkü klasörden yukarı doğru
// depo kökü aranır (FindFrom), böylece alt klasörden de sorulabilir.
func Resolve(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindFrom(cwd)
}

// FindFrom start'tan yukarı doğru depo kökünü arar: önce go.mod'unun modül yolu Module olan klasör
// (iç içe go.mod'lara, örneğin testdata modüllerine takılmaz), yoksa `git rev-parse --show-toplevel`,
// o da yoksa açık bir hata.
func FindFrom(start string) (string, error) {
	for dir := start; ; {
		if moduleOf(filepath.Join(dir, "go.mod")) == Module {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = start
	top, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot find the repository root from %s: no go.mod of module %s above it and not inside a git repository; give --root DIR", start, Module)
	}
	return filepath.FromSlash(strings.TrimSpace(string(top))), nil
}

// moduleOf go.mod'daki `module` satırının yolunu verir; dosya yoksa "".
func moduleOf(gomod string) string {
	f, err := os.Open(gomod)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

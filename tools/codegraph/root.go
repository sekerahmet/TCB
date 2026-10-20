package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// repoModule TR deposunun modül yoludur; kök, go.mod'u bunu söyleyen klasördür.
const repoModule = "github.com/sekerahmet/TCB"

// resolveRoot --root değerini mutlak yola çevirir. Verilmemişse (boş) bugünkü klasörden
// yukarı doğru depo kökü aranır, böylece alt klasörden de sorulabilir: önce go.mod'u
// `module github.com/sekerahmet/TCB` olan klasör, yoksa `git rev-parse --show-toplevel`,
// o da yoksa açık bir hata.
func resolveRoot(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRoot(cwd)
}

// findRoot start'tan yukarı doğru depo kökünü arar (bkz. resolveRoot).
func findRoot(start string) (string, error) {
	for dir := start; ; {
		if moduleOf(filepath.Join(dir, "go.mod")) == repoModule {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	top, err := gitOut(start, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("cannot find the repository root from %s: no go.mod of module %s above it and not inside a git repository; give --root DIR", start, repoModule)
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

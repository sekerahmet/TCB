// Package repodir araçlara (tools/gen/..., tools/codegraph) depo kökünü verir: kök, go.mod'u TR
// modülünü söyleyen klasördür; bulunamazsa git deposunun kökü; o da yoksa açık bir hata.
//
// Layer: tools (tools); kullanıcıya gitmez.
package repodir

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Module TR deposunun modül yoludur; kök, go.mod'u bunu söyleyen klasördür.
const Module = "github.com/sekerahmet/TCB"

// Find çalışma klasöründen yukarı doğru depo kökünü verir (go generate paket klasörlerinden
// çalıştırır); bulunamazsa ya da çalışma klasörü okunamazsa ".".
func Find() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	root, err := FindFrom(d)
	if err != nil {
		return "."
	}
	return root
}

// FindFrom start'tan yukarı doğru depo kökünü arar: önce go.mod'u `module github.com/sekerahmet/TCB`
// olan klasör (iç içe başka modüllerin go.mod'una, ör. testdata/mod, takılmaz), yoksa
// `git rev-parse --show-toplevel`, o da yoksa açık bir hata.
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
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cannot find the repository root from %s: no go.mod of module %s above it and not inside a git repository; give --root DIR", start, Module)
	}
	return filepath.FromSlash(strings.TrimSpace(out.String())), nil
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

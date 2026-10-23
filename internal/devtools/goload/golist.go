package goload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Listed `go list -json` çıktısının araçların kullandığı kısmıdır.
type Listed struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
	// IgnoredGoFiles yapı kısıtı yüzünden bu yapılandırmada derlenmeyen dosyalardır (başka işletim sistemi, mimari ya da
	// etiket; testler dahil).
	IgnoredGoFiles []string
	Imports        []string
	Export         string
	Module         *struct{ Path string }
}

func goList(dir string) ([]*Listed, error) {
	return runGoList(dir, "-e", "-json", "-deps", "-test", "./...")
}

// exportData modül dışı paketlerin derlenmiş dışa aktarım dosyalarını verir.
func exportData(dir string, paths []string) (map[string]string, error) {
	exports := map[string]string{}
	if len(paths) == 0 {
		return exports, nil
	}
	pkgs, err := runGoList(dir, append([]string{"-e", "-json", "-export"}, paths...)...)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
	}
	return exports, nil
}

func runGoList(dir string, args ...string) ([]*Listed, error) {
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list: %v\n%s", err, errb.String())
	}
	var pkgs []*Listed
	dec := json.NewDecoder(&out)
	for dec.More() {
		p := new(Listed)
		if err := dec.Decode(p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

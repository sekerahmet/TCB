package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Tanımsız ad denetimi (docs/spec/05_deyimler.md §5.14) dinamik dilde yanlış alarm vermemeli:
// depodaki bütün .tr dosyaları (testler, örnekler, kıyas programları, kitaplık) hatasız
// denetlenir. Bilerek hatalı fikstürler (tests/negative/) hariçtir. Başka dosyaların
// aktardığı modüller, yorumlayıcıdaki gibi aktaranın genel adlarıyla denetlenir (11.1).
func TestRepoTRFilesPassCheck(t *testing.T) {
	root := reporoot.Root()
	sources := map[string]string{}
	err := filepath.WalkDir(root, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "negative", "node_modules", ".claude", "testdata", "hatalar", "errors": // testdata: araçların kasıtlı bozuk fikstürleri; hatalar, errors: hata örnekleri (examples/)
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(filePath, ".tr") {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			sources[filePath] = string(content)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) < 20 {
		t.Fatalf("yalnızca %d .tr dosyası bulundu", len(sources))
	}

	// Modül → aktaranların genel adları (birleşimi); aktaranlardan biri dinamikse dinamik.
	type global struct {
		names   map[string]bool
		dynamic bool
	}
	modules := map[string]*global{}
	for filePath, code := range sources {
		names, dynamic := checker.TopLevelNames(source.Prepare(code, source.Options{}).Program)
		for _, y := range importSites(code) {
			resolved, _, ok := resolveModulePath(y.filePath, filePath, filePath)
			if !ok {
				continue
			}
			absolute, _ := filepath.Abs(resolved)
			g := modules[absolute]
			if g == nil {
				g = &global{names: map[string]bool{}}
				modules[absolute] = g
			}
			for _, a := range names {
				g.names[a] = true
			}
			g.dynamic = g.dynamic || dynamic
		}
	}

	for filePath, code := range sources {
		relative, _ := filepath.Rel(root, filePath)
		absolute, _ := filepath.Abs(filePath)
		var diags []error
		if g := modules[absolute]; g != nil {
			var names []string
			for a := range g.names {
				names = append(names, a)
			}
			diags = interp.CheckModule(code, names, g.dynamic).Errors
		} else {
			diags = source.Prepare(code, source.Options{TypeCheck: true}).Errors
		}
		for _, h := range diags {
			t.Errorf("%s: %s", filepath.ToSlash(relative), h)
		}
	}
}

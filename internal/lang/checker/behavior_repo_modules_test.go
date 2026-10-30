package checker_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (1.1, 12.5): depodaki modüller (kitaplık ve bir dosyanın aktardığı her dosya) ve test
// dosyaları (TR_test içeren dosya, 12.1) en dışta yalnız tanım tutar, ilk değerleri sabittir: denetleyici onları modül ve test
// dosyası olarak okuyunca H1107 ve H1108 vermez. Olumsuz fikstürler (tests/negative/) hariçtir.
func TestRepoModulesHoldOnlyDefinitions(t *testing.T) {
	files := testkit.GlobFiles(t, "lib/*.tr", "tests/*.tr", "tests/*/*.tr", "examples/tr/*.tr", "examples/tr/helpers/*.tr", "tools/gpu/*.tr", "tools/bench/tr/*.tr")
	modules := importedFiles(t, files)
	var checked []string
	for _, file := range files {
		rel := relPath(file)
		test := strings.HasSuffix(file, "_test.tr")
		if strings.HasPrefix(rel, "tests/negative/") || !test && !modules[file] && !strings.HasPrefix(rel, "lib/") {
			continue
		}
		checked = append(checked, rel)
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		r := source.Prepare(string(data), source.Options{TypeCheck: true, Module: !test})
		for _, err := range r.Errors {
			if messages.Has(err, messages.H1107) || messages.Has(err, messages.H1108) {
				line, _, _ := messages.Pos(err)
				t.Errorf("%s:%d: %v", rel, line, err)
			}
		}
	}
	if len(checked) < 40 {
		t.Fatalf("yalnız %d modül ve test dosyası bulundu: %v", len(checked), checked)
	}
}

// importedFiles files'ın TR_import ile aktardığı diskteki dosyalardır (kitaplığın "tr/x"i lib/x.tr).
func importedFiles(t *testing.T, files []string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		ast.Walk(source.Prepare(string(data), source.Options{}).Program, func(n any) bool {
			if im, ok := n.(*ast.ImportStatement); ok {
				p := strings.TrimSuffix(im.Path, ".tr") + ".tr"
				if lib, ok := strings.CutPrefix(p, "tr/"); ok {
					out[reporoot.Path("lib/"+lib)] = true
				} else {
					out[filepath.Join(filepath.Dir(file), filepath.FromSlash(p))] = true
				}
			}
			return true
		})
	}
	return out
}

func relPath(file string) string {
	rel, _ := filepath.Rel(reporoot.Root(), file)
	return filepath.ToSlash(rel)
}

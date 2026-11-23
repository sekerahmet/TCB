package internal_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/layoutrules"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// .tr dosyalarının boy ölçüsü (TRLines, TRFuncs) ve istisna listesi (TRBigFiles) internal/layoutrules'tadır: kod grafı
// (tools/codegraph, tests-for'un size: satırı) aynı sayıyı oradan okur. İstisnaların gerekçeleri Go'nunkilerle birlikte
// internal/layout_exceptions_test.go'dadır (trBigFileReasons).

// trSourceFiles izlenen el yazımı .tr dosyalarıdır: bilerek hatalı fikstürler (tests/negative),
// codegraph'ın fikstürleri ve arşiv dışında (layoutrules.TRMeasured).
func trSourceFiles(t *testing.T) map[string][]string {
	t.Helper()
	root := reporoot.Root()
	out := map[string][]string{}
	err := reporoot.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".tr" {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if !layoutrules.TRMeasured(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = layoutrules.TRLines(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 100 {
		t.Fatalf("yalnız %d .tr dosyası bulundu; tarama dosyaları kaçırıyor", len(out))
	}
	return out
}

// Kural (kod kalitesi; Go'nun TestLayoutFileSize'ının eşi): el yazımı .tr dosyası en çok
// layoutrules.FileLimit satırdır; mevcut istisnalar layoutrules.TRBigFiles'ta gerekçesiyle durur ve büyüyemez (tavan +
// düzeltme payı; TCB 2026-11-23 18:56).
func TestLayoutTRFileSize(t *testing.T) {
	visited := map[string]bool{}
	for rel, lines := range trSourceFiles(t) {
		n := len(lines)
		max, listed := layoutrules.TRFileMax(rel) // listede değilse FileLimit, istisnaysa tavan + düzeltme payı
		visited[rel] = listed
		switch {
		case !listed && n > max:
			t.Errorf("%s: %d satır (sınır %d); dosyayı konularına göre bölün (docs/development.md)", rel, n, max)
		case listed && n <= layoutrules.FileLimit:
			t.Errorf("%s: artık %d satır; layoutrules.TRBigFiles listesinden çıkarın (liste yalnızca küçülür)", rel, n)
		case listed && n > max:
			t.Errorf("%s: %d satır, istisna sınırı %d (tavan %d + düzeltme payı); büyüyen istisna dosyası bölünmeli", rel, n, max, layoutrules.TRBigFiles[rel])
		}
	}
	checkExceptions(t, "TRBigFiles", layoutrules.TRBigFiles, trBigFileReasons, layoutrules.MaxTRBigFiles, visited)
}

// Kural (kod kalitesi; Go'nun TestLayoutFuncSize'ının eşi): TR fonksiyonu (TR_func satırından
// kapanan parantezine) en çok layoutrules.FuncLimit satırdır; iç içe fonksiyon kendi boyuyla sayılır.
func TestLayoutTRFuncSize(t *testing.T) {
	funcs := 0
	for rel, lines := range trSourceFiles(t) {
		for _, f := range layoutrules.TRFuncs(lines) {
			funcs++
			if f.Lines > layoutrules.FuncLimit {
				t.Errorf("%s:%d: fonksiyon %d satır (sınır %d); bölün", rel, f.Line, f.Lines, layoutrules.FuncLimit)
			}
		}
	}
	if funcs < 500 {
		t.Fatalf("yalnız %d TR fonksiyonu bulundu; tarama fonksiyonları kaçırıyor", funcs)
	}
}

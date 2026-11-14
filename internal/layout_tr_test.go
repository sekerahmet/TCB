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

// trBigFiles layoutrules.FileLimit'i aşan el yazımı .tr dosyalarıdır, gerekçesiyle. Liste yalnız
// muafiyettir, tavan koymaz (TCB, DENETİM 738 Ş2 (a)): listedeki dosya büyüyebilir; sınırın altına
// inen dosya listeden çıkar.
var trBigFiles = map[string]string{
	"lib/turkish.tr":            "Türkçe sözcük tabloları: veri, bölünmesi ayrı iş",
	"tests/tensor_test.tr":      "tensör bölümünün (15) testleri tek dosyada",
	"tests/yerlesikler_test.tr": "yerleşiklerin (13) testleri tek dosyada",
	"lib/layers_test.tr":        "katmanların testleri tek dosyada",
	"lib/layers.tr":             "katmanlar; bölünmesi status'ta (kod kalitesi, lib/layers.tr)",
	"tests/tipler_test.tr":      "tiplerin (3) testleri tek dosyada",
	"lib/vits.tr":               "VITS modeli tek dosyada",
	"tests/vad_test.tr":         "ses etkinliği bulmanın (25) testleri tek dosyada",
}

// trSourceFiles izlenen el yazımı .tr dosyalarıdır: bilerek hatalı fikstürler (tests/negative),
// codegraph'ın fikstürleri ve arşiv dışında.
func trSourceFiles(t *testing.T) map[string][]string {
	t.Helper()
	root := reporoot.Root()
	out := map[string][]string{}
	err := reporoot.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".tr" {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		for _, skip := range []string{"tests/negative/", "tools/codegraph/testdata/", "docs/archive/"} {
			if strings.HasPrefix(rel, skip) {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n")
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
// layoutrules.FileLimit satırdır; aşan dosya trBigFiles'ta gerekçesiyle durur.
func TestLayoutTRFileSize(t *testing.T) {
	files := trSourceFiles(t)
	for rel, lines := range files {
		_, listed := trBigFiles[rel]
		switch {
		case !listed && len(lines) > layoutrules.FileLimit:
			t.Errorf("%s: %d satır (sınır %d); dosyayı konularına göre bölün (docs/development.md)", rel, len(lines), layoutrules.FileLimit)
		case listed && len(lines) <= layoutrules.FileLimit:
			t.Errorf("%s: artık %d satır; trBigFiles listesinden çıkarın", rel, len(lines))
		}
	}
	for rel := range trBigFiles {
		if _, ok := files[rel]; !ok {
			t.Errorf("trBigFiles: %s izlenen bir .tr dosyası değil; kaydı silin", rel)
		}
	}
}

// Kural (kod kalitesi; Go'nun TestLayoutFuncSize'ının eşi): TR fonksiyonu (TR_func satırından
// kapanan parantezine) en çok layoutrules.FuncLimit satırdır; iç içe fonksiyon kendi boyuyla sayılır.
func TestLayoutTRFuncSize(t *testing.T) {
	funcs := 0
	for rel, lines := range trSourceFiles(t) {
		for i, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "TR_func ") {
				continue
			}
			funcs++
			if n := trFuncLines(lines[i:]); n > layoutrules.FuncLimit {
				t.Errorf("%s:%d: fonksiyon %d satır (sınır %d); bölün", rel, i+1, n, layoutrules.FuncLimit)
			}
		}
	}
	if funcs < 500 {
		t.Fatalf("yalnız %d TR fonksiyonu bulundu; tarama fonksiyonları kaçırıyor", funcs)
	}
}

// trFuncLines lines[0]'da başlayan fonksiyonun satır sayısıdır: ilk '{'dan onu kapatan '}'a kadar
// (metin değişmezlerinin ve yorumların içindeki parantezler sayılmaz). Gövdesi bulunmayan satır 1'dir.
func trFuncLines(lines []string) int {
	depth, opened := 0, false
	for i, line := range lines {
		inText, raw := false, false
		for j := 0; j < len(line); j++ {
			switch c := line[j]; {
			case raw:
				raw = c != '`'
			case inText:
				if c == '\\' {
					j++
				} else if c == '"' {
					inText = false
				}
			case c == '#':
				j = len(line)
			case c == '"':
				inText = true
			case c == '`':
				raw = true
			case c == '{':
				depth, opened = depth+1, true
			case c == '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return i + 1
		}
	}
	return 1
}

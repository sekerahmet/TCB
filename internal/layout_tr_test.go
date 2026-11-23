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

// .tr dosyalarının boy ölçüsü (TRLines, TRFuncs) ve muafiyet listesi (TRBigFiles) internal/layoutrules'tadır: kod grafı
// (tools/codegraph, tests-for'un size: satırı) aynı sayıyı oradan okur.

// trBigFileReasons layoutrules.TRBigFiles'ın gerekçeleridir (internal/layout_exceptions_test.go'daki gibi: gerekçe
// geliştiricinin notudur, metin sabiti olarak test dosyasında durur). Her kaydın bir gerekçesi vardır, gerekçesi olan her
// dosya listededir (TestLayoutTRFileSize).
var trBigFileReasons = map[string]string{
	"lib/turkish.tr":            "Türkçe sözcük tabloları: veri, bölünmesi ayrı iş",
	"tests/tensor_test.tr":      "tensör bölümünün (15) testleri tek dosyada",
	"tests/yerlesikler_test.tr": "yerleşiklerin (13) testleri tek dosyada",
	"lib/layers_test.tr":        "katmanların testleri tek dosyada",
	"lib/layers.tr":             "katmanlar; bölünmesi status'ta (kod kalitesi, lib/layers.tr)",
	"tests/tipler_test.tr":      "tiplerin (3) testleri tek dosyada",
	"lib/vits.tr":               "VITS modeli tek dosyada",
	"lib/whisper.tr":            "Whisper modeli tek dosyada",
	"tests/vad_test.tr":         "ses etkinliği bulmanın (25) testleri tek dosyada",
	"lib/pretrained.tr":         "hazır model yükleyicisi tek modülde: _ adlar dışa aktarılmaz (11.7), bölmek yeni kamu adı ister",
}

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
// layoutrules.FileLimit satırdır; aşan dosya layoutrules.TRBigFiles'ta gerekçesiyle durur.
func TestLayoutTRFileSize(t *testing.T) {
	files := trSourceFiles(t)
	for rel, lines := range files {
		listed := layoutrules.TRFileExempt(rel)
		switch {
		case !listed && len(lines) > layoutrules.FileLimit:
			t.Errorf("%s: %d satır (sınır %d); dosyayı konularına göre bölün (docs/development.md)", rel, len(lines), layoutrules.FileLimit)
		case listed && len(lines) <= layoutrules.FileLimit:
			t.Errorf("%s: artık %d satır; layoutrules.TRBigFiles listesinden çıkarın", rel, len(lines))
		}
	}
	for rel := range layoutrules.TRBigFiles {
		if _, ok := files[rel]; !ok {
			t.Errorf("layoutrules.TRBigFiles: %s izlenen bir .tr dosyası değil; kaydı silin", rel)
		}
		if strings.TrimSpace(trBigFileReasons[rel]) == "" {
			t.Errorf("layoutrules.TRBigFiles: %s kaydının gerekçesi yok; trBigFileReasons'a yazın", rel)
		}
	}
	for rel := range trBigFileReasons {
		if !layoutrules.TRBigFiles[rel] {
			t.Errorf("trBigFileReasons: %s layoutrules.TRBigFiles'ta değil; gerekçeyi silin", rel)
		}
	}
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

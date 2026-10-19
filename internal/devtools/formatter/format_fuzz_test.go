package formatter

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// FuzzFormat: 'tr biçimle' bir klasördeki dosyaları yerinde yeniden yazar; indirilen bir
// klasör de biçimlenebilir. Biçimlendirici hiçbir girdide çökmemeli; başarılı sonuç
// yeniden biçimlendirildiğinde değişmemelidir (anlam denetimi Format'ın içindedir).
// Derin arama:
//
//	go test ./internal/devtools/formatter -run '^$' -fuzz FuzzFormat -fuzztime 5m
func FuzzFormat(f *testing.F) {
	for _, dir := range []string{"tests", "examples", "lib"} {
		_ = filepath.WalkDir(filepath.Join(reporoot.Root(), dir), func(fpath string, g fs.DirEntry, err error) error {
			if err == nil && !g.IsDir() && strings.HasSuffix(fpath, ".tr") {
				if data, err := os.ReadFile(fpath); err == nil {
					f.Add(string(data))
				}
			}
			return nil
		})
	}
	f.Add("eğer x {\r\n// yorum\r\n  yazdır(\"{x}\")   }\n")
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 64<<10 {
			return
		}
		output, err := Format(src)
		if err != nil {
			return
		}
		second, err := Format(output)
		if err != nil || second != output {
			t.Fatalf("biçimleme kararlı değil (hata %v):\n%q\n->\n%q", err, output, second)
		}
	})
}

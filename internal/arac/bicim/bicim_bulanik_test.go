package bicim

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/depo"
)

// FuzzBicimle: 'tr biçimle' bir klasördeki dosyaları yerinde yeniden yazar; indirilen bir
// klasör de biçimlenebilir. Biçimlendirici hiçbir girdide çökmemeli; başarılı sonuç
// yeniden biçimlendirildiğinde değişmemelidir (anlam denetimi Bicimle'nin içindedir).
// Derin arama:
//
//	go test ./internal/arac/bicim -run '^$' -fuzz FuzzBicimle -fuzztime 5m
func FuzzBicimle(f *testing.F) {
	for _, klasor := range []string{"testler", "ornekler", "kitaplik"} {
		_ = filepath.WalkDir(filepath.Join(depo.Kok(), klasor), func(yol string, g fs.DirEntry, err error) error {
			if err == nil && !g.IsDir() && strings.HasSuffix(yol, ".tr") {
				if veri, err := os.ReadFile(yol); err == nil {
					f.Add(string(veri))
				}
			}
			return nil
		})
	}
	f.Add("eğer x {\r\n// yorum\r\n  yazdır(\"{x}\")   }\n")
	f.Fuzz(func(t *testing.T, kaynak string) {
		if len(kaynak) > 64<<10 {
			return
		}
		cikti, err := Bicimle(kaynak)
		if err != nil {
			return
		}
		ikinci, err := Bicimle(cikti)
		if err != nil || ikinci != cikti {
			t.Fatalf("biçimleme kararlı değil (hata %v):\n%q\n->\n%q", err, cikti, ikinci)
		}
	})
}

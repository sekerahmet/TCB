package sozcuk

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/depo"
)

// trTohumlari depodaki bütün .tr dosyalarını (testler/, ornekler/, kitaplik/) tohum olarak ekler.
func trTohumlari(f *testing.F) {
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
}

// FuzzSozcukCozumleyici: kaynak kod güvenilmeyen girdidir ('tr kontrol', biçimlendirici ve
// VS Code bir dosyayı çalıştırmadan okur). Sözcük çözümleyici hiçbir girdide çökmemeli,
// sonsuz döngüye girmemeli (girdi uzunluğundan fazla sözcük birimi üretmemeli) ve her
// sözcük biriminin aralığı girdinin içinde kalmalıdır. Derin arama:
//
//	go test ./internal/dil/sozcuk -run '^$' -fuzz FuzzSozcukCozumleyici -fuzztime 5m
func FuzzSozcukCozumleyici(f *testing.F) {
	trTohumlari(f)
	for _, s := range []string{"\ufeff", `"{'{"{}"}'}"`, "\"\\", "'''", "1e999999", "0x", "/*", "#", "\x00", "\xff\xfe"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, kaynak string) {
		l := New(kaynak)
		uzunluk := len(l.Girdi())
		for i := 0; ; i++ {
			if i > uzunluk+1 {
				t.Fatalf("%d karakterlik girdiden %d'den fazla sözcük birimi: sonsuz döngü", uzunluk, i)
			}
			tok := l.NextToken()
			bas, son := l.SonAralik()
			if bas < 0 || bas > son || son > uzunluk {
				t.Fatalf("geçersiz aralık [%d, %d) (girdi %d karakter)", bas, son, uzunluk)
			}
			if tok.Type == TOKEN_EOF {
				return
			}
		}
	})
}

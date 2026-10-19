package lexer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// trTohumlari depodaki bütün .tr dosyalarını (tests/, examples/, lib/) tohum olarak ekler.
func trSeeds(f *testing.F) {
	for _, dir := range []string{"tests", "examples", "lib"} {
		_ = filepath.WalkDir(filepath.Join(reporoot.Kok(), dir), func(path string, g fs.DirEntry, err error) error {
			if err == nil && !g.IsDir() && strings.HasSuffix(path, ".tr") {
				if data, err := os.ReadFile(path); err == nil {
					f.Add(string(data))
				}
			}
			return nil
		})
	}
}

// FuzzSozcukCozumleyici: kaynak kod güvenilmeyen girdidir ('tr kontrol', biçimlendirici ve
// düzenleyici bir dosyayı çalıştırmadan okur). Sözcük çözümleyici hiçbir girdide çökmemeli,
// sonsuz döngüye girmemeli (girdi uzunluğundan fazla sözcük birimi üretmemeli) ve her
// sözcük biriminin aralığı girdinin içinde kalmalıdır. Derin arama:
//
//	go test ./internal/lang/lexer -run '^$' -fuzz FuzzSozcukCozumleyici -fuzztime 5m
func FuzzLexer(f *testing.F) {
	trSeeds(f)
	for _, s := range []string{"\ufeff", `"{'{"{}"}'}"`, "\"\\", "'''", "1e999999", "0x", "/*", "#", "\x00", "\xff\xfe"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		l := New(source)
		length := len(l.Input())
		for i := 0; ; i++ {
			if i > length+1 {
				t.Fatalf("%d karakterlik girdiden %d'den fazla sözcük birimi: sonsuz döngü", length, i)
			}
			tok := l.NextToken()
			begin, end := l.LastRange()
			if begin < 0 || begin > end || end > length {
				t.Fatalf("geçersiz aralık [%d, %d) (girdi %d karakter)", begin, end, length)
			}
			if tok.Type == TOKEN_EOF {
				return
			}
		}
	})
}

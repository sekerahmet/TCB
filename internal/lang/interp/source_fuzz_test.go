package interp

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// FuzzKaynakDerle: kaynak kod güvenilmeyen girdidir ('tr kontrol' ve düzenleyici bir dosyayı
// çalıştırmadan denetler; 'aktar' başka dosyaları okur). Ayrıştırıcı (program ve modül
// biçimi), tip denetleyicisi ve derleyici hiçbir girdide çökmemelidir. Program
// ÇALIŞTIRILMAZ: yalnızca derlenir (hata ayıklama kipi tip denetimini atladığı için
// derleyici denetimden geçmeyen programları da görür). Derin arama:
//
//	go test ./internal/lang/interp -run '^$' -fuzz FuzzKaynakDerle -fuzztime 5m
func FuzzKaynakDerle(f *testing.F) {
	for _, klasor := range []string{"tests", "examples", "lib"} {
		_ = filepath.WalkDir(filepath.Join(reporoot.Kok(), klasor), func(yol string, g fs.DirEntry, err error) error {
			if err == nil && !g.IsDir() && strings.HasSuffix(yol, ".tr") {
				if veri, err := os.ReadFile(yol); err == nil {
					f.Add(string(veri))
				}
			}
			return nil
		})
	}
	f.Add(strings.Repeat("(", 2000) + "1" + strings.Repeat(")", 2000))
	f.Add("x = \"{'{\"{1}\"}'}\"\n")
	f.Fuzz(func(t *testing.T, kod string) {
		if len(kod) > 64<<10 {
			return
		}
		_ = CheckModule(kod, nil, true)
		p := parser.New(lexer.New(kod))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return
		}
		chk := checker.New()
		_ = chk.CheckProgram(prog)
		_ = chk.Warnings()
		v := New()
		c := v.yeniDerleyici()
		c.kok.onTanimla(prog.Statements)
		_ = c.deyimler(c.kok, prog.Statements)
	})
}

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

// FuzzCompileSource: kaynak kod güvenilmeyen girdidir ('tr kontrol' ve düzenleyici bir dosyayı
// çalıştırmadan denetler; 'aktar' başka dosyaları okur). Ayrıştırıcı (program ve modül
// biçimi), tip denetleyicisi ve derleyici hiçbir girdide çökmemelidir. Program
// ÇALIŞTIRILMAZ: yalnızca derlenir (hata ayıklama kipi tip denetimini atladığı için
// derleyici denetimden geçmeyen programları da görür). Derin arama:
//
//	go test ./internal/lang/interp -run '^$' -fuzz FuzzCompileSource -fuzztime 5m
func FuzzCompileSource(f *testing.F) {
	for _, folder := range []string{"tests", "examples", "lib"} {
		_ = reporoot.WalkDir(filepath.Join(reporoot.Root(), folder), func(filePath string, g fs.DirEntry, err error) error {
			if err == nil && !g.IsDir() && strings.HasSuffix(filePath, ".tr") {
				if payload, err := os.ReadFile(filePath); err == nil {
					f.Add(string(payload))
				}
			}
			return nil
		})
	}
	f.Add(strings.Repeat("(", 2000) + "1" + strings.Repeat(")", 2000))
	f.Add("x = \"{'{\"{1}\"}'}\"\n")
	f.Fuzz(func(t *testing.T, codeVal string) {
		if len(codeVal) > 64<<10 {
			return
		}
		_ = CheckModule(codeVal, nil, true)
		p := parser.New(lexer.New(codeVal))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return
		}
		chk := checker.New()
		_ = chk.CheckProgram(prog)
		_ = chk.Warnings()
		v := New()
		c := v.newCompiler()
		c.root.predefine(prog.Statements)
		_ = c.statements(c.root, prog.Statements)
	})
}

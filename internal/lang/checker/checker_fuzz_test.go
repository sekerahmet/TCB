package checker

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// trSeeds adds every .tr file of the repository (tests/, examples/, lib/) as a seed.
func trSeeds(f *testing.F) {
	for _, dir := range []string{"tests", "examples", "lib"} {
		_ = filepath.WalkDir(filepath.Join(reporoot.Root(), dir), func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".tr") {
				if data, err := os.ReadFile(path); err == nil {
					f.Add(string(data))
				}
			}
			return nil
		})
	}
}

// FuzzChecker: the checker runs on every program before it runs and on files the editor only
// reads. On any program that parses it must not panic or take long. Deep search:
//
//	go test ./internal/lang/checker -run '^$' -fuzz FuzzChecker -fuzztime 5m
func FuzzChecker(f *testing.F) {
	trSeeds(f)
	for _, s := range []string{"değişken a: Tam = \"x\"", "sabit a = 1\na = 2", "fonksiyon f(a: Tam): Metin { sonuç a }", "yazdır(tanımsız)", "değişken yazdır = 1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		p := parser.New(lexer.New(source))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return
		}
		begin := time.Now()
		New().CheckProgram(prog)
		if d := time.Since(begin); d > 2*time.Second {
			t.Fatalf("%d baytlık program %v sürdü", len(source), d)
		}
	})
}

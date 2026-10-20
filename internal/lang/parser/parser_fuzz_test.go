package parser

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// FuzzParser: source is untrusted input ('tr kontrol', the formatter and the editor parse it
// without running it). The parser must not panic or take long, and every error must carry a
// position. Deep search:
//
//	go test ./internal/lang/parser -run '^$' -fuzz FuzzParser -fuzztime 5m
func FuzzParser(f *testing.F) {
	trSeeds(f)
	for _, s := range []string{"eğer {", "fonksiyon (", "yapı A { fonksiyon", "\"{(\"", "[x döngü", "eşle x {", "a.b.c(", "((((", "test \"x\" {"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		begin := time.Now()
		p := New(lexer.New(source))
		p.ParseProgram()
		for _, e := range p.Errs() {
			if _, _, ok := messages.Pos(e); !ok {
				t.Fatalf("konumsuz hata: %q", e)
			}
		}
		if d := time.Since(begin); d > 2*time.Second {
			t.Fatalf("%d baytlık girdi %v sürdü", len(source), d)
		}
	})
}

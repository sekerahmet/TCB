package checker

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// FuzzChecker: the checker runs on every program before it runs and on files the editor only
// reads. On any program that parses it must not panic or take long. Deep search:
//
//	go test ./internal/lang/checker -run '^$' -fuzz FuzzChecker -fuzztime 5m
func FuzzChecker(f *testing.F) {
	testkit.TRSeeds(f)
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

package parser

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// FuzzParser: source is untrusted input ('tr kontrol', the formatter and the editor parse it
// without running it). The parser must not panic or take long, and every error must carry a
// position. Deep search:
//
//	go test ./internal/lang/parser -run '^$' -fuzz FuzzParser -fuzztime 5m
func FuzzParser(f *testing.F) {
	testkit.TRSeeds(f)
	for _, s := range []string{"TR_if {", "TR_func (", "TR_struct A { TR_func", "\"{(\"", "[x TR_for", "TR_match x {", "a.b.c(", "((((", "TR_test \"x\" {"} {
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

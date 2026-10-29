package lexer

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// FuzzCodedLexer: a coded file is untrusted input too ('tr show', the editor). In coded
// mode the lexer must not panic, loop or take long, and every token range must stay inside
// the input. Deep search:
//
//	go test ./internal/lang/lexer -run '^$' -fuzz FuzzCodedLexer -fuzztime 5m
func FuzzCodedLexer(f *testing.F) {
	testkit.TRSeeds(f)
	for _, s := range []string{"TR_kw_var x = 1\nTR_fn_print(x)\n", `TR_fn_print("{TR_fn_print("{x}")}")`, "x.TR_mem_shape", "TR_arg_lr: 1", "$TR_kw_if", "A0", "Z999", "\"{", "TR_fn_print(\"{\"{\"{\"{1}\"}\"}\"})"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		begin := time.Now()
		l := NewBody(source, true)
		length := len(l.Input())
		checkFuzzLexer(t, l)
		if d := time.Since(begin); d > 2*time.Second {
			t.Fatalf("%d karakterlik girdi %v sürdü", length, d)
		}
	})
}

package lexer

import (
	"testing"
	"time"
)

// FuzzCodedLexer: a coded file is untrusted input too ('tr göster', the editor). In coded
// mode the lexer must not panic, loop or take long, and every token range must stay inside
// the input. Deep search:
//
//	go test ./internal/lang/lexer -run '^$' -fuzz FuzzCodedLexer -fuzztime 5m
func FuzzCodedLexer(f *testing.F) {
	trSeeds(f)
	for _, s := range []string{"A030 x = 1\nG100(x)\n", `G100("{G100("{x}")}")`, "x.Y400", "T910: 1", "$A001", "A0", "Z999", "\"{", "G100(\"{\"{\"{\"{1}\"}\"}\"})"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		begin := time.Now()
		l := NewBody(source, true)
		length := len(l.Input())
		for i := 0; ; i++ {
			if i > length+1 {
				t.Fatalf("%d karakterlik girdiden %d'den fazla sözcük birimi: sonsuz döngü", length, i)
			}
			tok := l.NextToken()
			b, e := l.LastRange()
			if b < 0 || b > e || e > length {
				t.Fatalf("geçersiz aralık [%d, %d) (girdi %d karakter)", b, e, length)
			}
			if tok.Line < 1 {
				t.Fatalf("geçersiz satır %d", tok.Line)
			}
			if tok.Type == TokEOF {
				break
			}
		}
		if d := time.Since(begin); d > 2*time.Second {
			t.Fatalf("%d karakterlik girdi %v sürdü", length, d)
		}
	})
}

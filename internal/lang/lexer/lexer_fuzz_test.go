package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// FuzzLexer: kaynak kod güvenilmeyen girdidir ('tr kontrol', biçimlendirici ve
// düzenleyici bir dosyayı çalıştırmadan okur). Sözcük çözümleyici hiçbir girdide çökmemeli,
// sonsuz döngüye girmemeli (girdi uzunluğundan fazla sözcük birimi üretmemeli) ve her
// sözcük biriminin aralığı girdinin içinde kalmalıdır. Derin arama:
//
//	go test ./internal/lang/lexer -run '^$' -fuzz FuzzLexer -fuzztime 5m
func FuzzLexer(f *testing.F) {
	testkit.TRSeeds(f)
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
			if tok.Type == TokEOF {
				return
			}
		}
	})
}

package source

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// FuzzShow: rastgele kodlu gövdenin görünümü ya yazılır ya da sözdizimi hatasıyla reddedilir; yazılan
// görünüm dosyanın satır sayısını tutar ve görünüm başlığını taşır (17.7). Derin arama:
//
//	go test ./internal/lang/source -run '^$' -fuzz FuzzShow -fuzztime 5m
func FuzzShow(f *testing.F) {
	testkit.TRSeeds(f)
	for _, s := range []string{"\xef\xbb\xbfTR_print(1)\r\n", "TR_var eğer = 1", "TR_print(\"\\(\"\\(TR_sum([1]))\")\")", "# TR 9", "`{eğer}` # TR_print"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 16<<10 {
			return
		}
		text := Header + "\n" + body
		if b, err := Split(text); err != nil || !b.Coded {
			return
		}
		shown, err := Show(text, codes.Source())
		var sh *SyntaxError
		if errors.As(err, &sh) || errors.Is(err, ErrUTF8) {
			return
		}
		if err != nil {
			t.Fatalf("görünüm yazılamadı: %v\nkaynak: %q", err, text)
		}
		if !strings.HasPrefix(shown, viewHeader(codes.Source())) || strings.Count(shown, "\n") != strings.Count(text, "\n") {
			t.Fatalf("görünüm başlığı ya da satırları tutmuyor\nkaynak: %q\ngörünüm: %q", text, shown)
		}
	})
}

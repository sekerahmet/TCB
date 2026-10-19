package source

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// FuzzEncodeDecode: rastgele Türkçe kaynakta kodlu biçim kayıpsızdır (17.4). Ayrıştırılabilen her
// kaynak kodlanır (çevirici kendi sonucunu denetler; ErrKayip bir çevirici hatasıdır) ve
// kodlu biçim çözülünce özgün kaynak bayt bayt geri gelir. Gereksiz '$' kaçışı çözümde
// düşebileceği için '$' içeren kaynakta kod → TR → kod kimliği sınanır. Derin arama:
//
//	go test ./internal/lang/source -run '^$' -fuzz FuzzEncodeDecode -fuzztime 5m
func FuzzEncodeDecode(f *testing.F) {
	for _, path := range []string{"examples/02_dil_turu.tr", "examples/03_testler_test.tr", "tests/yapilar_test.tr", "tests/sozcuk_yapisi_test.tr"} {
		if data, err := os.ReadFile(reporoot.Path(path)); err == nil {
			f.Add(string(data))
		}
	}
	for _, s := range []string{"\xef\xbb\xbfyazdır(1)\r\n", "değişken $eğer = 1", "yazdır(\"{\"{toplam([1])}\"}\")", Header + "\nG100(1)", "//! tr kodlu 9", "`{eğer}` // yazdır"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tr string) {
		if b, err := Split(tr); len(tr) > 16<<10 || err != nil || b.Coded {
			return
		}
		coded, err := Encode(tr)
		var sh *SyntaxError
		var bh *HeaderError
		if errors.As(err, &sh) || errors.As(err, &bh) || errors.Is(err, ErrUTF8) {
			return
		}
		if err != nil {
			t.Fatalf("kodlanamadı: %v\nkaynak: %q", err, tr)
		}
		back, err := Decode(coded, codes.Active())
		if err != nil {
			t.Fatalf("kodlu biçim çözülemedi: %v\nkaynak: %q\nkodlu: %q", err, tr, coded)
		}
		if strings.Contains(tr, "$") {
			if again, err := Encode(back); err != nil || again != coded {
				t.Fatalf("kod → TR → kod aynı değil (%v)\nkodlu: %q\nyine: %q", err, coded, again)
			}
			return
		}
		if back != tr {
			t.Fatalf("TR → kod → TR aynı değil\nkaynak: %q\nkodlu: %q\ngeri: %q", tr, coded, back)
		}
	})
}

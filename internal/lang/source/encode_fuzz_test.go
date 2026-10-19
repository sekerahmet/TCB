package source

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// FuzzKodlaCoz: rastgele Türkçe kaynakta kodlu biçim kayıpsızdır (17.4). Ayrıştırılabilen her
// kaynak kodlanır (çevirici kendi sonucunu denetler; ErrKayip bir çevirici hatasıdır) ve
// kodlu biçim çözülünce özgün kaynak bayt bayt geri gelir. Gereksiz '$' kaçışı çözümde
// düşebileceği için '$' içeren kaynakta kod → TR → kod kimliği sınanır. Derin arama:
//
//	go test ./internal/lang/source -run '^$' -fuzz FuzzKodlaCoz -fuzztime 5m
func FuzzKodlaCoz(f *testing.F) {
	for _, yol := range []string{"examples/02_dil_turu.tr", "examples/03_testler_test.tr", "tests/yapilar_test.tr", "tests/sozcuk_yapisi_test.tr"} {
		if veri, err := os.ReadFile(reporoot.Yol(yol)); err == nil {
			f.Add(string(veri))
		}
	}
	for _, s := range []string{"\xef\xbb\xbfyazdır(1)\r\n", "değişken $eğer = 1", "yazdır(\"{\"{toplam([1])}\"}\")", Header + "\nG100(1)", "//! tr kodlu 9", "`{eğer}` // yazdır"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tr string) {
		if b, err := Split(tr); len(tr) > 16<<10 || err != nil || b.Coded {
			return
		}
		kodlu, err := Encode(tr)
		var sh *SyntaxError
		var bh *HeaderError
		if errors.As(err, &sh) || errors.As(err, &bh) || errors.Is(err, ErrUTF8) {
			return
		}
		if err != nil {
			t.Fatalf("kodlanamadı: %v\nkaynak: %q", err, tr)
		}
		geri, err := Decode(kodlu, codes.Active())
		if err != nil {
			t.Fatalf("kodlu biçim çözülemedi: %v\nkaynak: %q\nkodlu: %q", err, tr, kodlu)
		}
		if strings.Contains(tr, "$") {
			if yine, err := Encode(geri); err != nil || yine != kodlu {
				t.Fatalf("kod → TR → kod aynı değil (%v)\nkodlu: %q\nyine: %q", err, kodlu, yine)
			}
			return
		}
		if geri != tr {
			t.Fatalf("TR → kod → TR aynı değil\nkaynak: %q\nkodlu: %q\ngeri: %q", tr, kodlu, geri)
		}
	})
}

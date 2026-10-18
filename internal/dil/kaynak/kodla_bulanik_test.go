package kaynak

import (
	"errors"
	"os"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/kod"
)

// FuzzKodlaCoz: rastgele Türkçe kaynakta kodlu biçim kayıpsızdır (17.4). Ayrıştırılabilen her
// kaynak kodlanır (çevirici kendi sonucunu denetler; ErrKayip bir çevirici hatasıdır) ve
// kodlu biçim çözülünce özgün kaynak bayt bayt geri gelir. Gereksiz '$' kaçışı çözümde
// düşebileceği için '$' içeren kaynakta kod → TR → kod kimliği sınanır. Derin arama:
//
//	go test ./internal/dil/kaynak -run '^$' -fuzz FuzzKodlaCoz -fuzztime 5m
func FuzzKodlaCoz(f *testing.F) {
	for _, yol := range []string{"ornekler/02_dil_turu.tr", "ornekler/03_testler_test.tr", "testler/yapilar_test.tr", "testler/sozcuk_yapisi_test.tr"} {
		if veri, err := os.ReadFile(depo.Yol(yol)); err == nil {
			f.Add(string(veri))
		}
	}
	for _, s := range []string{"\xef\xbb\xbfyazdır(1)\r\n", "değişken $eğer = 1", "yazdır(\"{\"{toplam([1])}\"}\")", Baslik + "\nG100(1)", "//! tr kodlu 9", "`{eğer}` // yazdır"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tr string) {
		if b, err := Ayir(tr); len(tr) > 16<<10 || err != nil || b.Kodlu {
			return
		}
		kodlu, err := Kodla(tr)
		var sh *SozdizimiHatasi
		var bh *BaslikHatasi
		if errors.As(err, &sh) || errors.As(err, &bh) || errors.Is(err, ErrUTF8) {
			return
		}
		if err != nil {
			t.Fatalf("kodlanamadı: %v\nkaynak: %q", err, tr)
		}
		geri, err := Coz(kodlu, kod.Etkin())
		if err != nil {
			t.Fatalf("kodlu biçim çözülemedi: %v\nkaynak: %q\nkodlu: %q", err, tr, kodlu)
		}
		if strings.Contains(tr, "$") {
			if yine, err := Kodla(geri); err != nil || yine != kodlu {
				t.Fatalf("kod → TR → kod aynı değil (%v)\nkodlu: %q\nyine: %q", err, kodlu, yine)
			}
			return
		}
		if geri != tr {
			t.Fatalf("TR → kod → TR aynı değil\nkaynak: %q\nkodlu: %q\ngeri: %q", tr, kodlu, geri)
		}
	})
}

package veribicim

import (
	"testing"

	"tr_dili/internal/dil/yurut"
)

// FuzzCSVCoz: csv_oku'ya verilen içerik güvenilmeyen girdidir (indirilen veri kümesi).
// Çözücü hiçbir girdide çökmemeli; her hücre ya sayı ya metin olmalı ve metin hücreler
// girdideki baytları değiştirmeden taşımalıdır (hiçbir hücre sessizce bozulmaz). Derin arama:
//
//	go test ./internal/motor/veribicim -run '^$' -fuzz FuzzCSVCoz -fuzztime 5m
func FuzzCSVCoz(f *testing.F) {
	for _, s := range []string{"ad,yaş\nAli,30\nAyşe,2.5e3", "\ufeff\"a\"\"b\",007\r\n1,-0", "a,\"b\nc\",d", "\"", "1e400,NaN,+5,.5,0x10", "x;y\n\n,,,"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, girdi string) {
		_ = yolGibi(girdi)
		deger, err := csvCoz(girdi)
		if err != nil {
			return
		}
		satirlar, ok := deger.(*yurut.ListVal)
		if !ok {
			t.Fatalf("liste bekleniyordu: %T", deger)
		}
		for _, s := range satirlar.Elements {
			hucreler, ok := s.(*yurut.ListVal)
			if !ok {
				t.Fatalf("satır liste değil: %T", s)
			}
			for _, h := range hucreler.Elements {
				switch v := h.(type) {
				case *yurut.IntVal, *yurut.FloatVal:
				case *yurut.StringVal:
					if d, ok := hucreDegeri(v.Val).(*yurut.StringVal); !ok || d.Val != v.Val {
						t.Fatalf("metin hücre %q yeniden çözümde değişti", v.Val)
					}
				default:
					t.Fatalf("beklenmeyen hücre türü %T", h)
				}
			}
		}
	})
}

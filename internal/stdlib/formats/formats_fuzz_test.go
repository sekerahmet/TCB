package formats

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzCSVDecode: csv_çöz'e ve csv_oku'ya verilen içerik güvenilmeyen girdidir (indirilen veri kümesi).
// Çözücü hiçbir girdide çökmemeli; her hücre kırpılmış metin olmalıdır (13.8): hiçbir hücre
// sayıya dönüşüp sessizce bozulmaz. Derin arama:
//
//	go test ./internal/stdlib/formats -run '^$' -fuzz FuzzCSVDecode -fuzztime 5m
func FuzzCSVDecode(f *testing.F) {
	for _, s := range []string{"ad,yaş\nAli,30\nAyşe,2.5e3", "\ufeff\"a\"\"b\",007\r\n1,-0", "a,\"b\nc\",d", "\"", "1e400,NaN,+5,.5,0x10", "x;y\n\n,,,"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		value, err := decodeCSV(text)
		if err != nil {
			return
		}
		lines, ok := value.(*interp.ListVal)
		if !ok {
			t.Fatalf("liste bekleniyordu: %T", value)
		}
		for _, s := range lines.Elements {
			cells, ok := s.(*interp.ListVal)
			if !ok {
				t.Fatalf("satır liste değil: %T", s)
			}
			for _, h := range cells.Elements {
				v, ok := h.(*interp.StringVal)
				if !ok {
					t.Fatalf("hücre metin olmalı: %T", h)
				}
				if v.Val != strings.TrimSpace(v.Val) {
					t.Fatalf("hücre %q kırpılmamış", v.Val)
				}
			}
		}
	})
}

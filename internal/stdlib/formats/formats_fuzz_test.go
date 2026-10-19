package formats

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzCSVDecode: csv_oku'ya verilen içerik güvenilmeyen girdidir (indirilen veri kümesi).
// Çözücü hiçbir girdide çökmemeli; her hücre ya sayı ya metin olmalı ve metin hücreler
// girdideki baytları değiştirmeden taşımalıdır (hiçbir hücre sessizce bozulmaz). Derin arama:
//
//	go test ./internal/stdlib/formats -run '^$' -fuzz FuzzCSVDecode -fuzztime 5m
func FuzzCSVDecode(f *testing.F) {
	for _, s := range []string{"ad,yaş\nAli,30\nAyşe,2.5e3", "\ufeff\"a\"\"b\",007\r\n1,-0", "a,\"b\nc\",d", "\"", "1e400,NaN,+5,.5,0x10", "x;y\n\n,,,"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		_ = looksLikePath(text)
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
				switch v := h.(type) {
				case *interp.IntVal, *interp.FloatVal:
				case *interp.StringVal:
					if d, ok := cellValue(v.Val).(*interp.StringVal); !ok || d.Val != v.Val {
						t.Fatalf("metin hücre %q yeniden çözümde değişti", v.Val)
					}
				default:
					t.Fatalf("beklenmeyen hücre türü %T", h)
				}
			}
		}
	})
}

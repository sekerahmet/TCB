package jsonwrite

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzJSONRoundTrip: json_çöz'e verilen metin güvenilmeyen girdidir (indirilen veri,
// yapılandırma). Çözücü (interp.JSONToTR) hiçbir girdide çökmemeli; çözdüğü değer
// yazılabiliyorsa yazılan metin geçerli JSON olmalı ve yeniden çözülüp yazıldığında
// aynı kalmalıdır. Derin arama:
//
//	go test ./internal/stdlib/common/jsonwrite -run '^$' -fuzz FuzzJSONRoundTrip -fuzztime 5m
func FuzzJSONRoundTrip(f *testing.F) {
	for _, s := range []string{`{"b": 1, "a": [1, 2.5, -3e-7, "ç", null, true]}`, `[]`, `{}`, `"😀"`,
		`1e400`, `-0`, `12345678901234567890`, `{"a":{"a":{"a":[[[[]]]]}}}`, `{"x":1,"x":2}`,
		strings.Repeat("[", 1001) + strings.Repeat("]", 1001)} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		value, err := interp.JSONToTR(data)
		if err != nil {
			return
		}
		written, err := Write(value)
		if err != nil {
			return // ör. sonsuz sayı: JSON'a yazılamaz, açık hata verir
		}
		if !json.Valid(written) {
			t.Fatalf("geçersiz JSON yazıldı: %q", written)
		}
		second, err := interp.JSONToTR(written)
		if err != nil {
			t.Fatalf("yazılan JSON yeniden çözülemedi: %v (%q)", err, written)
		}
		again, err := Write(second)
		if err != nil || !bytes.Equal(again, written) {
			t.Fatalf("gidiş-dönüş değişti: %q -> %q (%v)", written, again, err)
		}
		if _, err := WriteIndent(value, "  "); err != nil {
			t.Fatalf("girintili yazım düştü: %v", err)
		}
	})
}

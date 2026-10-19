package jsonwrite

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzJSONGidisDonus: json_çöz'e verilen metin güvenilmeyen girdidir (indirilen veri,
// yapılandırma). Çözücü (yurut.JSONToTR) hiçbir girdide çökmemeli; çözdüğü değer
// yazılabiliyorsa yazılan metin geçerli JSON olmalı ve yeniden çözülüp yazıldığında
// aynı kalmalıdır. Derin arama:
//
//	go test ./internal/stdlib/common/jsonwrite -run '^$' -fuzz FuzzJSONGidisDonus -fuzztime 5m
func FuzzJSONGidisDonus(f *testing.F) {
	for _, s := range []string{`{"b": 1, "a": [1, 2.5, -3e-7, "ç", null, true]}`, `[]`, `{}`, `"😀"`,
		`1e400`, `-0`, `12345678901234567890`, `{"a":{"a":{"a":[[[[]]]]}}}`, `{"x":1,"x":2}`,
		strings.Repeat("[", 1001) + strings.Repeat("]", 1001)} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, veri []byte) {
		deger, err := interp.JSONToTR(veri)
		if err != nil {
			return
		}
		yazilan, err := Yaz(deger)
		if err != nil {
			return // ör. sonsuz sayı: JSON'a yazılamaz, açık hata verir
		}
		if !json.Valid(yazilan) {
			t.Fatalf("geçersiz JSON yazıldı: %q", yazilan)
		}
		ikinci, err := interp.JSONToTR(yazilan)
		if err != nil {
			t.Fatalf("yazılan JSON yeniden çözülemedi: %v (%q)", err, yazilan)
		}
		tekrar, err := Yaz(ikinci)
		if err != nil || !bytes.Equal(tekrar, yazilan) {
			t.Fatalf("gidiş-dönüş değişti: %q -> %q (%v)", yazilan, tekrar, err)
		}
		if _, err := YazGirintili(deger, "  "); err != nil {
			t.Fatalf("girintili yazım düştü: %v", err)
		}
	})
}

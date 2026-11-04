package jsonwrite

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzJSONRoundTrip: json_çöz'e verilen metin güvenilmeyen girdidir (indirilen veri,
// yapılandırma). Çözücü (interp.JSONToTR) hiçbir girdide çökmemeli; bir nesnesinde aynı anahtar
// iki kez yazılmış metni reddetmeli (3.7a); çözdüğü değer yazılabiliyorsa yazılan metin geçerli JSON
// olmalı ve yeniden çözülüp yazıldığında aynı kalmalıdır. Derin arama:
//
//	go test ./internal/stdlib/common/jsonwrite -run '^$' -fuzz FuzzJSONRoundTrip -fuzztime 5m
func FuzzJSONRoundTrip(f *testing.F) {
	for _, s := range []string{`{"b": 1, "a": [1, 2.5, -3e-7, "ç", null, true]}`, `[]`, `{}`, `"😀"`,
		`1e400`, `-0`, `12345678901234567890`, `{"a":{"a":{"a":[[[[]]]]}}}`, `{"x":1,"x":2}`, `[{"a":{"b":1,"b":[2]}}]`, `{"a":1,"b":{"a":2}}`,
		strings.Repeat("[", 1001) + strings.Repeat("]", 1001)} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		value, err := interp.JSONToTR(data)
		if err == nil && duplicateKey(data) {
			t.Fatalf("aynı anahtar iki kez yazılmış nesne kabul edildi: %q", data)
		}
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

// duplicateKey metnin bir nesnesinde aynı anahtarın iki kez yazılıp yazılmadığını encoding/json'un
// belirteçleriyle, çözücüden bağımsız olarak söyler (geçersiz metinde false).
func duplicateKey(data []byte) bool {
	type frame struct {
		keys    map[string]bool // nil: dizi
		wantKey bool
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var stack []*frame
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].keys != nil {
			stack[n-1].wantKey = true
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				stack = append(stack, &frame{keys: map[string]bool{}, wantKey: true})
			case '[':
				stack = append(stack, &frame{})
			default:
				stack = stack[:len(stack)-1]
				valueDone()
			}
		case string:
			if n := len(stack); n > 0 && stack[n-1].keys != nil && stack[n-1].wantKey {
				if stack[n-1].keys[t] {
					return true
				}
				stack[n-1].keys[t], stack[n-1].wantKey = true, false
				continue
			}
			valueDone()
		default:
			valueDone()
		}
	}
}

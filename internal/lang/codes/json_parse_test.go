package codes

import (
	"encoding/json"
	"strings"
	"testing"
)

// Başlangıç için yazılan çözümleyici, tr.json'u encoding/json ile aynı okumalı ve bozuk
// girdiyi reddetmeli.
func TestCozumleyiciJSONIleAyni(t *testing.T) {
	ham, err := dosyaCoz(trJSON)
	if err != nil {
		t.Fatal(err)
	}
	type girdi struct {
		Ad      string   `json:"ad"`
		Eski    []string `json:"eski"`
		Yabanci []string `json:"yabancı"`
		Aile    string   `json:"aile"`
	}
	var std map[string]girdi
	if err := json.Unmarshal(trJSON, &std); err != nil {
		t.Fatal(err)
	}
	if len(ham) != len(std) {
		t.Fatalf("%d girdi, encoding/json %d", len(ham), len(std))
	}
	for _, g := range ham {
		s := std[g.anahtar]
		if g.ad != s.Ad || g.aile != s.Aile ||
			strings.Join(g.eski, "|") != strings.Join(s.Eski, "|") || strings.Join(g.yabanci, "|") != strings.Join(s.Yabanci, "|") {
			t.Errorf("%s farklı okundu: %+v / %+v", g.anahtar, g, s)
		}
	}
	g, err := dosyaCoz([]byte("\xef\xbb\xbf{ \"G100\" : {\"ad\": \"a\\u00e7\\\"\", \"eski\": [], \"yabancı\": [\"print\"]} }"))
	if err != nil || len(g) != 1 || g[0].ad != "aç\"" || len(g[0].yabanci) != 1 || g[0].yabanci[0] != "print" {
		t.Errorf("kaçışlı metin ya da BOM okunamadı: %+v %v", g, err)
	}
	for _, bozuk := range []string{``, `[]`, `{"G100": {"ad": "a"}`, `{"G100": {"ad": "a"},}`, `{"G100": {"ad": 1}}`,
		`{"G100": {"ad": "a", "ad": "b"}}`, `{"G100": {"ad": "a"}} x`, `{"G100": {"ascii": false}}`, `{"G100": {"yabancı": "a"}}`, `{"G100": {"ad": "a", "es": ["b"]}}`,
		"{\"G100\": {\"ad\": \"a\nb\"}}", `{"G100": {"ad": "a"`} {
		if _, err := dosyaCoz([]byte(bozuk)); err == nil {
			t.Errorf("%q reddedilmeliydi", bozuk)
		}
	}
}

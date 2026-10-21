package codes

import (
	"encoding/json"
	"strings"
	"testing"
)

// Başlangıç için yazılan çözümleyici, tr.json'u encoding/json ile aynı okumalı ve bozuk
// girdiyi reddetmeli.
func TestParserMatchesJSON(t *testing.T) {
	raw, err := parseFile(trJSON)
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		Ad      string   `json:"ad"`
		Yabanci []string `json:"yabancı"`
		Aile    string   `json:"aile"`
	}
	var std map[string]entry
	if err := json.Unmarshal(trJSON, &std); err != nil {
		t.Fatal(err)
	}
	if len(raw) != len(std) {
		t.Fatalf("%d girdi, encoding/json %d", len(raw), len(std))
	}
	for _, g := range raw {
		s := std[g.key]
		if g.name != s.Ad || g.family != s.Aile ||
			strings.Join(g.foreign, "|") != strings.Join(s.Yabanci, "|") {
			t.Errorf("%s farklı okundu: %+v / %+v", g.key, g, s)
		}
	}
	g, err := parseFile([]byte("\xef\xbb\xbf{ \"G100\" : {\"ad\": \"a\\u00e7\\\"\", \"yabancı\": [\"print\"]} }"))
	if err != nil || len(g) != 1 || g[0].name != "aç\"" || len(g[0].foreign) != 1 || g[0].foreign[0] != "print" {
		t.Errorf("kaçışlı metin ya da BOM okunamadı: %+v %v", g, err)
	}
	for _, broken := range []string{``, `[]`, `{"G100": {"ad": "a"}`, `{"G100": {"ad": "a"},}`, `{"G100": {"ad": 1}}`,
		`{"G100": {"ad": "a", "ad": "b"}}`, `{"G100": {"ad": "a"}} x`, `{"G100": {"ascii": false}}`, `{"G100": {"yabancı": "a"}}`, `{"G100": {"ad": "a", "es": ["b"]}}`, `{"G100": {"ad": "a", "eski": ["b"]}}`,
		"{\"G100\": {\"ad\": \"a\nb\"}}", `{"G100": {"ad": "a"`} {
		if _, err := parseFile([]byte(broken)); err == nil {
			t.Errorf("%q reddedilmeliydi", broken)
		}
	}
}

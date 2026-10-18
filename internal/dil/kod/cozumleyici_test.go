package kod

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
		Ad    string   `json:"ad"`
		Eski  []string `json:"eski"`
		Aile  string   `json:"aile"`
		ASCII *bool    `json:"ascii"`
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
			strings.Join(g.eski, "|") != strings.Join(s.Eski, "|") || (g.ascii == nil) != (s.ASCII == nil) ||
			(g.ascii != nil && *g.ascii != *s.ASCII) {
			t.Errorf("%s farklı okundu: %+v / %+v", g.anahtar, g, s)
		}
	}
	g, err := dosyaCoz([]byte("\xef\xbb\xbf{ \"G100\" : {\"ad\": \"a\\u00e7\\\"\", \"eski\": [], \"ascii\": true} }"))
	if err != nil || len(g) != 1 || g[0].ad != "aç\"" || g[0].ascii == nil || !*g[0].ascii {
		t.Errorf("kaçışlı metin ya da BOM okunamadı: %+v %v", g, err)
	}
	for _, bozuk := range []string{``, `[]`, `{"G100": {"ad": "a"}`, `{"G100": {"ad": "a"},}`, `{"G100": {"ad": 1}}`,
		`{"G100": {"ad": "a", "ad": "b"}}`, `{"G100": {"ad": "a"}} x`, `{"G100": {"ascii": evet}}`, `{"G100": {"ad": "a", "es": ["b"]}}`,
		"{\"G100\": {\"ad\": \"a\nb\"}}", `{"G100": {"ad": "a"`} {
		if _, err := dosyaCoz([]byte(bozuk)); err == nil {
			t.Errorf("%q reddedilmeliydi", bozuk)
		}
	}
}

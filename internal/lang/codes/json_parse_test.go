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
		Name    string   `json:"name"`
		Foreign []string `json:"foreign"`
		Family  string   `json:"family"`
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
		if g.name != s.Name || g.family != s.Family ||
			strings.Join(g.foreign, "|") != strings.Join(s.Foreign, "|") {
			t.Errorf("%s farklı okundu: %+v / %+v", g.key, g, s)
		}
	}
	g, err := parseFile([]byte("\xef\xbb\xbf{ \"TR_fn_print\" : {\"name\": \"a\\u00e7\\\"\", \"foreign\": [\"print\"]} }"))
	if err != nil || len(g) != 1 || g[0].name != "aç\"" || len(g[0].foreign) != 1 || g[0].foreign[0] != "print" {
		t.Errorf("kaçışlı metin ya da BOM okunamadı: %+v %v", g, err)
	}
	for _, broken := range []string{``, `[]`, `{"TR_fn_print": {"name": "a"}`, `{"TR_fn_print": {"name": "a"},}`, `{"TR_fn_print": {"name": 1}}`,
		`{"TR_fn_print": {"name": "a", "name": "b"}}`, `{"TR_fn_print": {"name": "a"}} x`, `{"TR_fn_print": {"ascii": false}}`, `{"TR_fn_print": {"foreign": "a"}}`, `{"TR_fn_print": {"name": "a", "es": ["b"]}}`, `{"TR_fn_print": {"name": "a", "eski": ["b"]}}`,
		"{\"TR_fn_print\": {\"name\": \"a\nb\"}}", `{"TR_fn_print": {"name": "a"`} {
		if _, err := parseFile([]byte(broken)); err == nil {
			t.Errorf("%q reddedilmeliydi", broken)
		}
	}
}

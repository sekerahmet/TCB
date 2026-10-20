package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var codeKeyLine = regexp.MustCompile(`^\s*"([A-Z]\d{3,4})"\s*:`)

// jsonKeyLines bir JSON dosyasında her kodun anahtarının satır numarasını verir.
func jsonKeyLines(file string) map[string]int {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	lines := map[string]int{}
	for i, l := range strings.Split(string(data), "\n") {
		if m := codeKeyLine.FindStringSubmatch(l); m != nil {
			lines[m[1]] = i + 1
		}
	}
	return lines
}

func readJSON(file string, v any) bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

// extractCodes ileti kodlarını (messages/tr.json + definitions.json) ve ad kodlarını
// (codes/tr.json) düğüm yapar. Kullanım kenarlarını Go çıkarıcısı kurar (uses_message, uses_name).
func extractCodes(root string, b *builder) {
	texts := map[string]string{}
	defs := map[string]struct {
		Param []string `json:"param"`
	}{}
	dir := filepath.Join(root, filepath.FromSlash(messagesDir))
	if readJSON(filepath.Join(dir, "tr.json"), &texts) {
		readJSON(filepath.Join(dir, "definitions.json"), &defs)
		rel := messagesDir + "/tr.json"
		lines := jsonKeyLines(filepath.Join(dir, "tr.json"))
		for code, text := range texts {
			b.addNode(Node{ID: "msg:" + code, Kind: kindMessage, Name: code, File: rel,
				Line: lines[code], Detail: text, Params: defs[code].Param})
		}
	}
	names := map[string]struct {
		Ad   string `json:"ad"`
		Aile string `json:"aile"`
	}{}
	cdir := filepath.Join(root, filepath.FromSlash(codesDir))
	if readJSON(filepath.Join(cdir, "tr.json"), &names) {
		rel := codesDir + "/tr.json"
		lines := jsonKeyLines(filepath.Join(cdir, "tr.json"))
		for code, n := range names {
			b.addNode(Node{ID: "name:" + code, Kind: kindName, Name: code, File: rel,
				Line: lines[code], Detail: n.Ad + " (" + n.Aile + ")"})
		}
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// codeKeyLine bir JSON satırındaki kod anahtarıdır: ileti kodu (H2410) ya da ad kimliği (TR_fn_sum).
var codeKeyLine = regexp.MustCompile(`^\s*"(H\d{4}|TR_[A-Za-z0-9_]+)"\s*:`)

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
				Line: lines[code], End: lines[code], Detail: text, Params: defs[code].Param})
		}
	}
	type nameEntry struct {
		Ad   string `json:"ad"`
		Aile string `json:"aile"`
	}
	names := map[string]nameEntry{}
	english := map[string]nameEntry{}
	cdir := filepath.Join(root, filepath.FromSlash(codesDir))
	readJSON(filepath.Join(cdir, "en.json"), &english)
	if readJSON(filepath.Join(cdir, "tr.json"), &names) {
		rel := codesDir + "/tr.json"
		lines := jsonKeyLines(filepath.Join(cdir, "tr.json"))
		for code, n := range names {
			b.addNode(Node{ID: "name:" + code, Kind: kindName, Name: code, File: rel,
				Line: lines[code], End: lines[code], Detail: n.Ad + " (" + n.Aile + ")",
				Aliases: nameAliases(n.Ad, english[code].Ad)})
		}
	}
	relatedNames(cdir, b)
	envMessages(root, b)
}

// relatedNames dil tanımının "ilişkili" bağını (definitions.json: yazdır ↔ girdi, birleştir ↔ böl) ad kodları arasında
// `related` kenarı yapar: yeni bir ad tasarlanırken akraba adlar ilk bakılacak yerdir (deneme 8 S2).
func relatedNames(cdir string, b *builder) {
	defs := map[string]struct {
		Related []string `json:"ilişkili"`
	}{}
	path := filepath.Join(cdir, "definitions.json")
	if !readJSON(path, &defs) {
		return
	}
	rel := codesDir + "/definitions.json"
	lines := jsonKeyLines(path)
	for code, d := range defs {
		for _, other := range d.Related {
			b.addEdge("name:"+code, "name:"+other, edgeRelated, extracted, rel, lines[code])
		}
	}
}

// nameAliases bir ad kodunun dildeki adlarıdır (Türkçe, İngilizce), boşlar ve yinelenenler atılır.
func nameAliases(tr, en string) []string {
	var out []string
	for _, a := range []string{tr, en} {
		if a != "" && (len(out) == 0 || out[0] != a) {
			out = append(out, a)
		}
	}
	return out
}

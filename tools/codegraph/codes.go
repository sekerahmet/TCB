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

// commonNames definitions.json'daki ortak adlardır (kod → ad): her yazım dilinde aynı yazılan, adı hiçbir dil dosyasında
// yazılmayan kodlar (madde 15 adım 2, "common_name"; codes.Load da öyle okur). Dil dosyasında adı olmayan kodun adı budur:
// tek kaynak. Okuyucular (ad kodu düğümleri, ortam değişkeni adları) adı buradan tamamlar.
func commonNames(root string) map[string]string {
	defs := map[string]struct {
		Common string `json:"common_name"`
	}{}
	readJSON(filepath.Join(root, filepath.FromSlash(codesDir), "definitions.json"), &defs)
	out := map[string]string{}
	for code, d := range defs {
		if d.Common != "" {
			out[code] = d.Common
		}
	}
	return out
}

// orCommon dil dosyasındaki ad, yoksa kodun ortak adıdır.
func orCommon(name, code string, common map[string]string) string {
	if name != "" {
		return name
	}
	return common[code]
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
		Name   string `json:"name"`
		Family string `json:"family"`
	}
	names := map[string]nameEntry{}
	english := map[string]nameEntry{}
	cdir := filepath.Join(root, filepath.FromSlash(codesDir))
	readJSON(filepath.Join(cdir, "en.json"), &english)
	if readJSON(filepath.Join(cdir, "tr.json"), &names) {
		rel := codesDir + "/tr.json"
		lines := jsonKeyLines(filepath.Join(cdir, "tr.json"))
		common := commonNames(root)
		for code, n := range names {
			tr := orCommon(n.Name, code, common)
			b.addNode(Node{ID: "name:" + code, Kind: kindName, Name: code, File: rel,
				Line: lines[code], End: lines[code], Detail: tr + " (" + n.Family + ")",
				Aliases: nameAliases(tr, orCommon(english[code].Name, code, common))})
		}
	}
	relatedNames(cdir, b)
	envMessages(root, b)
}

// relatedNames dil tanımının "related" bağını (definitions.json: yazdır ↔ girdi, birleştir ↔ böl) ad kodları arasında
// `related` kenarı yapar: yeni bir ad tasarlanırken akraba adlar ilk bakılacak yerdir (deneme 8 S2).
func relatedNames(cdir string, b *builder) {
	defs := map[string]struct {
		Related []string `json:"related"`
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

// snippets, VS Code eklentisinin kod şablonlarını (editors/vscode/snippets/snippets.json) her yazım
// dili için üretir. Şablonun gövdesi bir kez, Türkçe geçerli TR kodu olarak yazılır (templates.go);
// diğer dillerdeki gövde o kodun source.Show ile o dilin yazımında gösterimidir: anahtar kelime,
// yerleşik, tip ve üye adları dilin kod listesinden gelir, elle çevrilmez. Kullanıcının dolduracağı
// yerler (ad, koşul, sayı) kodda "ph_" ile başlayan adlarla yazılır; üretici onları dilin sözcükleriyle
// VS Code şablon durağına (${1:koşul}) çevirir. Eklenti, kişinin yazım diline (tr.writingLanguage) göre
// ilgili dilin şablonlarını sunar.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/gen/snippets           // dosyayı yeniden yazar
//	go run ./tools/gen/snippets -check    // güncel değilse 1 koduyla çıkar
//
// Aynı denetim `go test ./tools/gen/...` ile de yapılır.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
)

// snippetsPath depo köküne göre üretilen dosyadır.
var snippetsPath = filepath.Join("editors", "vscode", "snippets", "snippets.json")

func main() {
	genmain.RunMain("snippets", snippetsPath, func(string) ([]byte, error) { return Generate() })
}

// fallbackLanguage, sözcük, ad ve açıklama tablosunda satırı olmayan dillerin kullandığı dildir.
const fallbackLanguage = "en"

// holeRe şablon kodundaki yer tutucu adıdır (ph_ad).
var holeRe = regexp.MustCompile(`ph_[A-Za-z0-9_]+`)

// wordRe bir deliğin metnindeki dil sözcüğüdür (%ad%).
var wordRe = regexp.MustCompile(`%([A-Za-z_0-9]+)%`)

// entry bir şablonun bir dildeki hâlidir (snippets.json).
type entry struct {
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Prefix      []string `json:"prefix"`
	Body        []string `json:"body"`
}

// text tablosundan dil için değeri verir; dilin satırı yoksa yedek dilinkini kullanır.
func text(table map[string]string, lang string) string {
	if v, ok := table[lang]; ok {
		return v
	}
	return table[fallbackLanguage]
}

// words dilin sözcük tablosudur; satırı olmayan dil yedek dilinkini kullanır.
func words(lang string) map[string]string {
	if w, ok := wordTable[lang]; ok {
		return w
	}
	return wordTable[fallbackLanguage]
}

// fill bir deliğin metnindeki %sözcük% yazımlarını dilin sözcükleriyle doldurur.
func fill(hole, lang string) (string, error) {
	var missing string
	out := wordRe.ReplaceAllStringFunc(hole, func(m string) string {
		w, ok := words(lang)[strings.Trim(m, "%")]
		if !ok {
			missing = m
		}
		return w
	})
	if missing != "" {
		return "", fmt.Errorf("%q sözcüğü %s için tanımlı değil", missing, lang)
	}
	return out, nil
}

// build şablonun lang dilindeki hâlidir.
func build(t template, language *codes.Language) (entry, error) {
	lang := language.Code
	shown, err := source.Show(t.Source, language)
	if err != nil {
		return entry{}, fmt.Errorf("%s şablonu %s için gösterilemedi: %w", t.ID, lang, err)
	}
	used := map[string]bool{}
	var failure error
	shown = holeRe.ReplaceAllStringFunc(shown, func(name string) string {
		hole, ok := t.Holes[name]
		if !ok {
			failure = fmt.Errorf("%s şablonunda %s için delik tanımı yok", t.ID, name)
			return name
		}
		used[name] = true
		filled, err := fill(hole, lang)
		if err != nil {
			failure = fmt.Errorf("%s şablonu: %w", t.ID, err)
		}
		return filled
	})
	if failure != nil {
		return entry{}, failure
	}
	for name := range t.Holes {
		if !used[name] {
			return entry{}, fmt.Errorf("%s şablonunda %s deliği kullanılmıyor", t.ID, name)
		}
	}
	body := strings.Split(strings.TrimRight(shown, "\n"), "\n")
	if t.End {
		body[len(body)-1] += "$0"
	}
	var prefix []string
	for _, p := range t.Prefix {
		name, err := prefixName(p, language)
		if err != nil {
			return entry{}, fmt.Errorf("%s şablonu: %w", t.ID, err)
		}
		prefix = append(prefix, name)
		if t.Member {
			prefix = append(prefix, "."+name)
		}
	}
	return entry{Label: text(t.Label, lang), Description: text(t.Description, lang), Prefix: prefix, Body: body}, nil
}

// prefixName kısayolun dildeki yazımıdır: "code:KwIf" gibi bir kodun adı ya da "word:ad" sözcüğü.
func prefixName(p prefix, language *codes.Language) (string, error) {
	if p.Word != "" {
		return fill("%"+p.Word+"%", language.Code)
	}
	g, ok := language.Entry(p.Code)
	if !ok {
		return "", fmt.Errorf("%s kodunun %s dilinde adı yok", p.Code, language.Code)
	}
	return g.Name, nil
}

// Generate bütün yazım dillerinin şablon dosyasını üretir.
func Generate() ([]byte, error) {
	result := map[string][]entry{}
	for _, lang := range codes.Languages() {
		language, err := codes.LoadLanguage(lang)
		if err != nil {
			return nil, err
		}
		for _, t := range templates {
			e, err := build(t, language)
			if err != nil {
				return nil, err
			}
			result[lang] = append(result[lang], e)
		}
	}
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

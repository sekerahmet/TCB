package main

import (
	"embed"
	"encoding/json"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Şablonların verisi dilden bağımsızdır (data.json): gövde (source) kodlu biçimde durur (TR_func ...) ve her
// dilde kimlik yazımıyla kalır; Türkçe sözcük yalnız dil metinlerindedir (text/<dil>.json: şablon adı,
// açıklama ve kullanıcıya önerilen sözcükler). Kullanıcının dolduracağı yerler
// "ph_" adlarıdır ve holes'ta VS Code şablon durağına (${1:varsayılan}) çevrilir; bir deliğin metnindeki
// %sözcük% dilin sözcüğüyle gelir. Durak varsayılanında dil öğesi (anahtar kelime, yerleşik adı) bulunmaz.

//go:embed data.json
var dataJSON []byte

//go:embed text/*.json
var textFiles embed.FS

// prefix kısayolun kaynağıdır: bir kodun dildeki adı (Code, kimlik: TR_kw_func) ya da words
// tablosundaki bir sözcük (Word).
type prefix struct {
	Code string `json:"code"`
	Word string `json:"word"`
}

// template bir kod şablonudur.
type template struct {
	ID     string            `json:"id"`
	Prefix []prefix          `json:"prefix"`
	Member bool              `json:"member"` // kısayol ayrıca ".ad" olarak da sunulur (metot şablonları)
	End    bool              `json:"end"`    // son satırın sonuna $0 (imleç son durağı) eklenir
	Source string            `json:"source"`
	Holes  map[string]string `json:"holes"`
}

// languageText bir dilin metinleridir (text/<dil>.json).
type languageText struct {
	Words     map[string]string `json:"words"`
	Templates map[string]struct {
		Label       string `json:"label"`
		Description string `json:"description"`
	} `json:"templates"`
}

// texts dil koduna göre metinlerdir; templates data.json'daki şablonlardır.
var (
	texts     = map[string]languageText{}
	templates []template
)

func init() {
	var data struct {
		Templates []template `json:"templates"`
	}
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		panic("snippets: data.json: " + err.Error())
	}
	templates = data.Templates
	files, err := textFiles.ReadDir("text")
	if err != nil {
		panic("snippets: text: " + err.Error())
	}
	for _, f := range files {
		raw, err := textFiles.ReadFile("text/" + f.Name())
		var text languageText
		if err == nil {
			err = json.Unmarshal(raw, &text)
		}
		if err != nil {
			panic("snippets: " + f.Name() + ": " + err.Error())
		}
		texts[strings.TrimSuffix(f.Name(), ".json")] = text
	}
}

// code kısayolun kodudur.
func (p prefix) code() (codes.Code, error) { return codes.Parse(p.Code) }
